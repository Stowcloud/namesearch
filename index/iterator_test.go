package index

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestCandidatesOrderAndCursorResume(t *testing.T) {
	ix := openIndex(t, t.TempDir())
	entries := []Entry{{Namespace: 2, Path: "zeta.txt"}, {Namespace: 1, Path: "alpha.txt"}, {Namespace: 1, Path: "beta.txt"}}
	if err := ix.Append(entries); err != nil {
		t.Fatal(err)
	}
	tok := ix.BeginCoverage()
	if !ix.CompleteCoverage(tok) {
		t.Fatal("complete coverage")
	}
	it, plan, err := ix.Candidates(CandidateQuery{Needle: []byte("txt")})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Fallback != FallbackNone || it == nil {
		t.Fatalf("plan=%+v iterator=%v", plan, it)
	}
	first, err := it.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	cursor := it.Cursor()
	second, err := it.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Namespace != 1 || first.Path != "alpha.txt" || second.Namespace != 1 || second.Path != "beta.txt" {
		t.Fatalf("got %v then %v", first, second)
	}
	resume, _, err := ix.Candidates(CandidateQuery{Needle: []byte("txt"), Cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	got, err := resume.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != "beta.txt" {
		t.Fatalf("resumed path %q", got.Path)
	}
}

func TestCandidatesFallbackAndCursorValidation(t *testing.T) {
	ix := openIndex(t, t.TempDir())
	tok := ix.BeginCoverage()
	if !ix.CompleteCoverage(tok) {
		t.Fatal("complete coverage")
	}
	it, plan, err := ix.Candidates(CandidateQuery{Needle: []byte("ab")})
	if err != nil || it != nil || plan.Fallback != FallbackQueryTooShort {
		t.Fatalf("it=%v plan=%+v err=%v", it, plan, err)
	}
	_, _, err = ix.Candidates(CandidateQuery{Needle: []byte("abcdef"), Cursor: "bad"})
	if !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("cursor err=%v", err)
	}
}

func TestCandidatesBudgetAndCancellation(t *testing.T) {
	ix := openIndex(t, t.TempDir())
	if err := ix.Append([]Entry{{Path: "a.txt"}, {Path: "b.txt"}}); err != nil {
		t.Fatal(err)
	}
	tok := ix.BeginCoverage()
	if !ix.CompleteCoverage(tok) {
		t.Fatal("complete coverage")
	}
	it, _, err := ix.Candidates(CandidateQuery{Needle: []byte("txt"), Budget: Budget{MaxScannedEntries: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(context.Background()); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("budget err=%v", err)
	}
	if it.State() != IteratorBudgetExceeded {
		t.Fatalf("state=%v", it.State())
	}
	it, _, err = ix.Candidates(CandidateQuery{Needle: []byte("txt")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := it.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel err=%v", err)
	}
	if it.State() != IteratorCanceled {
		t.Fatalf("state=%v", it.State())
	}
}

func TestCandidatesExhaustion(t *testing.T) {
	ix := openIndex(t, t.TempDir())
	if err := ix.Append([]Entry{{Path: "a.txt"}}); err != nil {
		t.Fatal(err)
	}
	tok := ix.BeginCoverage()
	if !ix.CompleteCoverage(tok) {
		t.Fatal("complete coverage")
	}
	it, _, err := ix.Candidates(CandidateQuery{Needle: []byte("txt")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := it.Next(context.Background()); !errors.Is(err, io.EOF) {
		t.Fatalf("exhaustion err=%v", err)
	}
	if it.State() != IteratorExhausted {
		t.Fatalf("state=%v", it.State())
	}
}
