# hemilabs/x

This repository contains experimental and vendored packages with no promises of backwards compatibility.

<!--
Currently, the following packages are available:

- [`name`](name): Description - [Docs](https://pkg.go.dev/github.com/hemilabs/x/name)
-->

## Contributing

We welcome contributions of all kinds!

- Found a bug or have an idea for a new feature? Open an [issue](https://github.com/hemilabs/x/issues).
- Want to build something new? We recommend starting with
  a [proposal issue](https://github.com/hemilabs/x/issues).
- Have improvements for our code or docs? Pull requests are very welcome!

Join the discussion in our [Discord server](https://discord.gg/hemixyz) or keep an eye
on [issues](https://github.com/hemilabs/x/issues)
and [pull requests](https://github.com/hemilabs/x/pulls).

## License

This repository contains code under **multiple licenses**:

| Module          | License                                  | Origin                         |
|-----------------|------------------------------------------|--------------------------------|
| `tss`           | MIT, with the Binance notice             | bnb-chain tss-lib              |
| `leveldb`       | BSD 2-Clause                             | syndtr/goleveldb               |
| `merkle`        | ISC                                      | decred dcrtime                 |
| `eth-trie`      | LGPL-3.0 (which includes the GPL-3.0)    | go-ethereum `trie`, `triedb`   |
| `tools/slacker` | MIT (the root [LICENSE](LICENSE))        | Hemi Labs                      |

The default license for new code is the [MIT License](LICENSE).

A binary that links `eth-trie` must obey the LGPL-3.0 conditions. See
[eth-trie/README.md](eth-trie/README.md).

For more information, see the `LICENSE` and `COPYING` files of each module.
