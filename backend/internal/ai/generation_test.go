package ai

import (
	"encoding/json"
	"testing"

	"github.com/aishuati/backend/internal/learning"
)

func TestValidateGeneratedQuestionsRejectsUnapprovedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   "根据该知识点判断。", KnowledgePointIDs: []string{"unapproved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err == nil {
		t.Fatal("expected unapproved knowledge point to be rejected")
	}
}

func TestValidateGeneratedQuestionsAcceptsReviewedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは別の練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   "根据该知识点判断。", KnowledgePointIDs: []string{"approved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
		t.Fatalf("valid generated question rejected: %v", err)
	}
}
