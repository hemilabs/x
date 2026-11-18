# eth-trie

Module eth-trie holds the Merkle Patricia trie and the trie database of
go-ethereum.

## Origin

This module is derived from the `trie` and `triedb` packages of
[go-ethereum](https://github.com/ethereum/go-ethereum) v1.16.6. Hemi Labs
modified it on 2025-11-05. The changes are:

- The import paths now point to `github.com/hemilabs/x/eth-trie`.
- The verkle and binary tries (`trie/verkle.go`, `trie/verkle_test.go`,
  `trie/bintrie`, `trie/utils`) are removed.
- `trie/transition.go` is disabled: all of the file, except the package
  clause, is commented out.
- The tests use `internal/testutil`, from Hemi Labs, in place of the
  `internal/testrand` package of go-ethereum.

The module still depends on the go-ethereum module v1.16.6 for other packages.

## License

This module is licensed under the GNU Lesser General Public License v3.0.
See the [COPYING.LESSER](COPYING.LESSER) file for full terms.
