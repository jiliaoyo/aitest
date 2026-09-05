package admin

import "testing"

func TestParseDateRange(t *testing.T) {
	rangeValue, err := ParseDateRange("2026-09-01", "2026-09-05")
	if err != nil || rangeValue.From == nil || rangeValue.To == nil {
		t.Fatalf("valid date range rejected: %+v, %v", rangeValue, err)
	}
	if _, err := ParseDateRange("2026-09-06", "2026-09-05"); err == nil {
		t.Fatal("reversed date range should be rejected")
	}
	if _, err := ParseDateRange("bad", ""); err == nil {
		t.Fatal("invalid date should be rejected")
	}
}

func TestAIUsageKeepsTokenAndCostBreakdown(t *testing.T) {
	usage := aiUsage(3, 2, 1, 1, 100, 40, 900, 3, 0.25)
	if usage.TotalTokens != 140 || usage.EstimatedCostUSD == nil || *usage.EstimatedCostUSD != 0.25 {
		t.Fatalf("unexpected AI usage: %+v", usage)
	}
}

func TestParseCursorAcceptsPostgresTimestamp(t *testing.T) {
	createdAt, id, err := parseCursor("2026-09-05 12:00:00.123456+00\x00user-id")
	if err != nil || createdAt == "" || id != "user-id" {
		t.Fatalf("postgres cursor rejected: %q, %q, %v", createdAt, id, err)
	}
}
