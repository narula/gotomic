package gotomic

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

type ListIterator func(e entry) bool

type element struct {
	// The next element in the list.
	unsafe.Pointer
	entry entry
}

type hit struct {
	left    *element
	element *element
	right   *element
}

type Thing interface{}

var list_head = "LIST_HEAD"

func (self *element) next() *element {
	next := atomic.LoadPointer(&self.Pointer)
	if next == nil {
		return nil
	}
	nextElement := (*element)(next)
	return nextElement
}

func (self *element) each(i ListIterator) bool {
	n := self

	for n != nil {
		if i(n.entry) {
			return true
		}
		n = n.next()
	}

	return false
}

func (self *element) String() string {
	return fmt.Sprint(self.ToSlice())
}
func (self *element) Describe() string {
	if self == nil {
		return fmt.Sprint(nil)
	}
	return fmt.Sprintf("%#v -> %v", self, self.next().Describe())
}

func (self *element) add(e entry) (rval bool) {
	alloc := &element{}
	for {
		// If we succeed in adding before our perceived next, just return true.
		if self.addBefore(e, alloc, self.next()) {
			rval = true
			break
		}
	}
	return
}

func (self *element) addBefore(e entry, allocatedElement, before *element) bool {
	if self.next() != before {
		return false
	}
	allocatedElement.entry = e
	allocatedElement.Pointer = unsafe.Pointer(before)
	return atomic.CompareAndSwapPointer(&self.Pointer, unsafe.Pointer(before), unsafe.Pointer(allocatedElement))
}

// /*
//  inject c into self either before the first matching value (c.Compare(value) == 0), before the first value
//  it should be before (c.Compare(value) < 0) or after the first value it should be after (c.Compare(value) > 0).
// */
// func (self *element) inject(e entry) {
// 	alloc := &element{}
// 	for {
// 		hit := self.search(e)
// 		if hit.left != nil {
// 			if hit.element != nil {
// 				if hit.left.addBefore(e, alloc, hit.element) {
// 					break
// 				}
// 			} else {
// 				if hit.left.addBefore(e, alloc, hit.right) {
// 					break
// 				}
// 			}
// 		} else if hit.element != nil {
// 			if hit.element.addBefore(e, alloc, hit.right) {
// 				break
// 			}
// 		} else {
// 			panic(fmt.Errorf("Unable to inject %v properly into %v, it ought to be first but was injected into the first element of the list!", e, self))
// 		}
// 	}
// }

func (self *element) ToSlice() []Thing {
	rval := make([]Thing, 0)
	current := self
	for current != nil {
		rval = append(rval, current.entry)
		current = current.next()
	}
	return rval
}

// search without thread-local *hit
func (self *element) search(e entry) *hit {
	tmp := &hit{nil, self, nil}
	return self.search_local(e, tmp)
}

// search for e in self.  Will stop searching when finding nil or an
// element that should be after e (e.Compare(element) < 0).  Will
// return a hit containing the last elementRef and element before a
// match (if no match, the last elementRef and element before it stops
// searching), the elementRef and element for the match (if a match)
// and the last elementRef and element after the match (if no match,
// the first elementRef and element, or nil/nil if at the end of the
// list).
func (self *element) search_local(e entry, hh *hit) (rval *hit) {
	rval = hh
	for {
		if rval.element == nil {
			return
		}
		rval.right = rval.element.next()
		var cmp int = e.Compare(&(rval.element.entry))
		if cmp < 0 {
			rval.right = rval.element
			rval.element = nil
			return
		} else if cmp == 0 {
			return
		}
		rval.left = rval.element
		rval.element = rval.left.next()
		rval.right = nil
	}
	panic(fmt.Sprint("Unable to search for ", e, " in ", self))
}

func ReusableHit() *hit {
	return &hit{nil, nil, nil}
}

func (self *element) doRemove() bool {
	return false
}
