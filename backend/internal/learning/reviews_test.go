package learning

import (
	"testing"
	"time"
)

func TestBuildReviewPlans(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	plans := buildReviewPlans([]reviewEvent{
		{ResultID: "wrong", QuestionID: "q1", At: base, Status: "incorrect"},
		{ResultID: "early", QuestionID: "q1", At: base.Add(12 * time.Hour), Status: "correct"},
		{ResultID: "due", QuestionID: "q1", At: base.Add(24 * time.Hour), Status: "correct"},
	})
	if len(plans) != 1 || plans[0].Stage != 1 || !plans[0].NextReviewAt.Equal(base.Add(24*time.Hour+3*24*time.Hour)) {
		t.Fatalf("unexpected first review plan: %+v", plans)
	}
	plans = buildReviewPlans([]reviewEvent{
		{ResultID: "wrong", QuestionID: "q2", At: base, Status: "incorrect"},
		{ResultID: "wrong-again", QuestionID: "q2", At: base.Add(10 * 24 * time.Hour), Status: "incorrect"},
	})
	if len(plans) != 1 || plans[0].Stage != 0 || !plans[0].NextReviewAt.Equal(base.Add(11*24*time.Hour)) {
		t.Fatalf("repeated error should reset to 24 hours: %+v", plans)
	}
}
