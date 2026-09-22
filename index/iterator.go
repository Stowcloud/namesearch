package index

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"

	search "github.com/stowcloud/namesearch"
)

// Candidate is a raw index match. It deliberately contains no product ACL,
// stat information, or result limit.
type Candidate struct {
	Namespace Namespace
	Path      string
	Name      string
	Score     float32
}

// Cursor is an opaque position in a candidate snapshot.
type Cursor = string

// Budget bounds index entries inspected by an iterator. Zero is unlimited.
type Budget struct {
	MaxScannedEntries uint64
}

// CandidateQuery describes one index candidate stream.
type CandidateQuery struct {
	Needle []byte
	Cursor Cursor
	Budget Budget
}

// CandidatePlan records the process-local snapshot and coverage state to which
// an iterator is bound. These values never alter SCNB bytes.
type CandidatePlan struct {
	SnapshotGeneration Generation
	CoverageGeneration Generation
	Completeness       Completeness
	Fallback           FallbackReason
}

// IteratorState is the lifecycle of a CandidateIterator.
type IteratorState uint8

const (
	IteratorRunning IteratorState = iota
	IteratorExhausted
	IteratorBudgetExceeded
	IteratorCanceled
	IteratorInvalidated
	IteratorClosed
)

var (
	ErrInvalidCursor       = errors.New("index: invalid cursor")
	ErrCursorStale         = errors.New("index: stale cursor")
	ErrBudgetExceeded      = errors.New("index: candidate scan budget exceeded")
	ErrSnapshotInvalidated = errors.New("index: candidate iterator snapshot is stale")
	ErrIteratorClosed      = errors.New("index: candidate iterator is closed")
)

// CandidateStats describes work performed by an iterator.
type CandidateStats struct {
	ScannedEntries      uint64
	Yielded             uint64
	CandidateBlocks     uint64
	FalsePositiveBlocks uint64
}

type cursorToken struct {
	Version   uint8  `json:"v"`
	QueryHash string `json:"q"`
	Instance  string `json:"i"`
	Snapshot  uint64 `json:"s"`
	Coverage  uint64 `json:"c"`
	Offset    uint64 `json:"o"`
}

type candidateSnapshot struct {
	candidates []Candidate
}

// CandidateIterator streams one deterministic, raw candidate snapshot.
type CandidateIterator struct {
	ix        *NameIndex
	folded    []byte
	queryHash string
	plan      CandidatePlan
	budget    Budget
	offset    uint64
	cursor    Cursor

	mu       sync.Mutex
	state    IteratorState
	prepared bool
	snap     candidateSnapshot
	stats    CandidateStats
}

// Candidates plans a candidate stream. Fallback is represented by a nil
// iterator and a non-zero plan.Fallback so callers can run their walk tier.
func (ix *NameIndex) Candidates(q CandidateQuery) (*CandidateIterator, CandidatePlan, error) {
	folded := search.Fold(q.Needle)
	hash := queryHash(folded)

	ix.mu.RLock()
	state := State{
		Completeness:       ix.completeness,
		SnapshotGeneration: Generation{Instance: ix.instance, Revision: ix.snapshotRevision},
		CoverageGeneration: Generation{Instance: ix.instance, Revision: ix.coverageRevision},
	}
	fallback := FallbackNone
	switch {
	case len(folded) < MinTrigramQuery:
		fallback = FallbackQueryTooShort
	case state.Completeness != Complete:
		fallback = FallbackIncomplete
	case ix.base != nil:
		_, fallback = ix.candidateKindsLocked(search.DistinctTrigrams(folded))
	}
	ix.mu.RUnlock()

	plan := CandidatePlan{
		SnapshotGeneration: state.SnapshotGeneration,
		CoverageGeneration: state.CoverageGeneration,
		Completeness:       state.Completeness,
		Fallback:           fallback,
	}

	var offset uint64
	if q.Cursor != "" {
		tok, err := decodeCursor(q.Cursor)
		if err != nil {
			return nil, CandidatePlan{}, err
		}
		if tok.QueryHash != hash {
			return nil, CandidatePlan{}, ErrCursorStale
		}
		if tok.Instance != instanceHex(state.SnapshotGeneration.Instance) ||
			tok.Snapshot != state.SnapshotGeneration.Revision ||
			tok.Coverage != state.CoverageGeneration.Revision {
			return nil, CandidatePlan{}, ErrCursorStale
		}
		offset = tok.Offset
	}
	if fallback != FallbackNone {
		return nil, plan, nil
	}

	it := &CandidateIterator{
		ix: ix, folded: append([]byte(nil), folded...), queryHash: hash,
		plan: plan, budget: q.Budget, offset: offset, state: IteratorRunning,
	}
	it.cursor = it.makeCursor(offset)
	return it, plan, nil
}

// Next returns the next raw candidate, or io.EOF after exhaustion.
func (it *CandidateIterator) Next(ctx context.Context) (Candidate, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	it.mu.Lock()
	defer it.mu.Unlock()

	switch it.state {
	case IteratorExhausted:
		return Candidate{}, io.EOF
	case IteratorCanceled:
		return Candidate{}, context.Canceled
	case IteratorBudgetExceeded:
		return Candidate{}, ErrBudgetExceeded
	case IteratorInvalidated:
		return Candidate{}, ErrSnapshotInvalidated
	case IteratorClosed:
		return Candidate{}, ErrIteratorClosed
	}
	if err := ctx.Err(); err != nil {
		it.state = IteratorCanceled
		return Candidate{}, err
	}
	if err := it.checkCurrentStateLocked(); err != nil {
		it.state = IteratorInvalidated
		return Candidate{}, err
	}
	if !it.prepared {
		if err := it.prepareLocked(ctx); err != nil {
			return Candidate{}, err
		}
	}
	if it.offset >= uint64(len(it.snap.candidates)) {
		it.state = IteratorExhausted
		it.cursor = it.makeCursor(it.offset)
		return Candidate{}, io.EOF
	}
	if err := ctx.Err(); err != nil {
		it.state = IteratorCanceled
		return Candidate{}, err
	}
	candidate := it.snap.candidates[it.offset]
	it.offset++
	it.stats.Yielded++
	it.cursor = it.makeCursor(it.offset)
	return candidate, nil
}

// Cursor returns an opaque resume token for the next candidate.
func (it *CandidateIterator) Cursor() Cursor {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.cursor == "" {
		it.cursor = it.makeCursor(it.offset)
	}
	return it.cursor
}

// State returns the iterator lifecycle state.
func (it *CandidateIterator) State() IteratorState {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.state
}

// Stats returns a copy of iterator accounting.
func (it *CandidateIterator) Stats() CandidateStats {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.stats
}

// Close ends the stream. It is idempotent.
func (it *CandidateIterator) Close() error {
	it.mu.Lock()
	defer it.mu.Unlock()
	if it.state == IteratorClosed || it.state == IteratorExhausted {
		return nil
	}
	it.state = IteratorClosed
	return nil
}

func (it *CandidateIterator) makeCursor(offset uint64) Cursor {
	tok := cursorToken{
		Version: 1, QueryHash: it.queryHash,
		Instance: instanceHex(it.plan.SnapshotGeneration.Instance),
		Snapshot: it.plan.SnapshotGeneration.Revision,
		Coverage: it.plan.CoverageGeneration.Revision,
		Offset:   offset,
	}
	buf, _ := json.Marshal(tok)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func (it *CandidateIterator) checkCurrentStateLocked() error {
	it.ix.mu.RLock()
	defer it.ix.mu.RUnlock()
	if it.ix.instance != it.plan.SnapshotGeneration.Instance ||
		it.ix.snapshotRevision != it.plan.SnapshotGeneration.Revision ||
		it.ix.coverageRevision != it.plan.CoverageGeneration.Revision ||
		it.ix.completeness != it.plan.Completeness {
		return ErrSnapshotInvalidated
	}
	return nil
}

func (it *CandidateIterator) prepareLocked(ctx context.Context) error {
	it.ix.mu.RLock()
	defer it.ix.mu.RUnlock()
	if it.ix.instance != it.plan.SnapshotGeneration.Instance ||
		it.ix.snapshotRevision != it.plan.SnapshotGeneration.Revision ||
		it.ix.coverageRevision != it.plan.CoverageGeneration.Revision ||
		it.ix.completeness != it.plan.Completeness {
		it.state = IteratorInvalidated
		return ErrSnapshotInvalidated
	}

	seen := make(map[tombKey]struct{})
	var candidates []Candidate
	count := func(n uint64) error {
		it.stats.ScannedEntries += n
		if it.budget.MaxScannedEntries != 0 && it.stats.ScannedEntries > it.budget.MaxScannedEntries {
			it.state = IteratorBudgetExceeded
			return ErrBudgetExceeded
		}
		if err := ctx.Err(); err != nil {
			it.state = IteratorCanceled
			return err
		}
		return nil
	}
	if it.ix.base != nil {
		lists, fallback := it.basePostingListsLocked(search.DistinctTrigrams(it.folded))
		if fallback != FallbackNone {
			it.state = IteratorInvalidated
			return fmt.Errorf("index: candidate snapshot fallback changed: %s", fallback)
		}
		blocks := intersect(lists)
		it.stats.CandidateBlocks = uint64(len(blocks))
		for _, bid := range blocks {
			if err := ctx.Err(); err != nil {
				it.state = IteratorCanceled
				return err
			}
			entries, err := it.ix.base.Block(bid)
			if err != nil {
				it.state = IteratorInvalidated
				return err
			}
			before := len(candidates)
			for _, e := range entries {
				if err := count(1); err != nil {
					return err
				}
				if !matchesName(e.Path, it.folded) || it.ix.tombstonedLocked(e.Namespace, e.Path, 0) {
					continue
				}
				k := tombKey{namespace: e.Namespace, path: e.Path}
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = struct{}{}
				candidates = append(candidates, makeCandidate(e.Namespace, e.Path, it.folded))
			}
			if len(candidates) == before {
				it.stats.FalsePositiveBlocks++
			}
		}
	}
	for _, d := range it.ix.delta {
		if err := count(1); err != nil {
			return err
		}
		if !matchesName(d.path, it.folded) || it.ix.tombstonedLocked(d.namespace, d.path, d.seq) {
			continue
		}
		k := tombKey{namespace: d.namespace, path: d.path}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		candidates = append(candidates, makeCandidate(d.namespace, d.path, it.folded))
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		if candidates[i].Namespace != candidates[j].Namespace {
			return candidates[i].Namespace < candidates[j].Namespace
		}
		return candidates[i].Path < candidates[j].Path
	})
	it.snap.candidates = candidates
	it.prepared = true
	return nil
}

func (ix *NameIndex) candidateKindsLocked(tris []search.Trigram) ([][]uint32, FallbackReason) {
	if ix.base == nil {
		return nil, FallbackNone
	}
	lists := make([][]uint32, 0, len(tris))
	pruned := 0
	for _, t := range tris {
		kind, raw := ix.base.Lookup(t)
		switch kind {
		case LookupPruned:
			pruned++
		case LookupMissing:
			return nil, FallbackNone
		case LookupPostings:
			ids, err := search.Ascending(raw)
			if err != nil {
				return nil, FallbackNone
			}
			lists = append(lists, ids)
		}
	}
	if len(lists) == 0 && pruned == len(tris) && ix.base.BlockCount > 0 {
		return nil, FallbackAllTrigramsPruned
	}
	return lists, FallbackNone
}

func (it *CandidateIterator) basePostingListsLocked(tris []search.Trigram) ([][]uint32, FallbackReason) {
	return it.ix.candidateKindsLocked(tris)
}

func makeCandidate(namespace Namespace, path string, folded []byte) Candidate {
	hit := makeHit(namespace, path, folded)
	return Candidate{Namespace: hit.Namespace, Path: hit.Path, Name: hit.Name, Score: hit.Score}
}

func queryHash(folded []byte) string {
	sum := sha256.Sum256(folded)
	return hex.EncodeToString(sum[:])
}

func instanceHex(instance [16]byte) string { return hex.EncodeToString(instance[:]) }

func decodeCursor(raw Cursor) (cursorToken, error) {
	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return cursorToken{}, ErrInvalidCursor
	}
	var tok cursorToken
	if err := json.Unmarshal(buf, &tok); err != nil || tok.Version != 1 || tok.QueryHash == "" || tok.Instance == "" {
		return cursorToken{}, ErrInvalidCursor
	}
	if len(tok.QueryHash) != sha256.Size*2 || len(tok.Instance) != 32 {
		return cursorToken{}, ErrInvalidCursor
	}
	return tok, nil
}
