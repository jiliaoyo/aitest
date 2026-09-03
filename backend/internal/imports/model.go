package imports

import (
	"encoding/json"

	"github.com/aishuati/backend/internal/content"
)

// 导入任务只接受结构化 JSON：上传即同步生成待审核草稿（review_ready），
// 没有异步阶段，也不存在 failed 状态。
const (
	StatusReviewReady = "review_ready"
	StatusPublished   = "published"

	ReviewPending   = "pending"
	ReviewApproved  = "approved"
	ReviewPublished = "published"
	ReviewRejected  = "rejected"
)

type Job struct {
	ID            string `json:"id"`
	FileName      string `json:"fileName"`
	MimeType      string `json:"mimeType"`
	SizeBytes     int64  `json:"sizeBytes"`
	Status        string `json:"status"`
	StageError    string `json:"stageError"`
	ExtractedText string `json:"extractedText,omitempty"`
	ItemCount     int    `json:"itemCount"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// Draft 是导入审核时唯一可编辑的结构化题目；AI 建议答案和来源答案分开保存。
type Draft struct {
	MaterialKey       string               `json:"materialKey,omitempty"`
	Type              string               `json:"type"`
	Stem              string               `json:"stem"`
	Options           []content.Option     `json:"options"`
	MaterialTitle     string               `json:"materialTitle,omitempty"`
	MaterialContent   string               `json:"materialContent,omitempty"`
	LevelID           string               `json:"levelId"`
	SubjectID         string               `json:"subjectId"`
	SourceSectionID   *string              `json:"sourceSectionId,omitempty"`
	Difficulty        int                  `json:"difficulty"`
	KnowledgePointIDs []string             `json:"knowledgePointIds"`
	Answer            *content.AnswerInput `json:"answer,omitempty"`
	SourceAnswer      *content.AnswerInput `json:"sourceAnswer,omitempty"`
	AISuggestedAnswer *AnswerSuggestion    `json:"aiSuggestedAnswer,omitempty"`
}

type AnswerSuggestion struct {
	Value       json.RawMessage `json:"value"`
	Explanation string          `json:"explanation"`
}

type Item struct {
	ID                  string   `json:"id"`
	JobID               string   `json:"jobId"`
	Position            int      `json:"position"`
	RawExcerpt          string   `json:"rawExcerpt"`
	Draft               *Draft   `json:"draft"`
	Anomalies           []string `json:"anomalies"`
	ReviewStatus        string   `json:"reviewStatus"`
	PublishedQuestionID *string  `json:"publishedQuestionId"`
	JobStatus           string   `json:"jobStatus"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type UpdateItemRequest struct {
	Draft Draft `json:"draft"`
}
