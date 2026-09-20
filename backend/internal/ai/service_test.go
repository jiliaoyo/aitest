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
	if !validAIExplanation("原文翻译：这是测试题。\n答案依据：根据题干判断。") {
		t.Fatal("explanation with original translation rejected")
	}
	if validAIExplanation("原文翻译：这是测试题。\n答案依据：動作が行われる場所を表す時は「で」を使います。") {
		t.Fatal("Japanese explanation should be rejected")
	}
	if validAIExplanation("答案依据：缺少原文翻译。") {
		t.Fatal("explanation without original translation accepted")
	}
	if validAIExplanation("原文翻译：只有翻译，没有解析。") {
		t.Fatal("explanation without visible analysis accepted")
	}
	if validQuestionExplanationPrompt("practice_batch_analysis.v4") {
		t.Fatal("old cached explanation without translation accepted")
	}
	if validQuestionExplanationPrompt("practice_batch_analysis.v6") {
		t.Fatal("old cached explanation without Chinese-language guarantee accepted")
	}
	if !validQuestionExplanationPrompt("practice_batch_analysis.v7") {
		t.Fatal("current cached explanation prompt rejected")
	}
}

func TestRetryableAIExplanation(t *testing.T) {
	if !retryableAIExplanation("AI 解析语言异常，已留待重新分析。") {
		t.Fatal("language-failure marker should be retryable")
	}
	if retryableAIExplanation("原文翻译：测试。\n答案依据：根据题干判断。") {
		t.Fatal("valid explanation should not be retryable")
	}
}

func TestHasChineseExplanationAllowsJapaneseTermsInChineseProse(t *testing.T) {
	if !hasChineseExplanation("根据题干，助词「で」表示动作发生的场所。") {
		t.Fatal("Chinese explanation with Japanese term was rejected")
	}
	if hasChineseExplanation("動作が行われる場所を表す時は「で」を使います。") {
		t.Fatal("Japanese prose was accepted")
	}
}

func TestSanitizeAIExplanationRemovesKnowledgePointLine(t *testing.T) {
	got := sanitizeAIExplanation("原文翻译：测试。\n答案依据：根据题目判断。\n知识点：f60335ec-1d9a-5921-b6b8-f40903a0f030（ば（条件））\n常见误区：误选其他条件表达。")
	want := "原文翻译：测试。\n答案依据：根据题目判断。\n常见误区：误选其他条件表达。"
	if got != want {
		t.Fatalf("sanitized explanation = %q, want %q", got, want)
	}
}

func TestSanitizeAIExplanationRemovesKnowledgePointWithoutUUID(t *testing.T) {
	text := "原文翻译：测试。\n答案依据：根据题目判断。\n知识点: 条件表达\n常见误区：误选其他条件表达。"
	want := "原文翻译：测试。\n答案依据：根据题目判断。\n常见误区：误选其他条件表达。"
	if got := sanitizeAIExplanation(text); got != want {
		t.Fatalf("knowledge point line was not removed: %q", got)
	}
}

func TestSanitizeAIExplanationLeavesOrdinaryText(t *testing.T) {
	text := "原文翻译：测试。\n答案依据：根据题目判断。\n常见误区：误选其他条件表达。"
	if got := sanitizeAIExplanation(text); got != text {
		t.Fatalf("ordinary explanation changed: %q", got)
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
