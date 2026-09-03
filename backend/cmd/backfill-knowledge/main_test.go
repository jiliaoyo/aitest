package main

import (
	"os"
	"testing"
)

func TestWriteReviewsDoesNotRewriteUnchangedFile(t *testing.T) {
	path := t.TempDir() + "/knowledge_mapping_review.json"
	file := reviewFile{Version: 2, Items: []reviewItem{{
		Source: "database_fallback", Level: "n5", Subject: "grammar",
		KnowledgePointIDs: []string{"root"}, Method: "scope_fallback",
		Confidence: 0.5, ReviewStatus: "pending",
	}}}
	if err := writeReviews(path, file); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeReviews(path, file); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("writing unchanged review data should keep identical JSON")
	}
}
