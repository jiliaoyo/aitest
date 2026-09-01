package catalog

type Level struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"-"`
}

type Subject struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"-"`
}

type Exam struct {
	ID       string    `json:"id"`
	Code     string    `json:"code"`
	Name     string    `json:"name"`
	Levels   []Level   `json:"levels"`
	Subjects []Subject `json:"subjects"`
}

// KnowledgePoint 是管理端的知识点模型；学习端带个人统计的视图在 learning 模块。
type KnowledgePoint struct {
	ID             string  `json:"id"`
	ExamID         string  `json:"examId"`
	LevelID        string  `json:"levelId"`
	SubjectID      string  `json:"subjectId"`
	ParentID       *string `json:"parentId"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	CommonMistakes string  `json:"commonMistakes"`
	Examples       string  `json:"examples"`
	Status         string  `json:"status"`
	QuestionCount  int     `json:"questionCount"`
}
