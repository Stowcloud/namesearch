package namesearch

import (
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type fixtureReader struct {
	entries map[string][]Entry
	stats   map[string]Stat
}

func (r fixtureReader) ReadDir(path string, visit func(Entry) bool) error {
	for _, e := range r.entries[path] {
		if !visit(e) {
			break
		}
	}
	return nil
}
func (r fixtureReader) Stat(path string) (Stat, error) {
	st, ok := r.stats[path]
	if !ok {
		return Stat{}, errors.New("missing fixture stat")
	}
	return st, nil
}

func corpus(t *testing.T, namespace uint32, names ...string) Source {
	t.Helper()
	r := fixtureReader{entries: map[string][]Entry{"": {}}, stats: map[string]Stat{}}
	for _, name := range names {
		parts := strings.Split(strings.TrimSuffix(name, "/"), "/")
		for i := range parts {
			parent := strings.Join(parts[:i], "/")
			child := parts[i]
			path := strings.Join(parts[:i+1], "/")
			isDir := i < len(parts)-1 || strings.HasSuffix(name, "/")
			kind := EntryFile
			if isDir {
				kind = EntryDir
			}
			seen := false
			for _, e := range r.entries[parent] {
				if e.Name == child {
					seen = true
					break
				}
			}
			if !seen {
				r.entries[parent] = append(r.entries[parent], Entry{Name: child, Kind: kind, Ino: uint64(len(r.stats) + 1)})
			}
			r.stats[path] = Stat{Dev: 1, Ino: uint64(len(r.stats) + 1), Size: 1, MTimeNs: 1, Kind: kind}
		}
	}
	return Source{Namespace: Namespace(namespace), Reader: r}
}

func paths(hits []Hit) []string {
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.Path)
	}
	return out
}
func has(hits []Hit, path string) bool {
	for _, h := range hits {
		if h.Path == path {
			return true
		}
	}
	return false
}

func TestWalkFindsMatchesAcrossASubtree(t *testing.T) {
	src := corpus(t, 1, "report.pdf", "a/report-2024.pdf", "a/b/notes.txt", "other.bin")
	res, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 2 || !has(res.Hits, "report.pdf") || !has(res.Hits, "a/report-2024.pdf") {
		t.Fatalf("hits %v", paths(res.Hits))
	}
}

func TestWalkAppliesAllowBeforeDescending(t *testing.T) {
	src := corpus(t, 1, "public/report.pdf", "private/report.pdf")
	src.Allow = func(path string, _ bool) bool { return !strings.HasPrefix(path, "private") }
	res, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Hits) != 1 || !has(res.Hits, "public/report.pdf") {
		t.Fatalf("hits %v", paths(res.Hits))
	}
}

func TestWalkCallsAllowConcurrently(t *testing.T) {
	src := corpus(t, 1, "a/report.txt", "b/report.txt", "c/report.txt")
	var calls atomic.Int64
	src.Allow = func(string, bool) bool { calls.Add(1); return true }
	if _, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report"), Threads: 4}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() == 0 {
		t.Fatal("Allow was never called")
	}
}

func TestWalkMetadataIsOptional(t *testing.T) {
	src := corpus(t, 1, "report.pdf")
	bare, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report")})
	if err != nil {
		t.Fatal(err)
	}
	if bare.Hits[0].Size != nil {
		t.Fatal("name-only walk returned metadata")
	}
	full, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report"), WithMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	if full.Hits[0].Size == nil {
		t.Fatal("metadata walk omitted size")
	}
}

func TestWalkAppliesPrefix(t *testing.T) {
	src := corpus(t, 1, "report.pdf")
	src.Prefix = "/share/"
	res, err := Walk(t.Context(), []Source{src}, WalkOptions{Needle: FoldString("report")})
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Hits[0].Path; got != "/share/report.pdf" {
		t.Fatalf("path %q", got)
	}
}

func TestScanCorpusAndWalkAgreeOnMembership(t *testing.T) {
	src := corpus(t, 1, "a.txt", "dir/", "dir/b.txt")
	walk, err := Walk(t.Context(), []Source{src}, WalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	scan, err := ScanCorpus(t.Context(), []Source{src}, ScanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if uint64(len(walk.Hits)) != scan.Stats.Files {
		t.Fatalf("walk %d scan %d", len(walk.Hits), scan.Stats.Files)
	}
}
