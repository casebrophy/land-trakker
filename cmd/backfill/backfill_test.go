package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

type fakeQuerier struct {
	ids []int64
}

func (f *fakeQuerier) QueryEligibleRawFetchIDs(_ context.Context, _, _ string) ([]int64, error) {
	return f.ids, nil
}

func TestDryRunReturnsCount(t *testing.T) {
	q := &fakeQuerier{ids: []int64{10, 20, 30}}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n, err := dryRun(context.Background(), q, "landwatch", "landwatch.v1", log)
	if err != nil {
		t.Fatalf("dryRun: %v", err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
}

func TestDryRunEmptySource(t *testing.T) {
	q := &fakeQuerier{ids: nil}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n, err := dryRun(context.Background(), q, "unused-source", "v1", log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}
