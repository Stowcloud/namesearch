# v0.1.0 release checklist

This checklist is for the v0.1.0 release candidate. Complete the publication
steps only after the maintainer approves the candidate; this preparation does
not tag, push, or publish anything.

## Documentation and compatibility

- [x] Release notes identify v0.1.0 as a release candidate.
- [x] README describes release status, support boundaries, and dependencies.
- [x] SCNB v1 compatibility is explicit; no bytes are intentionally changed.
- [x] Candidate iterator behavior is documented: raw candidates, opaque cursor,
      budgets, cancellation, snapshot invalidation, and idempotent close.
- [x] Corruption contract is documented: recover torn append-only tails,
      reject structural/checksum corruption, and rebuild disposable caches.
- [x] Rollback keeps the source corpus authoritative and replaces caches only
      after verification.

## Verification matrix

Run with Go 1.27.1 from `go.mod`:

- [ ] Linux amd64: `go test ./...`
- [ ] Linux amd64: `go vet ./...`
- [ ] Linux amd64: `go test -race ./...`
- [ ] Linux arm64: `go test ./...`
- [ ] Linux arm64: `go vet ./...`
- [ ] Compile darwin/amd64: `GOOS=darwin GOARCH=amd64 go build ./...`
- [ ] Compile darwin/arm64: `GOOS=darwin GOARCH=arm64 go build ./...`
- [ ] Compile windows/amd64: `GOOS=windows GOARCH=amd64 go build ./...`
- [ ] Compile freebsd/amd64: `GOOS=freebsd GOARCH=amd64 go build ./...`
- [ ] Confirm `.github/workflows/ci.yml` parses and matches this matrix.

## Publication gate

- [ ] Review the final diff and confirm the repository is clean before release.
- [ ] Create the `v0.1.0` tag using the approved release commit.
- [ ] Push the tag and release commit through the normal maintainer workflow.
- [ ] Confirm the module proxy can resolve `github.com/stowcloud/namesearch@v0.1.0`.
- [ ] Replace release-candidate wording with published-release wording in a
      follow-up documentation commit, if needed.
