Non blocking hash table for Go, designed to have read performance
close to that of go's native map, but without requiring a RWMutex.
This should perform better if you have a lot of cores.  Copied from
github.com/zond/gotomic, with a few changes:

* Remove interfaces; strong typing for keys and hash table entires means less memory allocation
* Thread-local datastructures that are reused instead of allocating temporary datastructures per request

## Results

Note: garbage collection is turned off.  16 byte keys and interface values (integers). 2^20 keys.
```
BenchmarkGoMapReadConcurrentNoLock-80   1000000000             4.61 ns/op
BenchmarkGoMapReadConcurrentLocked-80   20000000               166 ns/op
BenchmarkGotomicReadConcurrent-80       30000000               226 ns/op
BenchmarkMyGotomicReadConcurrent-80     100000000              30.2 ns/op
```