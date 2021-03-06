Non blocking hash table for Go, designed to have read performance
close to that of go's native map, but without requiring a RWMutex.
This should perform better if you have a lot of cores.  Copied from
http://github.com/zond/gotomic, with a few changes:

* Remove interfaces; strong typing for keys and hash table entries means fewer memory allocations
* Thread-local datastructures that are reused instead of allocating temporary datastructures per request
* Tests are in flux and probably broken

## Results
Note: garbage collection is turned off.  16 byte keys and interface values (integers). 2^20 keys.

Look here for the benchmarking code:  http://github.com/narula/gomap-bench

```
BenchmarkGoMapReadConcurrentNoLock-80   300000000                5.07 ns/op
BenchmarkGoMapReadConcurrentLocked-80   10000000               573 ns/op
BenchmarkZgotomicReadConcurrent-80      20000000               119 ns/op
BenchmarkGotomicReadConcurrent-80       100000000               23.8 ns/op
```