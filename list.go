package gotomic

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

var deletedElement = "deleted"

type ListIterator func(e *entry) bool

type element struct {
	// The next element in the list. If this pointer has the deleted
	// flag set it means THIS element, not the next one, is deleted.
	unsafe.Pointer
	value Thing
	entry *entry
}

type hit struct {
	left    *element
	element *element
	right   *element
}

func (self *hit) String() string {
	return fmt.Sprintf("&hit{%v,%v,%v}", self.left.val(), self.element.val(), self.right.val())
}

type Thing interface{}

var list_head = "LIST_HEAD"

func (self *element) next() *element {
	next := atomic.LoadPointer(&self.Pointer)
	for next != nil {
		nextElement := (*element)(next)
		/*
		 If our next element contains &deletedElement that means WE are deleted, and
		 we can just return the next-next element. It will make it impossible to add
		 stuff to us, since we will always lie about our next(), but then again, deleted
		 elements shouldn't get new children anyway.
		*/
		// if sp, ok := nextElement.value.(*string); ok && sp == &deletedElement {
		// 	return nextElement.next()
		// }
		/*
		 If our next element is itself deleted (by the same criteria) then we will just replace
		 it with its next() (which should be the first thing behind it that isn't itself deleted
		 (the power of recursion compels you) and then check again.
		*/
		if nextElement.isDeleted() {
			atomic.CompareAndSwapPointer(&self.Pointer, next, unsafe.Pointer(nextElement.next()))
			next = atomic.LoadPointer(&self.Pointer)
		} else {
			/*
			 If it isn't deleted then we just return it.
			*/
			return nextElement
		}
	}
	/*
	 And if our next is nil, then we are at the end of the list and can just return nil for next()
	*/
	return nil
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

func (self *element) val() Thing {
	if self == nil {
		return nil
	}
	return self.value
}
func (self *element) String() string {
	return fmt.Sprint(self.ToSlice())
}
func (self *element) Describe() string {
	if self == nil {
		return fmt.Sprint(nil)
	}
	deleted := ""
	if sp, ok := self.value.(*string); ok && sp == &deletedElement {
		deleted = " (x)"
	}
	return fmt.Sprintf("%#v%v -> %v", self, deleted, self.next().Describe())
}
func (self *element) isDeleted() bool {
	// next := atomic.LoadPointer(&self.Pointer)
	// if next == nil {
	// 	return false
	// }
	// if sp, ok := (*element)(next).value.(*string); ok && sp == &deletedElement {
	// 	return true
	// }
	return false
}
func (self *element) add(e *entry) (rval bool) {
	alloc := &element{}
	for {
		/*
		 If we are deleted then we do not allow adding new children.
		*/
		if self.isDeleted() {
			break
		}
		/*
		 If we succeed in adding before our perceived next, just return true.
		*/
		if self.addBefore(e, alloc, self.next()) {
			rval = true
			break
		}
	}
	return
}
func (self *element) addBefore(e *entry, allocatedElement, before *element) bool {
	if self.next() != before {
		return false
	}
	allocatedElement.entry = e
	allocatedElement.Pointer = unsafe.Pointer(before)
	return atomic.CompareAndSwapPointer(&self.Pointer, unsafe.Pointer(before), unsafe.Pointer(allocatedElement))
}

/*
 inject c into self either before the first matching value (c.Compare(value) == 0), before the first value
 it should be before (c.Compare(value) < 0) or after the first value it should be after (c.Compare(value) > 0).
*/
func (self *element) inject(e *entry) {
	alloc := &element{}
	for {
		hit := self.search(e)
		if hit.left != nil {
			if hit.element != nil {
				if hit.left.addBefore(e, alloc, hit.element) {
					break
				}
			} else {
				if hit.left.addBefore(e, alloc, hit.right) {
					break
				}
			}
		} else if hit.element != nil {
			if hit.element.addBefore(e, alloc, hit.right) {
				break
			}
		} else {
			panic(fmt.Errorf("Unable to inject %v properly into %v, it ought to be first but was injected into the first element of the list!", e, self))
		}
	}
}
func (self *element) ToSlice() []Thing {
	rval := make([]Thing, 0)
	current := self
	for current != nil {
		rval = append(rval, current.value)
		current = current.next()
	}
	return rval
}

/*
 search for c in self.

 Will stop searching when finding nil or an element that should be after c (c.Compare(element) < 0).

 Will return a hit containing the last elementRef and element before a match (if no match, the last elementRef and element before
 it stops searching), the elementRef and element for the match (if a match) and the last elementRef and element after the match
 (if no match, the first elementRef and element, or nil/nil if at the end of the list).
*/
func (self *element) search(e *entry) (rval *hit) {
	rval = &hit{nil, self, nil}
	for {
		if rval.element == nil {
			return
		}
		rval.right = rval.element.next()
		switch cmp := e.Compare(rval.element.entry); {
		case cmp < 0:
			rval.right = rval.element
			rval.element = nil
			return
		case cmp == 0:
			return
		}
		rval.left = rval.element
		rval.element = rval.left.next()
		rval.right = nil
	}
	panic(fmt.Sprint("Unable to search for ", e, " in ", self))
}

func (self *element) search2(e *entry, hh *hit) (rval *hit) {
	//rval = &hit{nil, self, nil}
	rval = hh
	for {
		if rval.element == nil {
			return
		}
		rval.right = rval.element.next()
		var cmp int = e.Compare(rval.element.entry)
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

func (h *hit) Set(e *element) {
	h.element = e
}

func (self *element) doRemove() bool {
	return false
}

// Just a shorthand to hide the inner workings of our removal mechanism.
// func (self *element) doRemove() bool {
// 	return self.add(&deletedElement)
// }
// func (self *element) remove() (rval Thing, ok bool) {
// 	n := self.next()
// 	for {
// 		// No children to remove.
// 		if n == nil {
// 			break
// 		}
// 		// We managed to remove next!
// 		if n.doRemove() {
// 			self.next()
// 			rval = n.value
// 			ok = true
// 			break
// 		}
// 		n = self.next()
// 	}
// 	return
// }
