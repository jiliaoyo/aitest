package learning

import "time"

var reviewIntervals = [...]time.Duration{3 * 24 * time.Hour, 7 * 24 * time.Hour, 14 * 24 * time.Hour, 30 * 24 * time.Hour}

type reviewEvent struct {
	ResultID   string
	QuestionID string
	At         time.Time
	Status     string
}

type reviewPlan struct {
	QuestionID   string
	Stage        int
	NextReviewAt time.Time
	LastResultID string
	LastStatus   string
}

// buildReviewPlans 将不可变作答事实重放为可重建的复习计划；同一日提前答对不会加速阶段。
func buildReviewPlans(events []reviewEvent) []reviewPlan {
	plans := map[string]*reviewPlan{}
	for _, event := range events {
		plan := plans[event.QuestionID]
		if plan == nil {
			plan = &reviewPlan{QuestionID: event.QuestionID}
			plans[event.QuestionID] = plan
		}
		switch event.Status {
		case "incorrect", "unanswered":
			plan.Stage = 0
			plan.NextReviewAt = event.At.Add(24 * time.Hour)
		case "correct":
			// 只有曾经出错的题目进入计划；错误前的正确作答不推进阶段。
			if plan.LastStatus == "" || plan.NextReviewAt.IsZero() {
				continue
			}
			if event.At.Before(plan.NextReviewAt) {
				continue
			}
			if plan.Stage < len(reviewIntervals) {
				plan.Stage++
			}
			plan.NextReviewAt = event.At.Add(reviewIntervals[plan.Stage-1])
		}
		plan.LastResultID = event.ResultID
		plan.LastStatus = event.Status
	}
	out := make([]reviewPlan, 0, len(plans))
	for _, plan := range plans {
		if plan.LastStatus == "incorrect" || plan.LastStatus == "unanswered" || plan.Stage > 0 {
			out = append(out, *plan)
		}
	}
	return out
}
