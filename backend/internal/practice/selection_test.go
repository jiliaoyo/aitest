package practice

import (
	"context"
	"testing"
)

func TestSelectionOrderDefaultsToSourceOrder(t *testing.T) {
	s := &Service{}
	f, err := s.selectionFilter(context.Background(), "user", CreateRequest{LevelID: "n5", Count: 10})
	if err != nil {
		t.Fatal(err)
	}
	if f.SelectionOrder != SelectionOrderSource {
		t.Fatalf("selection order = %q, want %q", f.SelectionOrder, SelectionOrderSource)
	}

	f, err = s.selectionFilter(context.Background(), "user", CreateRequest{
		LevelID: "n5", Count: 10, SelectionOrder: SelectionOrderRandom,
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.SelectionOrder != SelectionOrderRandom {
		t.Fatalf("selection order = %q, want %q", f.SelectionOrder, SelectionOrderRandom)
	}

	if _, err := s.selectionFilter(context.Background(), "user", CreateRequest{
		LevelID: "n5", Count: 10, SelectionOrder: "other",
	}); err == nil {
		t.Fatal("invalid selection order unexpectedly accepted")
	}
}

func TestSelectionFilterKeepsSource(t *testing.T) {
	f, err := (&Service{}).selectionFilter(context.Background(), "user", CreateRequest{
		LevelID: "n5", Count: 10, SourceID: "source-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.SourceID != "source-1" {
		t.Fatalf("source = %q, want source-1", f.SourceID)
	}
}
