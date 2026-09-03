package content

import (
	"context"
	"testing"
)

func TestValidateInputRejectsOutOfRangeDifficulty(t *testing.T) {
	fields := ValidateInput(QuestionInput{
		Type: "single_choice", Stem: "题干", Options: []Option{{ID: "a", Text: "甲"}, {ID: "b", Text: "乙"}},
		LevelID: "level", SubjectID: "subject", Difficulty: 6,
	})
	if fields["difficulty"] == "" {
		t.Fatal("expected a field error for out-of-range difficulty")
	}
}

type scopeChecker struct{}

func (scopeChecker) KnowledgePointExists(context.Context, string) (bool, error) { return true, nil }
func (scopeChecker) KnowledgePointMatchesQuestionScope(context.Context, string, string, string) (bool, error) {
	return false, nil
}

func TestCheckReferencesRejectsMismatchedKnowledgePoint(t *testing.T) {
	s := &Service{catalogKP: scopeChecker{}}
	err := s.checkReferences(context.Background(), QuestionInput{
		LevelID: "question-level", SubjectID: "question-subject", KnowledgePointIDs: []string{"kp"},
	})
	if err == nil {
		t.Fatal("expected mismatched knowledge point to be rejected")
	}
}
