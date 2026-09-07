package ai

import (
	"strings"
	"testing"
)

func TestGeneratedAnswerFallbackKeepsCandidateAnswer(t *testing.T) {
	answer, explanation, ok := generatedAnswerFallback(batchAnalysisRow{
		GeneratedAnswer:      stringPtr(`{"optionIds":["a"]}`),
		GeneratedExplanation: stringPtr("出题时的语法说明。"),
	})
	if !ok || string(answer) != `{"optionIds":["a"]}` {
		t.Fatalf("unexpected fallback answer: %s, ok=%v", answer, ok)
	}
	if explanation == "" || !strings.Contains(explanation, "出题时生成的解析：") {
		t.Fatalf("fallback explanation should preserve generation context: %q", explanation)
	}
}

func TestGeneratedAnswerFallbackRejectsInvalidJSON(t *testing.T) {
	if _, _, ok := generatedAnswerFallback(batchAnalysisRow{GeneratedAnswer: stringPtr("not-json")}); ok {
		t.Fatal("invalid generated answer must not be exposed as a fallback")
	}
}

func stringPtr(value string) *string { return &value }
