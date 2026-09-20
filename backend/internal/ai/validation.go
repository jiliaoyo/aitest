package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aishuati/backend/internal/content"
)

const aiTranslationPrefix = "原文翻译："
const aiAnalysisPrefix = "\n答案依据："

var chineseExplanationMarkers = [...]string{
	"根据", "本题", "题干", "表示", "因为", "所以", "因此", "用于", "误用", "选择", "选项",
	"语法", "词义", "符合", "不能", "正确", "错误", "这里", "说明", "区别", "意思", "接在", "接续", "解析",
	"句意", "注意", "场景", "动作", "时间", "地点", "助词", "动词", "形容词", "名词", "连接",
}

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

func retryableAIExplanation(text string) bool {
	return strings.HasPrefix(strings.TrimSpace(text), "AI 解析语言异常")
}

// hasChineseExplanation 用少量中文说明词拦截整段日语；题干、答案词和例句仍可保留日语。
func hasChineseExplanation(text string) bool {
	for _, marker := range chineseExplanationMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func validAIExplanation(text string) bool {
	text = strings.TrimSpace(text)
	translationEnd := strings.Index(text, aiAnalysisPrefix)
	return strings.HasPrefix(text, aiTranslationPrefix) && translationEnd > len(aiTranslationPrefix) &&
		strings.TrimSpace(text[translationEnd+len(aiAnalysisPrefix):]) != "" &&
		hasChineseExplanation(text[translationEnd+len(aiAnalysisPrefix):]) && len([]rune(text)) <= 2000
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
