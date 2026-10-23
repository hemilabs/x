# github.com/hemilabs/x/leveldb 

[![BSD-2-Clause licensed](https://img.shields.io/badge/license-BSD--2--Clause-blue.svg)](LICENSE)

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

Fixes in this fork on the transaction path:

- `Commit` releases the compaction commit lock when the MANIFEST commit
  fails, so a retry and later compactions do not block.
- `OpenTransaction` releases the write lock on its error returns, and it
  does not open a transaction on a database that is closing.
- A large `DB.Write` that goes through a transaction discards the
  transaction when its commit fails.
- After a failed MANIFEST record write, the next commit writes a fresh
  MANIFEST.  `Discard` after a failed commit forces this rewrite before
  it removes the tables.  If the rewrite fails too, it keeps the tables
  until the next `Open` removes them, which costs disk space until then.

## Module path

```
github.com/hemilabs/x/goleveldb
```

Consume via a `replace` directive while iterating locally:

```
replace github.com/syndtr/goleveldb => github.com/hemilabs/x/goleveldb ...
```

## License

The contents of this directory are licensed under the 2-Clause BSD License. See [LICENSE](LICENSE).
