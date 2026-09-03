package imports

import (
	"encoding/json"
	"testing"

	"github.com/aishuati/backend/internal/content"
)

func TestValidateDraft(t *testing.T) {
	draft := Draft{
		Type: "single_choice", Stem: "これは問題です。", Options: []content.Option{
			{ID: "a", Label: "A", Text: "はい"}, {ID: "b", Label: "B", Text: "いいえ"},
		}, LevelID: "level", SubjectID: "subject", Difficulty: 3,
		Answer: &content.AnswerInput{Value: json.RawMessage(`{"optionIds":["a"]}`), Authority: content.AuthorityOfficial},
	}
	if err := validateDraft(&draft); err != nil {
		t.Fatalf("valid draft rejected: %v", err)
	}
	draft.Answer.Value = json.RawMessage(`{"optionIds":["missing"]}`)
	if err := validateDraft(&draft); err == nil {
		t.Fatal("invalid answer accepted")
	}
}

func TestCleanTextRejectsEmpty(t *testing.T) {
	if _, err := cleanText(" \n\t"); err == nil {
		t.Fatal("empty extracted text accepted")
	}
}
