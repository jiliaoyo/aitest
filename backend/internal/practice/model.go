package practice

import (
	"encoding/json"
	"fmt"

	"github.com/aishuati/backend/internal/content"
)

// ---------- 答案值 ----------

// AnswerValue 是用户/标准答案的统一结构；选择题用 optionIds，文字题用 text。
type AnswerValue struct {
	OptionIDs []string `json:"optionIds,omitempty"`
	Text      *string  `json:"text,omitempty"`
}

// ParseAnswerValue 校验并解析用户答案；非法答案返回错误。
func ParseAnswerValue(qType string, options []content.Option, raw json.RawMessage) (AnswerValue, error) {
	var v AnswerValue
	if len(raw) == 0 || string(raw) == "null" {
		return v, fmt.Errorf("答案为空")
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return v, fmt.Errorf("答案格式不正确")
	}
	switch qType {
	case "single_choice", "multiple_choice":
		if len(v.OptionIDs) == 0 {
			return v, fmt.Errorf("请选择至少一个选项")
		}
		valid := map[string]bool{}
		for _, o := range options {
			valid[o.ID] = true
		}
		seen := map[string]bool{}
		for _, id := range v.OptionIDs {
			if !valid[id] || seen[id] {
				return v, fmt.Errorf("答案引用了不存在的选项")
			}
			seen[id] = true
		}
		if qType == "single_choice" && len(v.OptionIDs) != 1 {
			return v, fmt.Errorf("单选题只能选择一个选项")
		}
	case "fill_blank", "short_answer":
		if v.Text == nil || *v.Text == "" {
			return v, fmt.Errorf("请填写答案")
		}
	default:
		return v, fmt.Errorf("未知题型")
	}
	return v, nil
}

// ---------- 答题前 DTO（严禁出现答案、解析、判分字段） ----------

type PreSubmitMaterial struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Content string `json:"content"`
}

type PreSubmitOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type PreSubmitItem struct {
	ID              string            `json:"id"`
	Position        int               `json:"position"`
	Type            string            `json:"type"`
	Material        *PreSubmitMaterial `json:"material"`
	Stem            string            `json:"stem"`
	Options         []PreSubmitOption `json:"options"`
	SavedAnswer     json.RawMessage   `json:"savedAnswer"`
	MarkedForReview bool              `json:"markedForReview"`
	SavedAt         *string           `json:"savedAt"`
}

type PreSubmitSession struct {
	ID            string          `json:"id"`
	Status        string          `json:"status"`
	AnsweredCount int             `json:"answeredCount"`
	TotalCount    int             `json:"totalCount"`
	Items         []PreSubmitItem `json:"items"`
}

// ---------- 答题后 DTO ----------

type ResultMaterial = PreSubmitMaterial

type ResultKnowledgePoint struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Explanation struct {
	Text   string `json:"text"`
	Source string `json:"source"` // official | human_verified | ai
}

type ResultItem struct {
	ID              string              `json:"id"`
	Position        int                 `json:"position"`
	Type            string              `json:"type"`
	Material        *ResultMaterial     `json:"material"`
	Stem            string              `json:"stem"`
	Options         []PreSubmitOption   `json:"options"`
	KnowledgePoints []ResultKnowledgePoint `json:"knowledgePoints"`
	UserAnswer      json.RawMessage     `json:"userAnswer"`
	GradingStatus   string              `json:"gradingStatus"`
	GradingSource   *string             `json:"gradingSource"`
	AnswerAuthority *string             `json:"answerAuthority"`
	CorrectAnswer   json.RawMessage     `json:"correctAnswer"`
	Explanation     *Explanation        `json:"explanation"`
}

type ConfirmedSummary struct {
	Correct  int      `json:"correct"`
	Total    int      `json:"total"`
	Accuracy *float64 `json:"accuracy"`
}

type AISummary struct {
	Correct   int `json:"correct"`
	Completed int `json:"completed"`
	Pending   int `json:"pending"`
	Failed    int `json:"failed"`
}

type ResultSummary struct {
	Confirmed *ConfirmedSummary `json:"confirmed"`
	AI        *AISummary        `json:"ai"`
}

type ResultSession struct {
	ID         string        `json:"id"`
	Status     string        `json:"status"`
	CreatedAt  string        `json:"createdAt"`
	SubmittedAt *string      `json:"submittedAt"`
	Summary    ResultSummary `json:"summary"`
	Items      []ResultItem  `json:"items"`
}

// ---------- 练习历史 ----------

type SessionListItem struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	TotalCount  int     `json:"totalCount"`
	CreatedAt   string  `json:"createdAt"`
	SubmittedAt *string `json:"submittedAt"`
}
