package learning

// KPStats 是单个知识点针对当前用户的聚合统计；数字全部由后端计算。
type KPStats struct {
	ConfirmedAnswered int     `json:"confirmedAnswered"`
	ConfirmedCorrect  int     `json:"confirmedCorrect"`
	RecentAnswered    int     `json:"recentAnswered"`
	RecentCorrect     int     `json:"recentCorrect"`
	AIAnswered        int     `json:"aiAnswered"`
	AICorrect         int     `json:"aiCorrect"`
	ConsecutiveWrong  int     `json:"consecutiveWrong"`
	LastPracticedAt   *string `json:"lastPracticedAt,omitempty"`
}

type KnowledgePointItem struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	LevelID       string   `json:"levelId"`
	LevelCode     string   `json:"levelCode"`
	SubjectID     string   `json:"subjectId"`
	SubjectName   string   `json:"subjectName"`
	ParentID      *string  `json:"parentId"`
	QuestionCount int      `json:"questionCount"`
	Stats         *KPStats `json:"stats,omitempty"`
}

type KnowledgePointDetail struct {
	KnowledgePointItem
	Description    string `json:"description"`
	CommonMistakes string `json:"commonMistakes"`
	Examples       string `json:"examples"`
	Status         string `json:"status"`
}

type Recommendation struct {
	Type              string   `json:"type"` // knowledge | comprehensive
	KnowledgePointID  *string  `json:"knowledgePointId,omitempty"`
	Name              string   `json:"name"`
	RecentAnswered    int      `json:"recentAnswered"`
	RecentWrongCount  int      `json:"recentWrongCount"`
	Accuracy          *float64 `json:"accuracy,omitempty"`
	ConsecutiveWrong  int      `json:"consecutiveWrong"`
	SuggestedCount    int      `json:"suggestedCount"`
	Reason            string   `json:"reason"`
	KnowledgePointIDs []string `json:"knowledgePointIds"`
}

type ActiveSession struct {
	ID            string `json:"id"`
	AnsweredCount int    `json:"answeredCount"`
	TotalCount    int    `json:"totalCount"`
}

type Dashboard struct {
	ActiveSession   *ActiveSession   `json:"activeSession"`
	RecentSessions  []RecentSession  `json:"recentSessions"`
	Recommendations []Recommendation `json:"recommendations"`
	Comprehensive   *Recommendation  `json:"comprehensive,omitempty"`
	StatsEmpty      bool             `json:"statsEmpty"`
	Memory          LearningMemory   `json:"memory"`
}

type MemoryAdvice struct {
	Status    string  `json:"status"`
	Text      string  `json:"text"`
	UpdatedAt *string `json:"updatedAt,omitempty"`
}

// LearningMemory 是当前账号的可重置学习档案；原始练习记录不包含在删除范围内。
type LearningMemory struct {
	ConfirmedAnswered int          `json:"confirmedAnswered"`
	ConfirmedCorrect  int          `json:"confirmedCorrect"`
	AIAnswered        int          `json:"aiAnswered"`
	AICorrect         int          `json:"aiCorrect"`
	StatsUpdatedAt    *string      `json:"statsUpdatedAt,omitempty"`
	Advice            MemoryAdvice `json:"advice"`
}

type AIMemorySnapshot struct {
	ConfirmedAnswered int                 `json:"confirmedAnswered"`
	ConfirmedCorrect  int                 `json:"confirmedCorrect"`
	WeakPoints        []AIMemoryWeakPoint `json:"weakPoints"`
}

type AIMemoryWeakPoint struct {
	Name             string `json:"name"`
	RecentAnswered   int    `json:"recentAnswered"`
	RecentCorrect    int    `json:"recentCorrect"`
	ConsecutiveWrong int    `json:"consecutiveWrong"`
}

type RecentSession struct {
	ID          string  `json:"id"`
	Status      string  `json:"status"`
	TotalCount  int     `json:"totalCount"`
	CreatedAt   string  `json:"createdAt"`
	SubmittedAt *string `json:"submittedAt"`
}
