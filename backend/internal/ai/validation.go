package ai

import (
	"encoding/json"
	"fmt"

	"github.com/aishuati/backend/internal/content"
)

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
