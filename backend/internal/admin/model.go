package admin

import (
	"errors"
	"strings"
	"time"
)

const (
	roleLearner = "learner"
	roleAdmin   = "admin"
)

// DateRange 用于管理端统计；边界按用户本地选择的日期解释为 UTC 日期。
type DateRange struct {
	From *time.Time
	To   *time.Time
}

type UserListFilter struct {
	Query  string
	Role   string
	Cursor string
	Limit  int
	DateRange
}

func ParseDateRange(from, to string) (DateRange, error) {
	var out DateRange
	if from != "" {
		value, err := time.Parse("2006-01-02", from)
		if err != nil {
			return DateRange{}, errors.New("from 日期格式无效")
		}
		out.From = &value
	}
	if to != "" {
		value, err := time.Parse("2006-01-02", to)
		if err != nil {
			return DateRange{}, errors.New("to 日期格式无效")
		}
		out.To = &value
	}
	if out.From != nil && out.To != nil && out.From.After(*out.To) {
		return DateRange{}, errors.New("开始日期不能晚于结束日期")
	}
	return out, nil
}

func (r DateRange) args() []any { return []any{r.From, r.To} }

func validRole(role string) bool { return role == roleLearner || role == roleAdmin }

type UserUsage struct {
	ActiveDays               int     `json:"activeDays"`
	LastActiveAt             *string `json:"lastActiveAt"`
	LoginCount               int     `json:"loginCount"`
	ActiveAuthSessions       int     `json:"activeAuthSessions"`
	LastLoginAt              *string `json:"lastLoginAt"`
	PracticeSessions         int     `json:"practiceSessions"`
	CompletedSessions        int     `json:"completedSessions"`
	AnalysisFailedSessions   int     `json:"analysisFailedSessions"`
	GenerationFailedSessions int     `json:"generationFailedSessions"`
	ActiveSessions           int     `json:"activeSessions"`
	SubmittedSessions        int     `json:"submittedSessions"`
	PracticeItems            int     `json:"practiceItems"`
	AnsweredItems            int     `json:"answeredItems"`
	AIGenerationRequests     int     `json:"aiGenerationRequests"`
	AIGeneratedQuestions     int     `json:"aiGeneratedQuestions"`
	AI                       AIUsage `json:"ai"`
}

type AIUsage struct {
	Calls            int      `json:"calls"`
	SuccessfulCalls  int      `json:"successfulCalls"`
	FailedCalls      int      `json:"failedCalls"`
	GenerationCalls  int      `json:"generationCalls"`
	PromptTokens     int64    `json:"promptTokens"`
	CompletionTokens int64    `json:"completionTokens"`
	TotalTokens      int64    `json:"totalTokens"`
	DurationMs       int64    `json:"durationMs"`
	CostedCalls      int      `json:"costedCalls"`
	EstimatedCostUSD *float64 `json:"estimatedCostUsd"`
}

type AIUsageBreakdown struct {
	Key              string   `json:"key"`
	Calls            int      `json:"calls"`
	SuccessfulCalls  int      `json:"successfulCalls"`
	FailedCalls      int      `json:"failedCalls"`
	PromptTokens     int64    `json:"promptTokens"`
	CompletionTokens int64    `json:"completionTokens"`
	TotalTokens      int64    `json:"totalTokens"`
	DurationMs       int64    `json:"durationMs"`
	CostedCalls      int      `json:"costedCalls"`
	EstimatedCostUSD *float64 `json:"estimatedCostUsd"`
}

type AIDailyUsage struct {
	Date             string   `json:"date"`
	Calls            int      `json:"calls"`
	FailedCalls      int      `json:"failedCalls"`
	PromptTokens     int64    `json:"promptTokens"`
	CompletionTokens int64    `json:"completionTokens"`
	TotalTokens      int64    `json:"totalTokens"`
	DurationMs       int64    `json:"durationMs"`
	CostedCalls      int      `json:"costedCalls"`
	EstimatedCostUSD *float64 `json:"estimatedCostUsd"`
}

type UserListItem struct {
	ID               string    `json:"id"`
	Email            string    `json:"email"`
	Role             string    `json:"role"`
	DefaultLevelID   *string   `json:"defaultLevelId"`
	DefaultLevelCode string    `json:"defaultLevelCode"`
	DefaultLevelName string    `json:"defaultLevelName"`
	CreatedAt        string    `json:"createdAt"`
	LastActiveAt     *string   `json:"lastActiveAt"`
	LastLoginAt      *string   `json:"lastLoginAt"`
	Usage            UserUsage `json:"usage"`
}

type UserProfile struct {
	ID               string  `json:"id"`
	Email            string  `json:"email"`
	Role             string  `json:"role"`
	DefaultLevelID   *string `json:"defaultLevelId"`
	DefaultLevelCode string  `json:"defaultLevelCode"`
	DefaultLevelName string  `json:"defaultLevelName"`
	CreatedAt        string  `json:"createdAt"`
	LastActiveAt     *string `json:"lastActiveAt"`
	LastLoginAt      *string `json:"lastLoginAt"`
}

type UsersSummary struct {
	TotalUsers   int                `json:"totalUsers"`
	LearnerUsers int                `json:"learnerUsers"`
	AdminUsers   int                `json:"adminUsers"`
	NewUsers     int                `json:"newUsers"`
	ActiveUsers  int                `json:"activeUsers"`
	Usage        UserUsage          `json:"usage"`
	AIByKind     []AIUsageBreakdown `json:"aiByKind"`
	AIByModel    []AIUsageBreakdown `json:"aiByModel"`
	AIDaily      []AIDailyUsage     `json:"aiDaily"`
}

type UsersPage struct {
	Summary    UsersSummary   `json:"summary"`
	Users      []UserListItem `json:"users"`
	NextCursor string         `json:"nextCursor"`
}

type RecentAIRun struct {
	ID               string   `json:"id"`
	Kind             string   `json:"kind"`
	PromptVersion    string   `json:"promptVersion"`
	Model            string   `json:"model"`
	InputRef         string   `json:"inputRef"`
	Status           string   `json:"status"`
	PromptTokens     int      `json:"promptTokens"`
	CompletionTokens int      `json:"completionTokens"`
	TotalTokens      int      `json:"totalTokens"`
	DurationMs       int      `json:"durationMs"`
	EstimatedCostUSD *float64 `json:"estimatedCostUsd"`
	Error            string   `json:"error"`
	CreatedAt        string   `json:"createdAt"`
}

type RecentPracticeSession struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	Mode            string  `json:"mode"`
	RequestedCount  int     `json:"requestedCount"`
	TotalCount      int     `json:"totalCount"`
	AnsweredCount   int     `json:"answeredCount"`
	AISummaryStatus string  `json:"aiSummaryStatus"`
	CreatedAt       string  `json:"createdAt"`
	SubmittedAt     *string `json:"submittedAt"`
	DeletedAt       *string `json:"deletedAt"`
}

type UserDetail struct {
	User           UserProfile             `json:"user"`
	Usage          UserUsage               `json:"usage"`
	AIByKind       []AIUsageBreakdown      `json:"aiByKind"`
	AIByModel      []AIUsageBreakdown      `json:"aiByModel"`
	AIDaily        []AIDailyUsage          `json:"aiDaily"`
	RecentAIRuns   []RecentAIRun           `json:"recentAiRuns"`
	RecentPractice []RecentPracticeSession `json:"recentPracticeSessions"`
}

func aiUsage(calls, successful, failed, generationCalls int, prompt, completion, duration int64, costed int, estimated float64) AIUsage {
	var cost *float64
	if costed > 0 {
		cost = &estimated
	}
	return AIUsage{
		Calls: calls, SuccessfulCalls: successful, FailedCalls: failed, GenerationCalls: generationCalls,
		PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion,
		DurationMs: duration, CostedCalls: costed, EstimatedCostUSD: cost,
	}
}

func aiBreakdown(key string, calls, successful, failed int, prompt, completion, duration int64, costed int, estimated float64) AIUsageBreakdown {
	var cost *float64
	if costed > 0 {
		cost = &estimated
	}
	return AIUsageBreakdown{
		Key: key, Calls: calls, SuccessfulCalls: successful, FailedCalls: failed,
		PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion,
		DurationMs: duration, CostedCalls: costed, EstimatedCostUSD: cost,
	}
}

func normalizedQuery(value string) string { return strings.TrimSpace(value) }
