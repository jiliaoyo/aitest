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

	plans = buildReviewPlans([]reviewEvent{
		{ResultID: "ai-wrong", QuestionID: "q3", VersionID: "v1", At: base, Status: "incorrect", Source: "ai"},
		{ResultID: "ai-correct-1", QuestionID: "q3", VersionID: "v1", At: base.Add(24 * time.Hour), Status: "correct", Source: "ai"},
		{ResultID: "ai-correct-2", QuestionID: "q3", VersionID: "v1", At: base.Add(4 * 24 * time.Hour), Status: "correct", Source: "ai"},
		{ResultID: "ai-correct-3", QuestionID: "q3", VersionID: "v1", At: base.Add(11 * 24 * time.Hour), Status: "correct", Source: "ai"},
		{ResultID: "ai-correct-4", QuestionID: "q3", VersionID: "v1", At: base.Add(25 * 24 * time.Hour), Status: "correct", Source: "ai"},
	})
	if len(plans) != 1 || plans[0].Stage != 3 || !plans[0].NextReviewAt.Equal(base.Add(39*24*time.Hour)) {
		t.Fatalf("AI review stage should cap at 14 days: %+v", plans)
	}

	plans = buildReviewPlans([]reviewEvent{
		{ResultID: "old-wrong", QuestionID: "q4", VersionID: "v1", At: base, Status: "incorrect"},
		{ResultID: "new-correct", QuestionID: "q4", VersionID: "v2", At: base.Add(24 * time.Hour), Status: "correct"},
		{ResultID: "new-wrong", QuestionID: "q4", VersionID: "v2", At: base.Add(2 * 24 * time.Hour), Status: "incorrect"},
	})
	if len(plans) != 1 || plans[0].VersionID != "v2" || plans[0].Stage != 0 || !plans[0].NextReviewAt.Equal(base.Add(3*24*time.Hour)) {
		t.Fatalf("a new version should start a fresh review plan: %+v", plans)
	}
}
