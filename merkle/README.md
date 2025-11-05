# merkle

Package merkle implements a disk storable merkle tree and partial path
verfication.

The tree is stored in a linear array, and returns a slice of the backing array.
A linear array was chosen as opposed to an actual tree structure since it uses
about half as much memory.  The following describes a merkle tree and how it is
stored in a linear array.

A merkle tree is a tree in which every non-leaf node is the hash of its
children nodes.  A diagram depicting how this works for decred transactions
where h(x) is a shas256 hash follows:
              
                        root = h1234 = h(h12 + h34)
                       /                           \
                 h12 = h(h1 + h2)            h34 = h(h3 + h4)
                  /            \              /            \
               h1 = h(tx1)  h2 = h(tx2)    h3 = h(tx3)  h4 = h(tx4)
              
 The above stored as a linear array is as follows:
              
              [h1 h2 h3 h4 h12 h34 root]

As the above shows, the merkle root is always the last element in the array.

The number of inputs is not always a power of two which results in a balanced
tree structure as above.  In that case, parent nodes with no children are also
zero and parent nodes with only a single left node are calculated by
concatenating the left node with itself before hashing.  Since this function
uses nodes that are pointers to the hashes, empty nodes will be nil.

If the sorted flag is true we sort the incoming hashes array in order to always
generate the same merkle tree regardless of input order.


# Origin

This package is a derivative of work originally available at
https://github.com/decred/dcrtime/tree/master/merkle, copied at commit
`e35b14e324c3b30f96f53b05383d5cd1ed62abb6`.

The contents of this package are distributed under the [ISC LICENSE](LICENSE)
file.
