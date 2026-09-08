package ai

import (
	"encoding/json"
	"testing"

	"github.com/aishuati/backend/internal/learning"
)

func TestValidateGeneratedQuestionsRejectsUnapprovedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは＿＿＿練習問題です。", Difficulty: 3,
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
		Type: "single_choice", Stem: "これは＿＿＿別の練習問題です。", Difficulty: 3,
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

func TestValidateGeneratedQuestionsRejectsChoiceWithoutBlank(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "図書館で日本語を勉強します。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "を"}, {ID: "b", Label: "B", Text: "で"},
			{ID: "c", Label: "C", Text: "に"}, {ID: "d", Label: "D", Text: "へ"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["b"]}`),
		Explanation:   "这是测试解析。", KnowledgePointIDs: []string{"approved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err == nil {
		t.Fatal("choice question without a blank should be rejected")
	}
}

func TestChoiceStemHasBlank(t *testing.T) {
	for _, test := range []struct {
		stem string
		want bool
	}{
		{"図書館＿＿＿日本語を勉強します。", true},
		{"図書館___日本語を勉強します。", true},
		{"図書館（　）日本語を勉強します。", true},
		{"図書館で日本語を勉強します。", false},
		{"（としょかん）で勉強します。", false},
	} {
		if got := choiceStemHasBlank(test.stem); got != test.want {
			t.Fatalf("choiceStemHasBlank(%q) = %v, want %v", test.stem, got, test.want)
		}
	}
}

func generatedQuestionForReuseTest(stem, optionText string) generatedQuestion {
	return generatedQuestion{
		Type: "single_choice", Stem: stem, Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: optionText}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
	}
}

func TestGeneratedQuestionReuseKeyUsesFullQuestion(t *testing.T) {
	first := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一")
	spacingVariant := generatedQuestionForReuseTest("図書館 ＿＿＿ 日本語を勉強します。", "一")
	differentOptions := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "へ")
	if generatedQuestionReuseKey("level", "subject", first) != generatedQuestionReuseKey("level", "subject", spacingVariant) {
		t.Fatal("whitespace-only stem changes should reuse the same key")
	}
	if generatedQuestionReuseKey("level", "subject", first) == generatedQuestionReuseKey("level", "subject", differentOptions) {
		t.Fatal("different options must remain different questions")
	}
	shuffled := first
	shuffled.Options = append([]generatedOption(nil), first.Options...)
	if err := remapGeneratedChoiceOptions(&shuffled, []int{2, 0, 3, 1}); err != nil {
		t.Fatal(err)
	}
	if generatedQuestionReuseKey("level", "subject", first) != generatedQuestionReuseKey("level", "subject", shuffled) {
		t.Fatal("option shuffling must not change the reuse key")
	}
}

func TestFilterGeneratedQuestionDuplicatesReturnsOnlyExactMatches(t *testing.T) {
	questions := []generatedQuestion{
		generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一"),
		generatedQuestionForReuseTest("駅＿＿＿本を読みます。", "一"),
	}
	existing := generatedQuestionReuseKey("level", "subject", questions[0])
	filtered, duplicates, err := filterGeneratedQuestionDuplicates(questions, "level", "subject", nil, []string{existing})
	if err != nil || len(filtered) != 1 || filtered[0].Stem != "駅＿＿＿本を読みます。" || len(duplicates) != 1 {
		t.Fatalf("unexpected filtered questions: %+v, duplicates: %v, error: %v", filtered, duplicates, err)
	}
	merged := appendUniqueGeneratedStems([]string{"駅＿＿＿本を読みます。"}, duplicates)
	if len(merged) != 2 {
		t.Fatalf("unexpected merged stems: %v", merged)
	}
}

func TestGeneratedQuestionDeduplicationKeepsValidQuestions(t *testing.T) {
	first := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一")
	variant := generatedQuestionForReuseTest(first.Stem, "五")
	first.Explanation, variant.Explanation = "这是第一题解析。", "这是第二题解析。"
	questions := []generatedQuestion{first, variant, first}

	if err := validateGeneratedQuestions(questions, 3, generatedDifficultyNormal, generatedQuestionTypeMixed, nil); err != nil {
		t.Fatalf("valid questions with a repeated stem should reach full-question deduplication: %v", err)
	}
	filtered, duplicates, err := filterGeneratedQuestionDuplicates(questions, "level", "subject", nil, nil)
	if err != nil || len(filtered) != 2 || len(duplicates) != 1 {
		t.Fatalf("unexpected filtered questions: %+v, duplicates: %v, error: %v", filtered, duplicates, err)
	}
}

func TestValidateGeneratedQuestionsAllowsUnmatchedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは＿＿＿知識点なしの練習問題です。", Difficulty: 3,
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

func TestRemapGeneratedChoiceOptionsUpdatesAnswer(t *testing.T) {
	question := generatedQuestion{
		Type: "multiple_choice",
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "甲"}, {ID: "b", Label: "B", Text: "乙"},
			{ID: "c", Label: "C", Text: "丙"}, {ID: "d", Label: "D", Text: "丁"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a","c"]}`),
	}

	if err := remapGeneratedChoiceOptions(&question, []int{2, 0, 3, 1}); err != nil {
		t.Fatal(err)
	}
	if got, want := question.Options[0].Text, "丙"; got != want {
		t.Fatalf("first option text = %q, want %q", got, want)
	}
	var answer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if err := json.Unmarshal(question.CorrectAnswer, &answer); err != nil {
		t.Fatal(err)
	}
	if got, want := answer.OptionIDs, []string{"b", "a"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("remapped answer = %v, want %v", got, want)
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

func TestCapGeneratedQuestionsDropsOnlyExtraQuestions(t *testing.T) {
	questions := []generatedQuestion{{Stem: "一"}, {Stem: "二"}, {Stem: "三"}}
	trimmed := capGeneratedQuestions(questions, 2)
	if len(trimmed) != 2 || trimmed[0].Stem != "一" || trimmed[1].Stem != "二" {
		t.Fatalf("unexpected capped questions: %+v", trimmed)
	}
	if same := capGeneratedQuestions(questions[:2], 2); len(same) != 2 {
		t.Fatalf("valid-sized response should be kept: %+v", same)
	}
}

func TestNonRetryableGenerationError(t *testing.T) {
	if !nonRetryableGenerationError(&httpResponseError{status: 401}) || nonRetryableGenerationError(&httpResponseError{status: 429}) || nonRetryableGenerationError(&httpResponseError{status: 502}) {
		t.Fatal("only configuration-like 4xx responses should stop generation immediately")
	}
	if !nonRetryableGenerationError(errNotConfigured) {
		t.Fatal("missing AI configuration should stop generation immediately")
	}
}
