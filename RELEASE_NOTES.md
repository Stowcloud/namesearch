# Release notes

## v0.1.0 release candidate

This release candidate prepares the standalone `github.com/stowcloud/namesearch`
module for its first tagged publication. It remains a release candidate until
the `v0.1.0` tag is created; no publication or push is performed by this
preparation change.

- Added repository support, security, maintainer, and contribution policies.
- Documented the SCNB v1 compatibility contract and disposable-index rollback
  procedure.
- Added the candidate iterator API for deterministic raw index candidates,
  opaque cursor resume, scan budgets, cancellation, snapshot invalidation,
  idempotent close, and explicit fallback plans for incomplete coverage or
  short queries.
- Defined corruption handling: an incomplete append-only tail is recoverable,
  while checksum, framing, and structural corruption disables the cache rather
  than changing search correctness.
- Added CI coverage for Linux amd64/arm64 tests, vet, and race checks plus
  non-Linux compile checks for darwin/amd64, darwin/arm64, windows/amd64, and
  freebsd/amd64.

### Compatibility

SCNB v1 bytes are unchanged. Existing SCNB v1 segments remain readable; any
future byte-format change requires an explicit format version and migration
plan. The source corpus remains authoritative. If an index cache is unusable,
disable it, rebuild into a fresh directory, verify a query, and replace the
old cache atomically.

The candidate iterator exposes raw indexed candidates only. It does not apply
product ACLs, filesystem stat results, or a product result limit. Callers must
apply those policies after candidate generation.

### Verification matrix

- Linux amd64: `go test ./...`, `go vet ./...`, and `go test -race ./...`.
- Linux arm64: `go test ./...` and `go vet ./...`.
- Compile-only: darwin/amd64, darwin/arm64, windows/amd64, and freebsd/amd64.

The release checklist records the commands and publication gate. This commit
does not create a tag, publish a module, or upload release artifacts.

