# Release notes

## Unreleased (pre-1.0)

- Added repository support, security, maintainer, and contribution policies.
- Documented the SCNB v1 compatibility contract, candidate-iterator boundary,
  dependency policy, and disposable-index rollback procedure.
- Added CI coverage for Linux amd64/arm64 tests, vet, and race checks plus
  non-Linux compile checks.
- No binaries, modules, or other release artifacts have been published.

### Compatibility

No SCNB v1 bytes are intentionally changed by this documentation and CI-only
change. The source corpus remains authoritative; deleting and rebuilding an
index is the supported rollback if an index cache is unusable.
