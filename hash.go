package gotomic

import (
	"bytes"
	"fmt"
	"hash/crc32"
	"sync/atomic"
	"unsafe"
)

const max_exponent = 32
const default_load_factor = 0.5

type HashIterator func(k Key, v unsafe.Pointer) bool

type Equalable interface {
	Equals(Thing) bool
}

// Convenience type for generic byte keys
type Key [16]byte

func (k Key) HashCode() uint32 {
	return crc32.ChecksumIEEE(k[:])
}

func (k Key) Equals(sk Key) bool {
	return sk == k
}

func MakeKey(x uint64) Key {
	var i uint64
	var b [16]byte
	for i = 0; i < 8; i++ {
		b[i] = byte((x >> (i * 8)))
	}
	return Key(b)
}

type entry struct {
	hashCode uint32
	hashKey  uint32
	key      Key
	value    unsafe.Pointer
}

type LocalData struct {
	te  *entry
	hh  *hashHit
	hit *hit
}

func InitLocalData() *LocalData {
	l := &LocalData{
		te:  &entry{},
		hh:  &hashHit{},
		hit: &hit{nil, nil, nil},
	}
	return l
}

func ReusableEntry() *entry {
	return &entry{}
}

func ReusableHashHit() *hashHit {
	return &hashHit{}
}

func (hh *hashHit) Set(hh2 *hashHit) {
	hh.left = hh2.left
	hh.element = hh2.element
	hh.right = hh2.right
}

func (te *entry) Set(hc uint32, k Key) {
	te.hashCode = hc
	te.hashKey = reverse(hc) | 1
	te.key = k
	te.value = nil
}

func newRealEntryWithHashCode(k Key, v unsafe.Pointer, hc uint32) *entry {
	return &entry{hashCode: hc, hashKey: reverse(hc) | 1, key: k, value: v}
}
func newRealEntry(k Key, v unsafe.Pointer) *entry {
	return newRealEntryWithHashCode(k, v, k.HashCode())
}
func newMockEntry(hashCode uint32) *entry {
	return &entry{hashCode: hashCode, hashKey: reverse(hashCode) &^ 1, value: nil}
}
func (self *entry) real() bool {
	return self.hashKey&1 == 1
}
func (self *entry) val() unsafe.Pointer {
	if self.value == nil {
		return nil
	}
	k := atomic.LoadPointer(&self.value)
	//return *(*Thing)(k)
	return k
	//	return *(*Thing)(atomic.LoadPointer(&self.value))
}
func (self *entry) String() string {
	return fmt.Sprintf("&entry{%0.32b/%0.32b, %v=>%v}", self.hashCode, self.hashKey, self.key, self.val())
}
func (self *entry) Compare(e *entry) int {
	if e == nil {
		return 1
	}
	if self.hashKey > e.hashKey {
		return 1
	} else if self.hashKey < e.hashKey {
		return -1
	} else {
		return 0
	}
}

/*
 Hash is a hash table based on "Split-Ordered Lists: Lock-Free
 Extensible Hash Tables" by Ori Shalev and Nir Shavit
 <http://www.cs.ucf.edu/~dcm/Teaching/COT4810-Spring2011/Literature/SplitOrderedLists.pdf>.

 TL;DR: It creates a linked list containing all hashed entries, and an
 extensible table of 'shortcuts' into said list. To enable future
 extensions to the shortcut table, the list is ordered in reversed bit
 order so that new table entries point into finer and finer sections
 of the potential address space.

 To enable growing the table a two dimensional slice of
 unsafe.Pointers is used, where each consecutive slice is twice the
 size of the one before.  This makes it simple to allocate
 exponentially more memory for the table with only a single extra
 indirection.
*/

type Bucket struct {
	padding  [128]byte
	b        []unsafe.Pointer
	padding1 [128]byte
}

type Hash struct {
	exponent   uint32
	buckets    []unsafe.Pointer
	size       int64
	loadFactor float64
}

func NewHash() *Hash {
	rval := &Hash{exponent: 0, buckets: make([]unsafe.Pointer, max_exponent), size: 0, loadFactor: default_load_factor}
	b := make([]unsafe.Pointer, 1)
	rval.buckets[0] = unsafe.Pointer(&b)
	return rval
}

func (self *Hash) Size() int {
	return int(atomic.LoadInt64(&self.size))
}

/*
 Each will run i on each key and value.

 It returns true if the iteration was interrupted.  This is the case
 when one of the HashIterator calls returned true, indicating the
 iteration should be stopped.
*/
func (self *Hash) Each(i HashIterator) bool {
	return self.getBucketByHashCode(0).each(func(e entry) bool {
		return e.real() && i(e.key, e.val())
	})
}

/*
 Verify the integrity of the Hash. Used mostly in my own tests but go
 ahead and call it if you fear corruption.
*/
// func (self *Hash) Verify() error {
// 	bucket := self.getBucketByHashCode(0)
// 	if e := bucket.verify(); e != nil {
// 		return e
// 	}
// 	for bucket != nil {
// 		e := bucket.value.(*entry)
// 		if e.real() {
// 			if ok, index, super, sub := self.isBucket(bucket); ok {
// 				return fmt.Errorf("%v has %v that should not be a bucket but is bucket %v (%v, %v)", self, e, index, super, sub)
// 			}
// 		} else {
// 			if ok, _, _, _ := self.isBucket(bucket); !ok {
// 				return fmt.Errorf("%v has %v that should be a bucket but isn't", self, e)
// 			}
// 		}
// 		bucket = bucket.next()
// 	}
// 	return nil
// }

/*
 ToMap returns a map[Hashable]Thing that is logically identical to the Hash.
*/
func (self *Hash) ToMap() map[Key]Thing {
	rval := make(map[Key]Thing)

	self.Each(func(k Key, v unsafe.Pointer) bool {
		rval[k] = v
		return false
	})

	return rval
}

func (self *Hash) isBucket(n *element) (isBucket bool, index, superIndex, subIndex uint32) {
	e := n.entry
	index = e.hashCode & ((1 << self.exponent) - 1)
	superIndex, subIndex = self.getBucketIndices(index)
	subBucket := *(*[]unsafe.Pointer)(self.buckets[superIndex])
	if subBucket[subIndex] == unsafe.Pointer(n) {
		isBucket = true
	}
	return
}

/*
 Describe returns a multi line description of the contents of the map for
 those of you interested in debugging it or seeing an example of how split-ordered lists work.
*/
func (self *Hash) Describe() string {
	buffer := bytes.NewBufferString(fmt.Sprintf("&Hash{%p size:%v exp:%v maxload:%v}\n", self, self.size, self.exponent, self.loadFactor))
	element := self.getBucketByIndex(0)
	for element != nil {
		e := element.entry
		if ok, index, super, sub := self.isBucket(element); ok {
			fmt.Fprintf(buffer, "%3v:%3v,%3v: %v *\n", index, super, sub, e)
		} else {
			fmt.Fprintf(buffer, "             %v\n", e)
		}
		element = element.next()
	}
	return string(buffer.Bytes())
}
func (self *Hash) String() string {
	return fmt.Sprint(self.ToMap())
}

type hashHit hit

func (self *hashHit) search(cmp *entry, tmpval *hashHit) (rval *hashHit) {
	rval = tmpval
	//	rval = &hashHit{self.left, self.element, self.right}
	for {
		if rval.element == nil {
			break
		}
		rval.right = rval.element.next()
		e := rval.element.entry
		if e.hashKey != cmp.hashKey {
			rval.right = rval.element
			rval.element = nil
			break
		}
		if cmp.key.Equals(e.key) {
			break
		}
		rval.left = rval.element
		rval.element = rval.left.next()
		rval.right = nil
	}
	return
}

// GetHC returns the key with hashCode that equals k.  Use this when
// you already have the hash code and don't want to force gotomic to
// calculate it again.
func (self *Hash) GetHC(hashCode uint32, k Key, ld *LocalData) (rval unsafe.Pointer, ok bool) {
	//	fmt.Printf("gotomic: hashcode: %v for key %v ", hashCode, k)
	//  testEntry := newRealEntryWithHashCode(k, nil, hashCode)
	ld.te.Set(hashCode, k)
	bucket := self.getBucketByIndexWrapper(ld.te.hashCode, ld.hit)
	ld.hit.element = bucket
	hit := (*hashHit)(bucket.search_local(*ld.te, ld.hit))
	ld.hh.Set(hit)
	if hit2 := hit.search(ld.te, ld.hh); hit2.element != nil {
		rval = hit2.element.entry.val()
		ok = true
	}
	return
}

// Get returns the value at k and whether it was present in the Hash.
func (self *Hash) Get(k Key) (unsafe.Pointer, bool) {
	ld := InitLocalData()
	return self.GetHC(k.HashCode(), k, ld)
}

// PutIfMissing will insert v under k if k contains expected in the Hash, and return whether it inserted anything.
func (self *Hash) PutIfPresent(k Key, v unsafe.Pointer, expected Equalable) (rval bool) {
	newEntry := newRealEntry(k, v)
	for {
		bucket := self.getBucketByHashCode(newEntry.hashCode)
		hit := (*hashHit)(bucket.search(*newEntry))
		tmp := &hashHit{hit.left, hit.element, hit.right}
		if hit2 := hit.search(newEntry, tmp); hit2.element == nil {
			break
		} else {
			oldEntry := hit2.element.entry
			oldValuePtr := atomic.LoadPointer(&oldEntry.value)
			if expected.Equals(*(*Thing)(oldValuePtr)) {
				if atomic.CompareAndSwapPointer(&oldEntry.value, oldValuePtr, unsafe.Pointer(newEntry.value)) {
					rval = true
					break
				}
			} else {
				break
			}
		}
	}
	return
}

// PutIfMissing will insert v under k if k was missing from the Hash, and return whether it inserted anything.
func (self *Hash) PutIfMissing(k Key, v unsafe.Pointer) (rval bool) {
	newEntry := newRealEntry(k, v)
	alloc := &element{}
	for {
		bucket := self.getBucketByHashCode(newEntry.hashCode)
		hit := (*hashHit)(bucket.search(*newEntry))
		tmp := &hashHit{hit.left, hit.element, hit.right}
		if hit2 := hit.search(newEntry, tmp); hit2.element == nil {
			if hit2.left.addBefore(*newEntry, alloc, hit2.right) {
				self.addSize(1)
				return true
			}
		} else {
			break
		}
	}
	return
}

// PutHC will put k and v in the Hash using hashCode and return the overwritten value and whether any value was overwritten.
// Use this when you already have the hash code and don't want to force gotomic to calculate it again.
func (self *Hash) PutHC(hashCode uint32, k Key, v unsafe.Pointer) (rval unsafe.Pointer, ok bool) {
	newEntry := newRealEntryWithHashCode(k, v, hashCode)
	alloc := &element{}
	for {
		bucket := self.getBucketByHashCode(newEntry.hashCode)
		hit := (*hashHit)(bucket.search(*newEntry))
		tmp := &hashHit{hit.left, hit.element, hit.right}
		if hit2 := hit.search(newEntry, tmp); hit2.element == nil {
			if hit2.left.addBefore(*newEntry, alloc, hit2.right) {
				self.addSize(1)
				break
			}
		} else {
			oldEntry := hit2.element.entry
			rval = oldEntry.val()
			ok = true
			atomic.StorePointer(&oldEntry.value, newEntry.value)
			break
		}
	}
	return
}

// Put k and v in the Hash and return the overwritten value and whether any value was overwritten.
func (self *Hash) Put(k Key, v unsafe.Pointer) (rval unsafe.Pointer, ok bool) {
	return self.PutHC(k.HashCode(), k, v)
}

func (self *Hash) addSize(i int) {
	atomic.AddInt64(&self.size, int64(i))
	if atomic.LoadInt64(&self.size) > int64(self.loadFactor*float64(uint32(1)<<self.exponent)) {
		self.grow()
	}
}
func (self *Hash) grow() {
	oldExponent := atomic.LoadUint32(&self.exponent)
	newExponent := oldExponent + 1
	newBuckets := make([]unsafe.Pointer, 1<<oldExponent)
	if atomic.CompareAndSwapPointer(&self.buckets[newExponent], nil, unsafe.Pointer(&newBuckets)) {
		atomic.CompareAndSwapUint32(&self.exponent, oldExponent, newExponent)
	}
}

func (self *Hash) getPreviousBucketIndex(bucketKey uint32) uint32 {
	exp := atomic.LoadUint32(&self.exponent)
	return reverse(((bucketKey >> (max_exponent - exp)) - 1) << (max_exponent - exp))
}

func (self *Hash) getBucketByHashCode(hashCode uint32) *element {
	x := hashCode & ((1 << self.exponent) - 1)
	return self.getBucketByIndex(x)
}

func (self *Hash) getBucketIndices(index uint32) (superIndex, subIndex uint32) {
	if index > 0 {
		superIndex = log2(index)
		subIndex = index - (1 << superIndex)
		superIndex++
	}
	return
}

func (self *Hash) getBucketByIndex(index uint32) (bucket *element) {
	superIndex, subIndex := self.getBucketIndices(index)
	subBuckets := *(*[]unsafe.Pointer)(self.buckets[superIndex])
	for {
		bucket = (*element)(subBuckets[subIndex])
		if bucket != nil {
			break
		}
		mockEntry := newMockEntry(index)
		if index == 0 {
			bucket := &element{Pointer: nil, entry: *mockEntry}
			atomic.CompareAndSwapPointer(&subBuckets[subIndex], nil, unsafe.Pointer(bucket))
		} else {
			prev := self.getPreviousBucketIndex(mockEntry.hashKey)
			previousBucket := self.getBucketByIndex(prev)
			if hit := previousBucket.search(*mockEntry); hit.element == nil {
				hit.left.addBefore(*mockEntry, &element{}, hit.right)
			} else {
				atomic.CompareAndSwapPointer(&subBuckets[subIndex], nil, unsafe.Pointer(hit.element))
			}
		}
	}
	return bucket
}

func (self *Hash) getBucketByIndexWrapper(hashCode uint32, hh *hit) (bucket *element) {
	index := hashCode & ((1 << self.exponent) - 1)
	superIndex, subIndex := self.getBucketIndices(index)
	subBuckets := *(*[]unsafe.Pointer)(self.buckets[superIndex])
	for {
		bucket = (*element)(subBuckets[subIndex])
		if bucket != nil {
			break
		}
		mockEntry := newMockEntry(index)
		if index == 0 {
			bucket := &element{Pointer: nil, entry: *mockEntry}
			atomic.CompareAndSwapPointer(&subBuckets[subIndex], nil, unsafe.Pointer(bucket))
		} else {
			prev := self.getPreviousBucketIndex(mockEntry.hashKey)
			previousBucket := self.getBucketByIndex(prev)
			hh.element = previousBucket
			if hit := previousBucket.search_local(*mockEntry, hh); hit.element == nil {
				hit.left.addBefore(*mockEntry, &element{}, hit.right)
			} else {
				atomic.CompareAndSwapPointer(&subBuckets[subIndex], nil, unsafe.Pointer(hit.element))
			}
		}
	}
	return bucket
}
