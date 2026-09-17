package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aishuati/backend/internal/content"
)

const aiTranslationPrefix = "原文翻译："
const aiAnalysisPrefix = "\n答案依据："

// sanitizeAIExplanation 去掉学习者可见解析中已单独展示的知识点段落。
func sanitizeAIExplanation(text string) string {
	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "知识点：") || strings.HasPrefix(line, "知识点:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func validAIExplanation(text string) bool {
	text = strings.TrimSpace(text)
	translationEnd := strings.Index(text, aiAnalysisPrefix)
	return strings.HasPrefix(text, aiTranslationPrefix) && translationEnd > len(aiTranslationPrefix) &&
		strings.TrimSpace(text[translationEnd+len(aiAnalysisPrefix):]) != "" && len([]rune(text)) <= 2000
}

// validateAICorrectAnswer checks the answer's meaning against the immutable question version.
// AI may explain or propose an answer, but it cannot invent an option ID or a malformed value.
func validateAICorrectAnswer(qType string, optionsText *string, raw json.RawMessage, correctness string) error {
	if correctness == "cannot_determine" {
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return fmt.Errorf("AI 判定缺少参考答案")
	}
	if qType == "single_choice" || qType == "multiple_choice" {
		if optionsText == nil {
			return fmt.Errorf("选择题缺少选项")
		}
		var options []content.Option
		if err := json.Unmarshal([]byte(*optionsText), &options); err != nil {
			return fmt.Errorf("题目选项格式不合法: %w", err)
		}
		if err := content.ValidateAnswerValue(qType, options, raw); err != nil {
			return fmt.Errorf("AI 参考答案不合法: %w", err)
		}
		return nil
	}
	var answer struct {
		Text string `json:"text"`
	}
	if err := strictDecode(raw, &answer); err != nil || answer.Text == "" {
		return fmt.Errorf("AI 文字参考答案不合法")
	}
	return nil
}
