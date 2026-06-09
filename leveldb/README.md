# Hemi goleveldb

[![MIT licensed](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

LevelDB key/value database in Go, forked from
[syndtr/goleveldb](https://github.com/syndtr/goleveldb).

## Attribution

This library is built on the work of the
[syndtr/goleveldb](https://github.com/syndtr/goleveldb) project, a pure-Go
implementation of LevelDB. The original journal, memdb, table, compaction,
and transaction implementations form the foundation of this library. We are
grateful for that contribution to the open-source Go ecosystem.

## Why this fork

Upstream goleveldb has been unmaintained since 2022. This fork exists so
that projects depending on it (initially Hemi's `tbcd`) have a maintained
base for correctness fixes, particularly around the `OpenTransaction` /
`Commit` path under concurrent access.

The import point is upstream commit `126854a` (the final upstream master
commit, "leveldb: fix table file leaks when manifest is rotated").

## Module path

```
github.com/hemilabs/x/goleveldb
```

Consume via a `replace` directive while iterating locally:

```
replace github.com/syndtr/goleveldb => github.com/hemilabs/x/goleveldb v0.0.0-...
```

## License

MIT, same as upstream. See [LICENSE](LICENSE).
