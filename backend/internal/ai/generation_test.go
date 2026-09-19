package ai

import (
	"encoding/json"
	"testing"

	"github.com/aishuati/backend/internal/learning"
)

const testAIExplanation = "原文翻译：这是测试题。\n答案依据：根据题目判断。"

func TestValidateGeneratedQuestionsRejectsUnapprovedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは＿＿＿練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   testAIExplanation, KnowledgePointIDs: []string{"unapproved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyMixed, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err == nil {
		t.Fatal("expected unapproved knowledge point to be rejected")
	}
}

func TestValidateGeneratedQuestionsAcceptsReviewedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは＿＿＿別の練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   testAIExplanation, KnowledgePointIDs: []string{"approved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyMixed, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
		t.Fatalf("valid generated question rejected: %v", err)
	}
}

func TestValidateGeneratedQuestionsRejectsChoiceWithoutBlank(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "図書館で日本語を勉強します。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "を"}, {ID: "b", Label: "B", Text: "で"},
			{ID: "c", Label: "C", Text: "に"}, {ID: "d", Label: "D", Text: "へ"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["b"]}`),
		Explanation:   testAIExplanation, KnowledgePointIDs: []string{"approved"},
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err == nil {
		t.Fatal("choice question without a blank should be rejected")
	}
}

func TestChoiceStemHasBlank(t *testing.T) {
	for _, test := range []struct {
		stem string
		want bool
	}{
		{"図書館＿＿＿日本語を勉強します。", true},
		{"図書館___日本語を勉強します。", true},
		{"図書館（　）日本語を勉強します。", true},
		{"図書館で日本語を勉強します。", false},
		{"（としょかん）で勉強します。", false},
	} {
		if got := choiceStemHasBlank(test.stem); got != test.want {
			t.Fatalf("choiceStemHasBlank(%q) = %v, want %v", test.stem, got, test.want)
		}
	}
}

func generatedQuestionForReuseTest(stem, optionText string) generatedQuestion {
	return generatedQuestion{
		Type: "single_choice", Stem: stem, Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: optionText}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
	}
}

func TestGeneratedQuestionReuseKeyUsesFullQuestion(t *testing.T) {
	first := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一")
	spacingVariant := generatedQuestionForReuseTest("図書館 ＿＿＿ 日本語を勉強します。", "一")
	differentOptions := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "へ")
	if generatedQuestionReuseKey("level", "subject", first) != generatedQuestionReuseKey("level", "subject", spacingVariant) {
		t.Fatal("whitespace-only stem changes should reuse the same key")
	}
	if generatedQuestionReuseKey("level", "subject", first) == generatedQuestionReuseKey("level", "subject", differentOptions) {
		t.Fatal("different options must remain different questions")
	}
	shuffled := first
	shuffled.Options = append([]generatedOption(nil), first.Options...)
	if err := remapGeneratedChoiceOptions(&shuffled, []int{2, 0, 3, 1}); err != nil {
		t.Fatal(err)
	}
	if generatedQuestionReuseKey("level", "subject", first) != generatedQuestionReuseKey("level", "subject", shuffled) {
		t.Fatal("option shuffling must not change the reuse key")
	}
}

func TestFilterGeneratedQuestionDuplicatesReturnsOnlyExactMatches(t *testing.T) {
	questions := []generatedQuestion{
		generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一"),
		generatedQuestionForReuseTest("駅＿＿＿本を読みます。", "一"),
	}
	existing := generatedQuestionReuseKey("level", "subject", questions[0])
	filtered, duplicates, err := filterGeneratedQuestionDuplicates(questions, "level", "subject", nil, []string{existing}, nil)
	if err != nil || len(filtered) != 1 || filtered[0].Stem != "駅＿＿＿本を読みます。" || len(duplicates) != 1 {
		t.Fatalf("unexpected filtered questions: %+v, duplicates: %v, error: %v", filtered, duplicates, err)
	}
	merged := appendUniqueGeneratedStems([]string{"駅＿＿＿本を読みます。"}, duplicates)
	if len(merged) != 2 {
		t.Fatalf("unexpected merged stems: %v", merged)
	}
}

func TestGeneratedQuestionDeduplicationRejectsSameStemWithDifferentOptions(t *testing.T) {
	first := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一")
	variant := generatedQuestionForReuseTest(first.Stem, "五")
	first.Explanation, variant.Explanation = testAIExplanation, testAIExplanation
	questions := []generatedQuestion{first, variant, first}

	if err := validateGeneratedQuestions(questions, 3, generatedDifficultyNormal, generatedQuestionTypeMixed, nil); err != nil {
		t.Fatalf("valid questions with a repeated stem should reach full-question deduplication: %v", err)
	}
	filtered, duplicates, err := filterGeneratedQuestionDuplicates(questions, "level", "subject", nil, nil, nil)
	if err != nil || len(filtered) != 1 || len(duplicates) != 2 {
		t.Fatalf("unexpected filtered questions: %+v, duplicates: %v, error: %v", filtered, duplicates, err)
	}
}

func TestGeneratedQuestionDeduplicationRejectsTemplateSwap(t *testing.T) {
	questions := []generatedQuestion{
		generatedQuestionForReuseTest("教室＿＿＿日本語を勉強します。", "一"),
		generatedQuestionForReuseTest("駅＿＿＿本を読みます。", "一"),
	}
	filtered, duplicates, err := filterGeneratedQuestionDuplicates(questions, "level", "subject", nil, nil,
		[]string{"図書館＿＿＿日本語を勉強します。"})
	if err != nil || len(filtered) != 1 || filtered[0].Stem != "駅＿＿＿本を読みます。" || len(duplicates) != 1 {
		t.Fatalf("unexpected filtered questions: %+v, duplicates: %v, error: %v", filtered, duplicates, err)
	}
}

func TestGeneratedDiversityPlanRotatesContextsAndKnowledgePoints(t *testing.T) {
	points := []learning.AIGenerationKnowledgePoint{{ID: "kp-1"}, {ID: "kp-2"}, {ID: "kp-3"}}
	plan := generatedDiversityPlan(10, 0, "00", points, true)
	contexts := map[string]bool{}
	pointCounts := map[string]int{}
	for _, slot := range plan {
		contexts[slot.Context] = true
		pointCounts[slot.KnowledgePointID]++
	}
	if len(contexts) != 10 || pointCounts["kp-1"] != 4 || pointCounts["kp-2"] != 3 || pointCounts["kp-3"] != 3 {
		t.Fatalf("unexpected diversity plan: %+v", plan)
	}
}

func TestValidateGeneratedQuestionCandidatesKeepsValidQuestions(t *testing.T) {
	valid := generatedQuestionForReuseTest("図書館＿＿＿日本語を勉強します。", "一")
	valid.Explanation = testAIExplanation
	invalid := generatedQuestionForReuseTest("教室＿＿＿日本語を勉強します。", "一")
	invalid.Options = invalid.Options[:3]
	invalid.Explanation = testAIExplanation

	accepted, err := validateGeneratedQuestionCandidates([]generatedQuestion{valid, invalid}, 2,
		generatedDifficultyNormal, "single_choice", nil, "n5", "grammar", "mixed", nil, nil)
	if err == nil || len(accepted) != 1 || accepted[0].Stem != valid.Stem {
		t.Fatalf("unexpected accepted questions: %+v, error: %v", accepted, err)
	}
}

func TestValidateGeneratedQuestionsAllowsUnmatchedKnowledgePoint(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "これは＿＿＿知識点なしの練習問題です。", Difficulty: 3,
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "一"}, {ID: "b", Label: "B", Text: "二"},
			{ID: "c", Label: "C", Text: "三"}, {ID: "d", Label: "D", Text: "四"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`),
		Explanation:   testAIExplanation,
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, generatedQuestionTypeMixed, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
		t.Fatalf("unmatched knowledge point should be allowed: %v", err)
	}
}

func TestQuestionTypeMatches(t *testing.T) {
	for _, test := range []struct {
		mode, questionType string
		want               bool
	}{
		{generatedQuestionTypeMixed, "single_choice", true},
		{generatedQuestionTypeMixed, "short_answer", true},
		{"single_choice", "single_choice", true},
		{"single_choice", "fill_blank", false},
		{"fill_blank", "unknown", false},
	} {
		if got := questionTypeMatches(test.mode, test.questionType); got != test.want {
			t.Fatalf("questionTypeMatches(%q, %q) = %v, want %v", test.mode, test.questionType, got, test.want)
		}
	}
}

func TestValidateGeneratedQuestionsAcceptsTextQuestionTypes(t *testing.T) {
	for _, test := range []struct {
		questionType  string
		correctAnswer string
	}{
		{"fill_blank", `{"acceptable":["食べます"]}`},
		{"short_answer", `{"reference":"日本語を勉強します。"}`},
	} {
		question := generatedQuestion{
			Type: test.questionType, Stem: "日本語の練習問題です。", Difficulty: 3,
			CorrectAnswer: json.RawMessage(test.correctAnswer), Explanation: testAIExplanation, KnowledgePointIDs: []string{"approved"},
		}
		if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, test.questionType, []learning.AIGenerationKnowledgePoint{{ID: "approved"}}); err != nil {
			t.Fatalf("valid %s question rejected: %v", test.questionType, err)
		}
	}
}

func TestNormalizeGeneratedQuestionAnswersAcceptsTextForShortAnswer(t *testing.T) {
	questions := []generatedQuestion{{Type: "short_answer", CorrectAnswer: json.RawMessage(`{"text":"日本語で答えます。"}`)}}
	if err := normalizeGeneratedQuestionAnswers(questions); err != nil {
		t.Fatal(err)
	}
	if got := string(questions[0].CorrectAnswer); got != `{"reference":"日本語で答えます。"}` {
		t.Fatalf("normalized answer = %s", got)
	}
}

func TestNormalizeGeneratedQuestionAnswersRejectsEmptyShortAnswer(t *testing.T) {
	questions := []generatedQuestion{{Type: "short_answer", CorrectAnswer: json.RawMessage(`{"reference":null}`)}}
	if err := normalizeGeneratedQuestionAnswers(questions); err == nil {
		t.Fatal("empty reference should be rejected")
	}
}

func TestValidateGeneratedReadingQuestionsUsesMultipleSharedMaterials(t *testing.T) {
	materials := []*generatedMaterial{
		{Title: "通知", Content: "これは日本語の読解練習に使う共有材料です。駅の案内について説明しています。利用時間と注意事項も詳しく書かれています。"},
		{Title: "案内", Content: "これは別の日本語読解材料です。図書館の利用方法と予約できる時間について説明しています。"},
	}
	questions := make([]generatedQuestion, 6)
	for i := range questions {
		questions[i] = generatedQuestion{Type: "single_choice", Stem: "材料の内容について問う。", Material: materials[i/3]}
	}
	if err := validateGeneratedReadingQuestions("reading", "mixed", questions); err != nil {
		t.Fatalf("multiple shared materials should be valid: %v", err)
	}
	for i := range questions {
		questions[i].Material = materials[0]
	}
	if err := validateGeneratedReadingQuestions("reading", "mixed", append(questions, questions[:4]...)); err == nil {
		t.Fatal("a 10-question reading batch should not use one material")
	}
	for i := range questions {
		questions[i].Material = materials[0]
	}
	questions[5].Material = materials[1]
	if err := validateGeneratedReadingQuestions("reading", "mixed", questions); err == nil {
		t.Fatal("each material should be shared by multiple questions")
	}
	lopsided := make([]generatedQuestion, 10)
	for i := range lopsided {
		material := materials[1]
		if i < 6 {
			material = materials[0]
		}
		lopsided[i] = generatedQuestion{Type: "single_choice", Stem: "材料の内容について問う。", Material: material}
	}
	if err := validateGeneratedReadingQuestions("reading", "mixed", lopsided); err != nil {
		t.Fatalf("non-uniform but reasonable distribution should be valid: %v", err)
	}
	lopsided[6].Material = materials[0]
	if err := validateGeneratedReadingQuestions("reading", "mixed", lopsided); err == nil {
		t.Fatal("one material should not dominate a reading batch")
	}
}

func TestValidateGeneratedQuestionsAllowsNonBlankReadingChoice(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "本文の内容と合っているものはどれですか。", Difficulty: 3,
		Material: &generatedMaterial{Title: "案内", Content: "これは日本語の読解練習に使う共有材料です。駅の案内について説明しています。利用時間と注意事項も詳しく書かれています。"},
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "正しい答え"}, {ID: "b", Label: "B", Text: "別の答え"},
			{ID: "c", Label: "C", Text: "別の答え"}, {ID: "d", Label: "D", Text: "別の答え"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a"]}`), Explanation: testAIExplanation,
	}
	if err := validateGeneratedQuestions([]generatedQuestion{question}, 1, generatedDifficultyNormal, "single_choice", nil, "reading", "mixed"); err != nil {
		t.Fatalf("reading comprehension choice should not require a blank: %v", err)
	}
}

func TestRemapGeneratedChoiceOptionsUpdatesAnswer(t *testing.T) {
	question := generatedQuestion{
		Type: "multiple_choice",
		Options: []generatedOption{
			{ID: "a", Label: "A", Text: "甲"}, {ID: "b", Label: "B", Text: "乙"},
			{ID: "c", Label: "C", Text: "丙"}, {ID: "d", Label: "D", Text: "丁"},
		},
		CorrectAnswer: json.RawMessage(`{"optionIds":["a","c"]}`),
	}

	if err := remapGeneratedChoiceOptions(&question, []int{2, 0, 3, 1}); err != nil {
		t.Fatal(err)
	}
	if got, want := question.Options[0].Text, "丙"; got != want {
		t.Fatalf("first option text = %q, want %q", got, want)
	}
	var answer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if err := json.Unmarshal(question.CorrectAnswer, &answer); err != nil {
		t.Fatal(err)
	}
	if got, want := answer.OptionIDs, []string{"b", "a"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("remapped answer = %v, want %v", got, want)
	}
}

func TestDifficultyMatches(t *testing.T) {
	for _, test := range []struct {
		mode       string
		difficulty int
		want       bool
	}{
		{generatedDifficultyEasy, 2, true},
		{generatedDifficultyEasy, 3, false},
		{generatedDifficultyNormal, 3, true},
		{generatedDifficultyHard, 4, true},
		{generatedDifficultyHard, 2, false},
		{generatedDifficultyMixed, 5, true},
	} {
		if got := difficultyMatches(test.mode, test.difficulty); got != test.want {
			t.Fatalf("difficultyMatches(%q, %d) = %v, want %v", test.mode, test.difficulty, got, test.want)
		}
	}
}

func TestValidGeneratedCategory(t *testing.T) {
	for _, test := range []struct {
		category string
		want     bool
	}{
		{"mixed", true},
		{"grammar_case_particle", true},
		{"vocabulary_counter", true},
		{"reading_author", true},
		{"grammar_unknown", false},
	} {
		if got := validGeneratedCategory(test.category); got != test.want {
			t.Fatalf("validGeneratedCategory(%q) = %v, want %v", test.category, got, test.want)
		}
	}
}

func TestMixedQuestionTypePlanIsBalanced(t *testing.T) {
	for _, count := range []int{10, 20, 30} {
		plan := mixedQuestionTypePlan(count)
		if len(plan) != 4 {
			t.Fatalf("mixed plan for %d questions has %d types", count, len(plan))
		}
		total := 0
		for _, size := range plan {
			total += size
		}
		if total != count {
			t.Fatalf("mixed plan total = %d, want %d", total, count)
		}
	}
}

func TestValidateGeneratedQuestionTypePlanRejectsAllSingleChoice(t *testing.T) {
	questions := make([]generatedQuestion, 20)
	for i := range questions {
		questions[i].Type = "single_choice"
	}
	if err := validateGeneratedQuestionTypePlan(questions, generatedQuestionTypeMixed, mixedQuestionTypePlan(20)); err == nil {
		t.Fatal("all-single response should not satisfy mixed question plan")
	}
}

func TestValidateGeneratedQuestionLevelRejectsN5PurposeExpression(t *testing.T) {
	question := generatedQuestion{
		Type: "single_choice", Stem: "日本語を勉強する＿＿＿、日本へ行きます。",
		Options: []generatedOption{{ID: "a", Text: "ために"}, {ID: "b", Text: "から"}, {ID: "c", Text: "ので"}, {ID: "d", Text: "まで"}},
	}
	if err := validateGeneratedQuestionLevel("n5", "grammar", []generatedQuestion{question}); err == nil {
		t.Fatal("N5 should reject ～ために")
	}
}

func TestValidGeneratedCategoryForLevel(t *testing.T) {
	if validGeneratedCategoryForLevel("grammar_condition", "n5") {
		t.Fatal("N5 should not expose condition category")
	}
	if validGeneratedCategoryForLevel("grammar_voice", "n5") || validGeneratedCategoryForLevel("reading_style", "n2") {
		t.Fatal("lower levels should not accept higher-level categories")
	}
	if !validGeneratedCategoryForLevel("grammar_condition", "n4") || !validGeneratedCategoryForLevel("grammar_voice", "n3") {
		t.Fatal("level-specific category should be allowed at its minimum level")
	}
	if !validGeneratedCategoryForLevel("reading_style", "n1") {
		t.Fatal("N1 should accept the advanced reading category")
	}
}

func TestValidGeneratedCategoryForSubject(t *testing.T) {
	if validGeneratedCategoryForSubject("vocabulary_kanji", "grammar") || !validGeneratedCategoryForSubject("grammar_case_particle", "grammar") || !validGeneratedCategoryForSubject("mixed", "grammar") {
		t.Fatal("category and subject validation mismatch")
	}
}

func TestNormalizeGeneratedSelection(t *testing.T) {
	if got := normalizeStringList([]string{" vocab ", "grammar", "vocab", ""}); len(got) != 2 || got[0] != "grammar" || got[1] != "vocab" {
		t.Fatalf("normalized subjects = %v", got)
	}
	if got := normalizeGeneratedCategories([]string{"grammar_verb", "grammar_verb", " vocabulary_noun "}); len(got) != 2 || got[0] != "grammar_verb" || got[1] != "vocabulary_noun" {
		t.Fatalf("normalized categories = %v", got)
	}
	if got := normalizeGeneratedCategories([]string{"mixed", "grammar_verb"}); len(got) != 0 {
		t.Fatalf("mixed category should clear the selection: %v", got)
	}
}

func TestValidGeneratedCategoryForSubjects(t *testing.T) {
	if !validGeneratedCategoryForSubjects("grammar_case_particle", []string{"grammar", "vocabulary"}) {
		t.Fatal("category should match at least one selected subject")
	}
	if validGeneratedCategoryForSubjects("reading_logic", []string{"grammar", "vocabulary"}) {
		t.Fatal("category should be rejected when no selected subject matches")
	}
}

func TestCapGeneratedQuestionsDropsOnlyExtraQuestions(t *testing.T) {
	questions := []generatedQuestion{{Stem: "一"}, {Stem: "二"}, {Stem: "三"}}
	trimmed := capGeneratedQuestions(questions, 2)
	if len(trimmed) != 2 || trimmed[0].Stem != "一" || trimmed[1].Stem != "二" {
		t.Fatalf("unexpected capped questions: %+v", trimmed)
	}
	if same := capGeneratedQuestions(questions[:2], 2); len(same) != 2 {
		t.Fatalf("valid-sized response should be kept: %+v", same)
	}
}

func TestNonRetryableGenerationError(t *testing.T) {
	if !nonRetryableGenerationError(&httpResponseError{status: 401}) || nonRetryableGenerationError(&httpResponseError{status: 429}) || nonRetryableGenerationError(&httpResponseError{status: 502}) {
		t.Fatal("only configuration-like 4xx responses should stop generation immediately")
	}
	if !nonRetryableGenerationError(errNotConfigured) {
		t.Fatal("missing AI configuration should stop generation immediately")
	}
}
