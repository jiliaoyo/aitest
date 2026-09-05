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
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyMixed, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err == nil {
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
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyMixed, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
		t.Fatalf("valid generated question rejected: %v", err)
	}
}

func TestValidateGeneratedQuestionsAllowsUnmatchedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは知識点なしの練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   "这是没有匹配知识点的测试解析。",
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
		t.Fatalf("unmatched knowledge point should be allowed: %v", err)
	}
}

func TestQuestionTypeMatches(t *testing.T) {
	for _, test := range []struct {
		mode, questionType string
		want               bool
	}{
		{generatedQuestionTypeMixed, "single_choice", true},
		{generatedQuestionTypeMixed, "short_answer", true},
		{"single_choice", "single_choice", true},
		{"single_choice", "fill_blank", false},
		{"fill_blank", "unknown", false},
	} {
		if got := questionTypeMatches(test.mode, test.questionType); got != test.want {
			t.Fatalf("questionTypeMatches(%q, %q) = %v, want %v", test.mode, test.questionType, got, test.want)
		}
	}
}

func TestValidateGeneratedQuestionsAcceptsTextQuestionTypes(t *testing.T) {
	for _, test := range []struct {
		questionType  string
		correctAnswer string
	}{
		{"fill_blank", `{"acceptable":["食べます"]}`},
		{"short_answer", `{"reference":"日本語を勉強します。"}`},
	} {
		question := generatedQuestion{
			Type: test.questionType, Stem: "日本語の練習問題です。", Difficulty: 3,
			CorrectAnswer: json.RawMessage(test.correctAnswer), Explanation: "这是测试解析。", KnowledgePointIDs: []string{"approved"},
		}
		if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, test.questionType, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
			t.Fatalf("valid %s question rejected: %v", test.questionType, err)
		}
	}
}

func TestDifficultyMatches(t *testing.T) {
	for _, test := range []struct {
		mode       string
		difficulty int
		want       bool
	}{
		{generatedDifficultyEasy, 2, true},
		{generatedDifficultyEasy, 3, false},
		{generatedDifficultyNormal, 3, true},
		{generatedDifficultyHard, 4, true},
		{generatedDifficultyHard, 2, false},
		{generatedDifficultyMixed, 5, true},
	} {
		if got := difficultyMatches(test.mode, test.difficulty); got != test.want {
			t.Fatalf("difficultyMatches(%q, %d) = %v, want %v", test.mode, test.difficulty, got, test.want)
		}
	}
}

func TestValidGeneratedCategory(t *testing.T) {
	for _, test := range []struct {
		category string
		want     bool
	}{
		{"mixed", true},
		{"grammar_case_particle", true},
		{"vocabulary_counter", true},
		{"reading_author", true},
		{"grammar_unknown", false},
	} {
		if got := validGeneratedCategory(test.category); got != test.want {
			t.Fatalf("validGeneratedCategory(%q) = %v, want %v", test.category, got, test.want)
		}
	}
}
