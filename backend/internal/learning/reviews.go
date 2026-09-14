package learning

import "time"

var reviewIntervals = [...]time.Duration{3 * 24 * time.Hour, 7 * 24 * time.Hour, 14 * 24 * time.Hour, 30 * 24 * time.Hour}
var aiReviewIntervals = [...]time.Duration{3 * 24 * time.Hour, 7 * 24 * time.Hour, 14 * 24 * time.Hour}

type reviewEvent struct {
	ResultID   string
	QuestionID string
	VersionID  string
	At         time.Time
	Status     string
	Source     string
}

type reviewPlan struct {
	QuestionID   string
	VersionID    string
	Stage        int
	NextReviewAt time.Time
	LastResultID string
	LastStatus   string
}

func normalizeReviewSource(source string) string {
	if source == "ai" {
		return "ai"
	}
	return "confirmed"
}

func intervalsForReviewSource(source string) []time.Duration {
	if normalizeReviewSource(source) == "ai" {
		return aiReviewIntervals[:]
	}
	return reviewIntervals[:]
}

// buildReviewPlans 将不可变作答事实重放为可重建的复习计划；同一日提前答对不会加速阶段。
func buildReviewPlans(events []reviewEvent) []reviewPlan {
	plans := map[string]*reviewPlan{}
	for _, event := range events {
		source := normalizeReviewSource(event.Source)
		plan := plans[event.QuestionID]
		if plan == nil {
			plan = &reviewPlan{QuestionID: event.QuestionID, VersionID: event.VersionID}
			plans[event.QuestionID] = plan
		}
		if plan.VersionID != "" && event.VersionID != "" && plan.VersionID != event.VersionID {
			// 题目发布新版本后，旧版本的复习阶段不应迁移到新版本。
			*plan = reviewPlan{QuestionID: event.QuestionID, VersionID: event.VersionID}
		}
		switch event.Status {
		case "incorrect", "unanswered":
			plan.Stage = 0
			plan.NextReviewAt = event.At.Add(24 * time.Hour)
			plan.LastResultID = event.ResultID
			plan.LastStatus = event.Status
		case "correct":
			// 只有曾经出错的题目进入计划；错误前的正确作答不推进阶段。
			if plan.LastStatus == "" || plan.NextReviewAt.IsZero() {
				continue
			}
			if event.At.Before(plan.NextReviewAt) {
				continue
			}
			intervals := intervalsForReviewSource(source)
			if plan.Stage < len(intervals) {
				plan.Stage++
			}
			if plan.Stage > len(intervals) {
				plan.Stage = len(intervals)
			}
			plan.NextReviewAt = event.At.Add(intervals[plan.Stage-1])
			plan.LastResultID = event.ResultID
			plan.LastStatus = event.Status
		}
	}
	out := make([]reviewPlan, 0, len(plans))
	for _, plan := range plans {
		if !plan.NextReviewAt.IsZero() {
			out = append(out, *plan)
		}
	}
	return out
}
