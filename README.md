# namesearch

Standalone filename search primitives and a persistent trigram index.

The root package provides folding, filtering, ranking, trigrams, walking, and corpus estimation. The `index` subpackage provides the SCNB v1-compatible base, delta, and tombstone segments.

## Development

```sh
go test ./...
go vet ./...
```

The module intentionally contains no Stowcloud service, storage adapter, ACL, transport, or application dependencies.
