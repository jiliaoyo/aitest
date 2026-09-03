package content

import "testing"

func TestSourceOrderLessKeepsNullsLast(t *testing.T) {
	first := "2026-01-01"
	second := "2026-01-02"
	book := "book"
	selfMade := "self_made"
	one := 1
	two := 2

	if !sourceOrderLess(SelectedQuestion{QuestionID: "b", SourceCreatedAt: &first, SourceOrder: &one}, SelectedQuestion{QuestionID: "a", SourceCreatedAt: &first, SourceOrder: &two}) {
		t.Fatal("lower source order should come first")
	}
	if !sourceOrderLess(SelectedQuestion{QuestionID: "a", SourceCreatedAt: &first, SourceOrder: &two}, SelectedQuestion{QuestionID: "b", SourceCreatedAt: &second, SourceOrder: &one}) {
		t.Fatal("older source should come first")
	}
	if !sourceOrderLess(SelectedQuestion{QuestionID: "a", SourceKind: &book, SourceCreatedAt: &second}, SelectedQuestion{QuestionID: "b", SourceKind: &selfMade, SourceCreatedAt: &first}) {
		t.Fatal("book questions should come before self-made questions")
	}
	if sourceOrderLess(SelectedQuestion{QuestionID: "a"}, SelectedQuestion{QuestionID: "b", SourceCreatedAt: &first, SourceOrder: &one}) {
		t.Fatal("questions without source order should come last")
	}
}
