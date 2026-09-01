package practice

import (
	"encoding/json"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const (
	StatusPending    = "pending"
	StatusCorrect    = "correct"
	StatusIncorrect  = "incorrect"
	StatusUnanswered = "unanswered"
	StatusFailed     = "failed"

	SourceDeterministic = "deterministic"
	SourceAI            = "ai"
)

// Outcome 是一次判分的纯结果，不含任何副作用。
type Outcome struct {
	Status       string
	Source       string // deterministic | ai
	Authority    string // official | human_verified | ""
	CorrectValue json.RawMessage
}

// StandardKey 是来自 answer_keys 的标准答案（value 与 authority 分离存储）。
type StandardKey struct {
	Value     json.RawMessage
	Authority string // official | human_verified
}

// Grade 按规范实现确定性判分（纯函数，无副作用）：
//   - 未作答：有权威答案记 unanswered（按错误计），无权威答案不调用 AI。
//   - 选择题按选项集合比较；填空按可接受值归一化比较。
//   - 简答题或无标准答案题返回 pending，由调用方创建 AI 任务。
func Grade(qType string, userValue json.RawMessage, key *StandardKey) Outcome {
	hasKey := key != nil && len(key.Value) > 0 && string(key.Value) != "null"
	if !answered(userValue) {
		if hasKey {
			return Outcome{Status: StatusUnanswered, Source: SourceDeterministic, Authority: key.Authority, CorrectValue: key.Value}
		}
		return Outcome{Status: StatusUnanswered, Source: SourceDeterministic}
	}
	if !hasKey || qType == "short_answer" {
		return Outcome{Status: StatusPending, Source: SourceAI}
	}
	if gradeObjective(qType, userValue, key.Value) {
		return Outcome{Status: StatusCorrect, Source: SourceDeterministic, Authority: key.Authority, CorrectValue: key.Value}
	}
	return Outcome{Status: StatusIncorrect, Source: SourceDeterministic, Authority: key.Authority, CorrectValue: key.Value}
}

func answered(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var v AnswerValue
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	if len(v.OptionIDs) > 0 {
		return true
	}
	return v.Text != nil && *v.Text != ""
}

func gradeObjective(qType string, userValue, keyValue json.RawMessage) bool {
	switch qType {
	case "single_choice", "multiple_choice":
		return sameOptionSet(userValue, keyValue)
	case "fill_blank":
		return fillBlankCorrect(userValue, keyValue)
	default:
		return false
	}
}

func sameOptionSet(userValue, keyValue json.RawMessage) bool {
	var u, k AnswerValue
	if json.Unmarshal(userValue, &u) != nil || json.Unmarshal(keyValue, &k) != nil {
		return false
	}
	if len(u.OptionIDs) != len(k.OptionIDs) {
		return false
	}
	set := map[string]bool{}
	for _, id := range k.OptionIDs {
		set[id] = true
	}
	for _, id := range u.OptionIDs {
		if !set[id] {
			return false
		}
	}
	return true
}

func fillBlankCorrect(userValue, keyValue json.RawMessage) bool {
	var u AnswerValue
	if json.Unmarshal(userValue, &u) != nil || u.Text == nil {
		return false
	}
	var k struct {
		Acceptable []string `json:"acceptable"`
	}
	if json.Unmarshal(keyValue, &k) != nil || len(k.Acceptable) == 0 {
		return false
	}
	got := NormalizeText(*u.Text)
	for _, acceptable := range k.Acceptable {
		if NormalizeText(acceptable) == got {
			return true
		}
	}
	return false
}

// NormalizeText 做首尾空白（含全角空格）与 Unicode NFKC 规范化，用于填空判分。
func NormalizeText(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "　")
	s = strings.Join(strings.Fields(s), " ")
	return norm.NFKC.String(s)
}
