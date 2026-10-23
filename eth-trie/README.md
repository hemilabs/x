# eth-trie

Module eth-trie holds the Merkle Patricia trie and the trie database of
go-ethereum.

## Origin

This module is derived from the `trie` and `triedb` packages of
[go-ethereum](https://github.com/ethereum/go-ethereum) v1.16.6. Hemi Labs
modified it on 2025-11-05.  The changes are:

- The import paths now point to `github.com/hemilabs/x/eth-trie`.
- The verkle and binary tries (`trie/verkle.go`, `trie/verkle_test.go`,
  `trie/bintrie`, `trie/utils`) are removed.
- `trie/transition.go` is disabled: all code in the file is commented out.
- The tests use `internal/testutil`, from Hemi Labs, in place of the
  `internal/testrand` package of go-ethereum.

The module still depends on the go-ethereum module v1.16.6 for other packages.

## License

This module is under the GNU Lesser General Public License v3.0
([LICENSE](LICENSE)). The LGPL-3.0 adds permissions to the GNU General
Public License v3.0 ([COPYING](COPYING)), so both texts apply.

If you distribute a binary that links this module, the LGPL-3.0, section 4,
applies to the binary.  In summary:

- Give a notice that the binary uses this library, and that the library is
  under the LGPL-3.0.
- Give a copy of the GPL-3.0 and the LGPL-3.0 texts.
- Let the user link the binary again with a changed version of this library.
  For example, give the source code of the application, or the object code
  that the user can link.

Read the license texts for the full terms. This summary is not legal
advice.
