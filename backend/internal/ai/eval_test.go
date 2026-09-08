package ai

import "testing"

func TestOfflineEvalCases(t *testing.T) {
	report, err := RunOfflineEval()
	if err != nil {
		t.Fatal(err)
	}
	if report.Cases != 40 || report.Passed != report.Cases || report.FirstPassRate != 1 {
		t.Fatalf("offline AI eval did not pass all fixed cases: %+v", report)
	}
	if report.TotalCalls != report.Cases || report.TotalTokens <= 0 {
		t.Fatalf("offline AI eval should report replay cost inputs: %+v", report)
	}
	t.Logf("offline AI eval: cases=%d first_pass_rate=%.2f calls=%d total_tokens=%d", report.Cases, report.FirstPassRate, report.TotalCalls, report.TotalTokens)
}
