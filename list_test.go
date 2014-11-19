package gotomic

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"runtime"
	"testing"
	"time"
)

type c int

func (self c) Compare(t Thing) int {
	if s, ok := t.(c); ok {
		if self > s {
			return 1
		} else if self < s {
			return -1
		} else {
			return 0
		}
	}
	panic(fmt.Errorf("%#v can only compare to other c's, not %#v of type %T", self, t, t))
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func fiddle(t *testing.T, nr *element, do, done chan bool) {
	<-do
	num := 10000
	for i := 0; i < num; i++ {
		x := rand.Int()
		nr.add(x)
	}
	done <- true
}

func fiddleAndAssertSort(t *testing.T, nr *element, do chan bool, ichan chan []c) {
	<-do
	num := 1000
	var injected []c
	for i := 0; i < num; i++ {
		v := c(-int(math.Abs(float64(rand.Int()))))
		nr.inject(v)
		injected = append(injected, v)
		if err := nr.verify(); err != nil {
			t.Error(nr, "should be correct, but got", err)
		}
	}
	ichan <- injected
}

func assertSlicey(t *testing.T, nr *element, cmp []Thing) {
	if sl := nr.ToSlice(); !reflect.DeepEqual(sl, cmp) {
		t.Errorf("%v should be %#v but is %#v", nr.Describe(), cmp, sl)
	}
}

func TestPushPop(t *testing.T) {
	nr := new(element)
	assertSlicey(t, nr, []Thing{nil})
	nr.add("hej")
	assertSlicey(t, nr, []Thing{nil, "hej"})
	nr.add("haj")
	assertSlicey(t, nr, []Thing{nil, "haj", "hej"})
	nr.add("hoj")
	assertSlicey(t, nr, []Thing{nil, "hoj", "haj", "hej"})
}

func TestConcPushPop(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	nr := new(element)
	assertSlicey(t, nr, []Thing{nil})
	nr.add("1")
	nr.add("2")
	nr.add("3")
	nr.add("4")
	do := make(chan bool)
	done := make(chan bool)
	go fiddle(t, nr, do, done)
	go fiddle(t, nr, do, done)
	go fiddle(t, nr, do, done)
	go fiddle(t, nr, do, done)
	close(do)
	<-done
	<-done
	<-done
	<-done
	assertSlicey(t, nr, []Thing{nil, "4", "3", "2", "1"})
}

const ANY = "ANY VALUE"

func searchTest(t *testing.T, nr *element, s c, l, n, r Thing) {
	h := nr.search(s)
	if (l != ANY && !reflect.DeepEqual(h.left.val(), l)) ||
		(n != ANY && !reflect.DeepEqual(h.element.val(), n)) ||
		(r != ANY && !reflect.DeepEqual(h.right.val(), r)) {
		t.Error(nr, ".search(", s, ") should produce ", l, n, r, " but produced ", h.left.val(), h.element.val(), h.right.val())
	}
}

func makeStringEntry(ch string) entry {
	k := Key([]byte(ch))
	return *(newRealEntry(k, unsafe.Pointer(&ch)))
}

func TestListEach(t *testing.T) {
	nr := new(element)
	nr.add(makeStringEntry("h"))
	nr.add(makeStringEntry("g"))
	nr.add(makeStringEntry("f"))
	nr.add(makeStringEntry("d"))
	nr.add(makeStringEntry("c"))
	nr.add(makeStringEntry("b"))

	var a []*entry

	nr.each(func(e entry) bool {
		a = append(a, e)
		return false
	})

	exp := []entry{makeStringEntry("b"), makeStringEntry("c"), makeStringEntry("d"), makeStringEntry("f"), makeStringEntry("g"), makeStringEntry("h")}
	for i, _ := range exp {
		ch1 := *(*string)(exp[i].value)
		ch2 := *(*string)(nr[i+1].value)
		if ch1 != ch2 {
			t.Error(ch1, "should be", ch2)
		}
	}
}

func TestListEachInterrupt(t *testing.T) {
	nr := new(element)
	nr.add("h")
	nr.add("g")
	nr.add("f")
	nr.add("d")
	nr.add("c")
	nr.add("b")

	var a []entry

	interrupted := nr.each(func(e entry) bool {
		a = append(a, e)
		return len(a) == 2
	})

	if !interrupted {
		t.Error("Iteration should have been interrupted.")
	}

	if len(a) != 2 {
		t.Error("List should have 2 elements. Have", len(a))
	}
}

func TestPushBefore(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	nr := new(element)
	nr.add("h")
	nr.add("g")
	nr.add("f")
	nr.add("d")
	nr.add("c")
	nr.add("b")
	element := &element{}
	if nr.addBefore("a", element, nr) {
		t.Error("should not be possible")
	}
	if !nr.addBefore("a", element, nr.next()) {
		t.Error("should be possible")
	}
}

func TestSearch(t *testing.T) {
	nr := &element{nil, &list_head}
	nr.add(c(9))
	nr.add(c(8))
	nr.add(c(7))
	nr.add(c(5))
	nr.add(c(4))
	nr.add(c(3))
	assertSlicey(t, nr, []Thing{&list_head, c(3), c(4), c(5), c(7), c(8), c(9)})
	searchTest(t, nr, c(1), &list_head, nil, c(3))
	searchTest(t, nr, c(2), &list_head, nil, c(3))
	searchTest(t, nr, c(3), &list_head, c(3), c(4))
	searchTest(t, nr, c(4), c(3), c(4), c(5))
	searchTest(t, nr, c(5), c(4), c(5), c(7))
	searchTest(t, nr, c(6), c(5), nil, c(7))
	searchTest(t, nr, c(7), c(5), c(7), c(8))
	searchTest(t, nr, c(8), c(7), c(8), c(9))
	searchTest(t, nr, c(9), c(8), c(9), nil)
	searchTest(t, nr, c(10), c(9), nil, nil)
	searchTest(t, nr, c(11), c(9), nil, nil)
}

func TestVerify(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	nr := &element{nil, &list_head}
	nr.inject(c(3))
	nr.inject(c(5))
	nr.inject(c(9))
	nr.inject(c(7))
	nr.inject(c(4))
	nr.inject(c(8))
	assertSlicey(t, nr, []Thing{&list_head, c(3), c(4), c(5), c(7), c(8), c(9)})
	if err := nr.verify(); err != nil {
		t.Error(nr, "should verify as ok, got", err)
	}
	nr = &element{nil, &list_head}
	nr.add(c(3))
	nr.add(c(5))
	nr.add(c(9))
	nr.add(c(7))
	nr.add(c(4))
	nr.add(c(8))
	assertSlicey(t, nr, []Thing{&list_head, c(8), c(4), c(7), c(9), c(5), c(3)})
	s := fmt.Sprintf("[%v 8 4 7 9 5 3] is badly ordered. The following elements are in the wrong order: 8,4; 9,5; 5,3", &list_head)
	if err := nr.verify(); err.Error() != s {
		t.Error(nr, "should have errors", s, "but had", err)
	}
}

func TestInjectAndSearch(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	nr := &element{nil, &list_head}
	nr.inject(c(3))
	nr.inject(c(5))
	nr.inject(c(9))
	nr.inject(c(7))
	nr.inject(c(4))
	nr.inject(c(8))
	assertSlicey(t, nr, []Thing{&list_head, c(3), c(4), c(5), c(7), c(8), c(9)})
	searchTest(t, nr, c(1), &list_head, nil, c(3))
	searchTest(t, nr, c(2), &list_head, nil, c(3))
	searchTest(t, nr, c(3), &list_head, c(3), c(4))
	searchTest(t, nr, c(4), c(3), c(4), c(5))
	searchTest(t, nr, c(5), c(4), c(5), c(7))
	searchTest(t, nr, c(6), c(5), nil, c(7))
	searchTest(t, nr, c(7), c(5), c(7), c(8))
	searchTest(t, nr, c(8), c(7), c(8), c(9))
	searchTest(t, nr, c(9), c(8), c(9), nil)
	searchTest(t, nr, c(10), c(9), nil, nil)
	searchTest(t, nr, c(11), c(9), nil, nil)
}

func TestConcInjectAndSearch(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())
	nr := &element{nil, &list_head}
	nr.inject(c(3))
	nr.inject(c(5))
	nr.inject(c(9))
	nr.inject(c(7))
	nr.inject(c(4))
	nr.inject(c(8))
	assertSlicey(t, nr, []Thing{&list_head, c(3), c(4), c(5), c(7), c(8), c(9)})
	do := make(chan bool)
	ichan := make(chan []c)
	var injected [][]c
	for i := 0; i < runtime.NumCPU(); i++ {
		go fiddleAndAssertSort(t, nr, do, ichan)
	}
	close(do)
	for i := 0; i < runtime.NumCPU(); i++ {
		searchTest(t, nr, c(1), ANY, ANY, c(3))
		searchTest(t, nr, c(2), ANY, ANY, c(3))
		searchTest(t, nr, c(3), ANY, c(3), c(4))
		searchTest(t, nr, c(4), c(3), c(4), c(5))
		searchTest(t, nr, c(5), c(4), c(5), c(7))
		searchTest(t, nr, c(6), c(5), nil, c(7))
		searchTest(t, nr, c(7), c(5), c(7), c(8))
		searchTest(t, nr, c(8), c(7), c(8), c(9))
		searchTest(t, nr, c(9), c(8), c(9), nil)
		searchTest(t, nr, c(10), c(9), nil, nil)
		searchTest(t, nr, c(11), c(9), nil, nil)
		injected = append(injected, <-ichan)
	}
	assertSlicey(t, nr, []Thing{&list_head, c(3), c(4), c(5), c(7), c(8), c(9)})
	imap := make(map[c]int)
	for _, vals := range injected {
		for _, val := range vals {
			imap[val] = imap[val] + 1
		}
	}
	for val, num := range imap {
		if num2, ok := rmap[val]; ok {
			if num2 != num {
				t.Errorf("fiddlers injected %v of %v but removed %v", num, val, num2)
			}
		} else {
			t.Errorf("fiddlers injected %v of %v but removed none", num, val)
		}
	}
	for val, num := range rmap {
		if num2, ok := imap[val]; ok {
			if num2 != num {
				t.Errorf("fiddlers removed %v of %v but injected %v", num, val, num2)
			}
		} else {
			t.Errorf("fiddlers removed %v of %v but injected none", num, val)
		}
	}
}
