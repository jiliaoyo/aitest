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

func TestBuildAIDraftCarriesFields(t *testing.T) {
	raw := jsonItem{RawExcerpt: "原文", Type: "single_choice", Stem: "题干",
		LevelCode: "n5", SubjectCode: "grammar", KnowledgePointNames: []string{"助词"}}
	draft := buildAIDraft(raw, "level-id", "subject-id", []string{"kp-id"}, []string{"知识点未唯一匹配，已跳过: X"})
	if draft.LevelID != "level-id" || draft.SubjectID != "subject-id" {
		t.Fatalf("codes not resolved into draft: %+v", draft)
	}
	if len(draft.KnowledgePointIDs) != 1 || draft.KnowledgePointIDs[0] != "kp-id" {
		t.Fatalf("knowledge points not carried: %+v", draft.KnowledgePointIDs)
	}
	if len(draft.Anomalies) != 1 {
		t.Fatalf("extra anomalies not carried: %+v", draft.Anomalies)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("  あいうえお  ", 3); got != "あいう" {
		t.Fatalf("unexpected truncation: %q", got)
	}
}
