package imports

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
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

func TestImportJSONRejectsTrailingContent(t *testing.T) {
	path := t.TempDir() + "/items.json"
	if err := os.WriteFile(path, []byte(`{"items":[]} {"items":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (&Service{}).importJSON(context.Background(), "admin", "items.json", path, "digest", "application/json", 1)
	if err == nil {
		t.Fatal("trailing JSON object should be rejected")
	}
	var apiErr *httpapi.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("unexpected trailing JSON error: %v", err)
	}
	details, ok := apiErr.Details.(map[string]any)
	fields, fieldsOK := details["fields"].(map[string]string)
	if !ok || !fieldsOK || !strings.Contains(fields["file"], "只能包含一个对象") {
		t.Fatalf("unexpected trailing JSON error: %v", err)
	}
}
