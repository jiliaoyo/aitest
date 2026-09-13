package ai

import "testing"

func TestValidateAICorrectAnswer(t *testing.T) {
	options := stringPtr(`[ {"id":"a","label":"A","text":"甲"}, {"id":"b","label":"B","text":"乙"} ]`)
	if err := validateAICorrectAnswer("single_choice", options, []byte(`{"optionIds":["a"]}`), "correct"); err != nil {
		t.Fatalf("valid choice rejected: %v", err)
	}
	if err := validateAICorrectAnswer("single_choice", options, []byte(`{"optionIds":["x"]}`), "incorrect"); err == nil {
		t.Fatal("unknown option accepted")
	}
	if err := validateAICorrectAnswer("fill_blank", nil, []byte(`{"text":"に"}`), "correct"); err != nil {
		t.Fatalf("valid text answer rejected: %v", err)
	}
	if err := validateAICorrectAnswer("short_answer", nil, []byte(`{"reference":"参考"}`), "correct"); err == nil {
		t.Fatal("reference-shaped AI answer accepted instead of text protocol")
	}
	if err := validateAICorrectAnswer("short_answer", nil, nil, "cannot_determine"); err != nil {
		t.Fatalf("cannot_determine should permit missing answer: %v", err)
	}
}

func TestValidAIExplanationRequiresOriginalTranslation(t *testing.T) {
	if !validAIExplanation("原文翻译：这是测试题。\n答案依据：测试。") {
		t.Fatal("explanation with original translation rejected")
	}
	if validAIExplanation("答案依据：缺少原文翻译。") {
		t.Fatal("explanation without original translation accepted")
	}
	if validQuestionExplanationPrompt("practice_batch_analysis.v4") {
		t.Fatal("old cached explanation without translation accepted")
	}
}

func TestGeneratedAnswerFallbackKeepsCandidateAnswer(t *testing.T) {
	answer, explanation, ok := generatedAnswerFallback(batchAnalysisRow{
		GeneratedAnswer:      stringPtr(`{"optionIds":["a"]}`),
		GeneratedExplanation: stringPtr("出题时的语法说明。"),
	})
	if !ok || string(answer) != `{"optionIds":["a"]}` {
		t.Fatalf("unexpected fallback answer: %s, ok=%v", answer, ok)
	}
	if explanation != "出题时的语法说明。" {
		t.Fatalf("fallback explanation should preserve generation context: %q", explanation)
	}
}

func TestGeneratedAnswerFallbackRejectsInvalidJSON(t *testing.T) {
	if _, _, ok := generatedAnswerFallback(batchAnalysisRow{GeneratedAnswer: stringPtr("not-json")}); ok {
		t.Fatal("invalid generated answer must not be exposed as a fallback")
	}
}

func stringPtr(value string) *string { return &value }
