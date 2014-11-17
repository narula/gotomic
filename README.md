# gotomic

Non blocking hash table for Go.

## Algorithms

The `List` type is implemented using [A Pragmatic Implementation of Non-Blocking Linked-Lists by Timothy L. Harris](http://www.timharris.co.uk/papers/2001-disc.pdf).

The `Hash` type is implemented using [Split-Ordered Lists: Lock-Free Extensible Hash Tables by Ori Shalev and Nir Shavit](http://www.cs.ucf.edu/~dcm/Teaching/COT4810-Spring2011/Literature/SplitOrderedLists.pdf) with the List type used as backend.