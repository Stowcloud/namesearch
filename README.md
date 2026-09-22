# namesearch

Standalone filename-search primitives and a persistent trigram index.

The root `namesearch` package provides folding, filtering, ranking, trigrams,
walking, and corpus estimation. The `index` subpackage provides the persistent
SCNB v1-compatible base, delta, and tombstone segments. The index is an
optional cache: callers can always fall back to a filesystem walk.

## Stability and release policy

This repository is preparing the v0.1.0 release candidate. Public APIs,
defaults, and on-disk behavior may change between pre-1.0 releases when the
change is documented and migration or rollback steps are provided. The module
is published only when the maintainer creates the `v0.1.0` tag; this release
preparation does not publish, push, or tag anything.

The SCNB v1 bytes are a separate compatibility contract. Existing SCNB v1
segments must remain readable, and changing their bytes requires an explicit
format version and a migration plan. The index directory is disposable cache
state: deleting it and rebuilding from the source corpus is the supported
rollback for an index-format or publication problem. Keep the source corpus
authoritative and retain a copy of the old index until a rebuilt index opens
and answers a verification query.

## Support and dependencies

The supported toolchain is Go 1.27.1, as declared by `go.mod`. CI tests Linux
amd64 and arm64, and compiles the non-Linux targets listed in the workflow.
Other platforms may work but are not release-tested. The module's direct
dependencies are `github.com/klauspost/compress`,
`github.com/stowcloud/durablefs`, and `golang.org/x/text`; the indirect
`golang.org/x/sys` dependency is managed by Go modules. Dependency updates
must preserve the SCNB byte contract and pass the full CI matrix.

The module intentionally contains no Stowcloud service, storage adapter, ACL,
transport, or application dependencies. Product callers must perform their own
ACL and stat checks; index candidates are not authorization decisions.

See [SUPPORT], [SECURITY], and [CONTRIBUTING] for project policies. The current
maintainer is listed in [MAINTAINERS].

## Format contract

`index/base.idx` is the immutable compressed SCNB v1 base segment (`SCNB`
magic, version 1). `delta.NNN.idx` files carry append-only updates and
`tomb.idx` carries deletions. Queries combine the base and deltas, then apply
tombstones. Delta and tombstone records use a length/checksum frame so a crash
at the tail can be recovered without silently accepting an incomplete record;
structural corruption is an error and disables the cache rather than changing
search correctness.

The candidate iterator exposes raw indexed candidates only. It does not apply
product ACLs, filesystem stat results, or a product result limit. Callers must
apply those policies after candidate generation.

## Development

Install Go 1.27.1, then run:

```sh
gofmt -w .
go test ./...
go vet ./...
go test -race ./...
```

The race run is useful for the concurrent in-memory/index paths. Do not commit
generated binaries or local index directories. Pull requests should explain
any compatibility impact and include rollback notes for format changes.

## License

namesearch is distributed under the Apache License 2.0; see [LICENSE].

[CONTRIBUTING]: CONTRIBUTING
[LICENSE]: LICENSE
[MAINTAINERS]: MAINTAINERS
[SECURITY]: SECURITY
[SUPPORT]: SUPPORT
