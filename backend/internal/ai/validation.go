package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aishuati/backend/internal/content"
)

const aiTranslationPrefix = "原文翻译："
const aiAnalysisPrefix = "\n答案依据："

var aiKnowledgePointUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// sanitizeAIExplanation 去掉学习者可见解析中的知识点内部 ID。
func sanitizeAIExplanation(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "知识点：") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(line, "知识点："))
		match := aiKnowledgePointUUID.FindStringIndex(value)
		if match == nil {
			continue
		}
		value = strings.TrimSpace(value[match[1]:])
		if runes := []rune(value); len(runes) >= 2 {
			if (runes[0] == '（' && runes[len(runes)-1] == '）') || (runes[0] == '(' && runes[len(runes)-1] == ')') {
				value = strings.TrimSpace(string(runes[1 : len(runes)-1]))
			}
		}
		lines[i] = "知识点：" + value
	}
	return strings.Join(lines, "\n")
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
