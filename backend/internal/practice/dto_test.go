package practice

import (
	"encoding/json"
	"strings"
	"testing"
)

// 答题前 DTO 序列化后绝不能出现答案、解析或判分相关字段（规范 §18.1 / 前端规范 §10.1）。
func TestPreSubmitDTOLeak(t *testing.T) {
	savedAt := "2026-09-01T01:42:00Z"
	session := PreSubmitSession{
		ID:            "s1",
		Status:        "active",
		AnsweredCount: 1,
		TotalCount:    1,
		Items: []PreSubmitItem{{
			ID:              "i1",
			Position:        1,
			Type:            "single_choice",
			Material:        &PreSubmitMaterial{ID: "m1", Title: "阅读材料", Content: "…"},
			Stem:            "この店は、駅から近い（　　）、いつも混んでいる。",
			Options:         []PreSubmitOption{{ID: "a", Label: "A", Text: "にしては"}},
			SavedAnswer:     json.RawMessage(`{"optionIds":["a"]}`),
			MarkedForReview: true,
			SavedAt:         &savedAt,
		}},
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, banned := range []string{
		"correctAnswer", "answerKey", "answerAuthority", "authority",
		"gradingStatus", "gradingSource", "explanation", "correct", "analysis",
	} {
		if strings.Contains(s, banned) {
			t.Errorf("答题前 DTO 泄漏字段 %q: %s", banned, s)
		}
	}
}

// 结果 DTO 的分层字段必须齐全，且 AI 汇总不与正式正确率混合。
func TestResultDTOLayering(t *testing.T) {
	acc := 0.7778
	item := ResultItem{
		ID: "i1", GradingStatus: "incorrect",
		GradingSource: strPtr("deterministic"), AnswerAuthority: strPtr("human_verified"),
		CorrectAnswer: json.RawMessage(`{"optionIds":["c"]}`),
		Explanation:   &Explanation{Text: "…", Source: "ai"},
	}
	result := ResultSession{
		ID: "s1", Status: "grading",
		Summary: ResultSummary{
			Confirmed: &ConfirmedSummary{Correct: 14, Total: 18, Accuracy: &acc},
			AI:        &AISummary{Correct: 1, Completed: 1, Pending: 1, Failed: 0},
		},
		Items: []ResultItem{item},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, required := range []string{
		`"confirmed"`, `"ai"`, `"accuracy"`, `"gradingStatus"`, `"gradingSource"`,
		`"answerAuthority"`, `"correctAnswer"`, `"explanation"`,
	} {
		if !strings.Contains(s, required) {
			t.Errorf("结果 DTO 缺少字段 %s", required)
		}
	}
}

func strPtr(s string) *string { return &s }
