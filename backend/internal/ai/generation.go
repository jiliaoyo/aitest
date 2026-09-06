package ai

import (
	"context"
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

const questionGenerationPromptVersion = "practice_question_generation.v11"
const questionGenerationRetryPromptVersion = "practice_question_generation.v11.retry"

const questionGenerationRetryInstructions = `上一轮输出没有通过服务端结构校验。本轮必须重新生成完整的一组题目，不能只返回修改后的题目；请优先修正下面的服务端错误，并再次逐题检查题量、题型、答案结构和解析。`

//go:embed prompts/practice_question_generation.v11.md
var questionGenerationPrompt string

const (
	generatedDifficultyEasy    = "easy"
	generatedDifficultyNormal  = "normal"
	generatedDifficultyHard    = "hard"
	generatedDifficultyMixed   = "mixed"
	generationModeMemory       = "memory"
	generationModeLevel        = "level"
	generatedQuestionTypeMixed = "mixed"
	generatedCategoryMixed     = "mixed"
)

var generatedCategories = map[string]struct{}{
	generatedCategoryMixed:  {},
	"grammar_case_particle": {}, "grammar_conjunctive_particle": {}, "grammar_adverbial_particle": {}, "grammar_final_particle": {},
	"grammar_auxiliary": {}, "grammar_verb": {}, "grammar_adjective": {}, "grammar_adverb": {}, "grammar_conjunction": {},
	"grammar_adnominal": {}, "grammar_sentence_pattern": {}, "grammar_tense_aspect": {}, "grammar_condition": {},
	"grammar_voice": {}, "grammar_benefactive": {}, "grammar_honorific": {}, "grammar_negation": {},
	"vocabulary_kanji": {}, "vocabulary_noun": {}, "vocabulary_verb": {}, "vocabulary_adjective": {}, "vocabulary_adverb": {},
	"vocabulary_conjunction": {}, "vocabulary_pronoun": {}, "vocabulary_counter": {}, "vocabulary_time_number": {},
	"vocabulary_synonym": {}, "vocabulary_polysemy": {}, "vocabulary_collocation": {}, "vocabulary_compound": {},
	"vocabulary_affix": {}, "vocabulary_onoma": {}, "vocabulary_katakana": {}, "vocabulary_honorific": {}, "vocabulary_usage": {},
	"reading_information": {}, "reading_main_idea": {}, "reading_reference": {}, "reading_paraphrase": {}, "reading_logic": {},
	"reading_inference": {}, "reading_author": {}, "reading_vocabulary": {}, "reading_structure": {}, "reading_chart_notice": {}, "reading_style": {},
}

type AIGenerateRequest struct {
	LevelID           string   `json:"levelId"`
	SubjectID         string   `json:"subjectId"`
	KnowledgePointIDs []string `json:"knowledgePointIds"`
	Count             int      `json:"count"`
	Difficulty        string   `json:"difficulty"`
	GenerationMode    string   `json:"generationMode"`
	QuestionType      string   `json:"questionType"`
	ShowFurigana      bool     `json:"showFurigana"`
	Category          string   `json:"category"`
}

type AIGeneratedSession struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// RegisterRoutes 提供账号私有的 AI 个性化出题入口；生成结果仍通过普通练习接口答题。
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ai-practice-sessions", s.createGeneratedSession)
}

func (s *Service) createGeneratedSession(w http.ResponseWriter, r *http.Request) {
	var req AIGenerateRequest
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	session, err := s.CreateGeneratedSession(r.Context(), ctxkeys.UserID(r.Context()), req)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, session)
}

func (s *Service) CreateGeneratedSession(ctx context.Context, userID string, req AIGenerateRequest) (AIGeneratedSession, error) {
	if !s.client.Configured() {
		return AIGeneratedSession{}, httpapi.E(http.StatusServiceUnavailable, "ai_unavailable", "AI 出题服务暂不可用")
	}
	if req.Count == 0 {
		req.Count = 20
	}
	if !validGeneratedCount(req.Count) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"count": "题量只能是 10、20 或 30"})
	}
	if req.Difficulty == "" {
		req.Difficulty = generatedDifficultyMixed
	}
	if !validGeneratedDifficulty(req.Difficulty) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"difficulty": "难度必须是 easy、normal、hard 或 mixed"})
	}
	if req.GenerationMode == "" {
		req.GenerationMode = generationModeMemory
	}
	if !validGenerationMode(req.GenerationMode) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"generationMode": "生成依据必须是 memory 或 level"})
	}
	if req.QuestionType == "" {
		req.QuestionType = generatedQuestionTypeMixed
	}
	if !validGeneratedQuestionType(req.QuestionType) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"questionType": "题型不合法"})
	}
	if req.Category == "" {
		req.Category = generatedCategoryMixed
	}
	if !validGeneratedCategory(req.Category) {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"category": "出题分类不合法"})
	}
	if len(req.KnowledgePointIDs) > 10 {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"knowledgePointIds": "一次最多选择 10 个知识点"})
	}
	if req.LevelID == "" {
		if err := s.pool.QueryRow(ctx, `SELECT coalesce(default_level_id::text, '') FROM users WHERE id::text = $1`, userID).Scan(&req.LevelID); err != nil {
			return AIGeneratedSession{}, err
		}
	}
	if req.LevelID == "" {
		return AIGeneratedSession{}, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
	}
	if err := s.validateGenerationScope(ctx, req); err != nil {
		return AIGeneratedSession{}, err
	}
	scope, _ := json.Marshal(map[string]any{
		"mode":              "ai_generated",
		"subjectId":         req.SubjectID,
		"knowledgePointIds": req.KnowledgePointIDs,
		"difficulty":        req.Difficulty,
		"generationMode":    req.GenerationMode,
		"questionType":      req.QuestionType,
		"showFurigana":      req.ShowFurigana,
		"category":          req.Category,
	})
	var out AIGeneratedSession
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var subjectID any
		if req.SubjectID != "" {
			subjectID = req.SubjectID
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO practice_sessions (user_id, status, level_id, subject_id, scope, requested_count)
			 VALUES ($1, 'generating', $2, $3, $4, $5) RETURNING id::text`,
			userID, req.LevelID, subjectID, scope, req.Count).Scan(&out.ID); err != nil {
			return err
		}
		out.Status = "generating"
		return jobs.EnqueueTx(ctx, tx, "generate_ai_practice_session", map[string]string{"sessionId": out.ID})
	})
	return out, err
}

func validGeneratedCount(count int) bool { return count == 10 || count == 20 || count == 30 }

func validGeneratedDifficulty(difficulty string) bool {
	return difficulty == generatedDifficultyEasy || difficulty == generatedDifficultyNormal || difficulty == generatedDifficultyHard || difficulty == generatedDifficultyMixed
}

func validGenerationMode(mode string) bool {
	return mode == generationModeMemory || mode == generationModeLevel
}

func validGeneratedQuestionType(questionType string) bool {
	return questionType == generatedQuestionTypeMixed || content.ValidType(questionType)
}

func validGeneratedCategory(category string) bool {
	_, ok := generatedCategories[category]
	return ok
}

func (s *Service) validateGenerationScope(ctx context.Context, req AIGenerateRequest) error {
	var levelExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM exam_levels WHERE id::text = $1)`, req.LevelID).Scan(&levelExists); err != nil {
		return err
	}
	if !levelExists {
		return httpapi.ErrNotFound
	}
	if req.SubjectID != "" {
		var scopeExists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM exam_levels l JOIN subjects sub ON sub.exam_id = l.exam_id
			   WHERE l.id::text = $1 AND sub.id::text = $2)`, req.LevelID, req.SubjectID).Scan(&scopeExists); err != nil {
			return err
		}
		if !scopeExists {
			return httpapi.ErrNotFound
		}
	}
	if len(req.KnowledgePointIDs) > 0 {
		var count int
		if err := s.pool.QueryRow(ctx,
			`SELECT count(*) FROM knowledge_points
			 WHERE id::text = ANY($1::text[]) AND status = 'published'
			   AND level_id::text = $2 AND ($3 = '' OR subject_id::text = $3)`,
			req.KnowledgePointIDs, req.LevelID, req.SubjectID).Scan(&count); err != nil {
			return err
		}
		if count != len(uniqueStrings(req.KnowledgePointIDs)) {
			return httpapi.ErrNotFound
		}
		return nil
	}
	var count int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM knowledge_points
		 WHERE status = 'published' AND level_id::text = $1 AND ($2 = '' OR subject_id::text = $2)`,
		req.LevelID, req.SubjectID).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return httpapi.E(http.StatusConflict, "no_knowledge_points", "当前级别暂无可用于 AI 出题的知识点")
	}
	return nil
}

func uniqueStrings(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

type generationJobRequest struct {
	SessionID string `json:"sessionId"`
}

type questionGenerationInput struct {
	Count          int                         `json:"count"`
	LevelID        string                      `json:"levelId"`
	LevelCode      string                      `json:"levelCode"`
	SubjectID      string                      `json:"subjectId,omitempty"`
	Difficulty     string                      `json:"difficulty"`
	GenerationMode string                      `json:"generationMode"`
	QuestionType   string                      `json:"questionType"`
	ShowFurigana   bool                        `json:"showFurigana"`
	Category       string                      `json:"category"`
	RandomSeed     string                      `json:"randomSeed"`
	RetryFeedback  string                      `json:"retryFeedback,omitempty"`
	LearningMemory learning.AIGenerationMemory `json:"learningMemory"`
}

type generatedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type generatedQuestion struct {
	Type              string            `json:"type"`
	Stem              string            `json:"stem"`
	Options           []generatedOption `json:"options"`
	CorrectAnswer     json.RawMessage   `json:"correctAnswer"`
	Explanation       string            `json:"explanation"`
	KnowledgePointIDs []string          `json:"knowledgePointIds"`
	SubjectID         string            `json:"subjectId"`
	Difficulty        int               `json:"difficulty"`
}

type generatedQuestionResponse struct {
	Questions []generatedQuestion `json:"questions"`
}

type generationSessionRow struct {
	UserID         string
	LevelID        string
	LevelCode      string
	SubjectID      *string
	RequestedCount int
	Scope          string
	Status         string
}

func (s *Service) handleGenerate(ctx context.Context, attempts, maxAttempts int, payload json.RawMessage) error {
	var req generationJobRequest
	if err := strictDecode(payload, &req); err != nil {
		return err
	}
	var row generationSessionRow
	err := s.pool.QueryRow(ctx,
		`SELECT ps.user_id::text, ps.level_id::text, l.code, ps.subject_id::text, ps.requested_count, ps.scope::text, ps.status
		 FROM practice_sessions ps JOIN exam_levels l ON l.id = ps.level_id WHERE ps.id = $1`, req.SessionID,
	).Scan(&row.UserID, &row.LevelID, &row.LevelCode, &row.SubjectID, &row.RequestedCount, &row.Scope, &row.Status)
	if errors.Is(err, pgx.ErrNoRows) || row.Status == "active" {
		return nil
	}
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	if row.Status != "generating" {
		return nil
	}
	var scope struct {
		Mode              string   `json:"mode"`
		SubjectID         string   `json:"subjectId"`
		KnowledgePointIDs []string `json:"knowledgePointIds"`
		Difficulty        string   `json:"difficulty"`
		GenerationMode    string   `json:"generationMode"`
		QuestionType      string   `json:"questionType"`
		ShowFurigana      bool     `json:"showFurigana"`
		Category          string   `json:"category"`
		Script            string   `json:"script"`
	}
	if err := strictDecode([]byte(row.Scope), &scope); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("解析 AI 出题范围失败: %w", err))
	}
	subjectID := ""
	if row.SubjectID != nil {
		subjectID = *row.SubjectID
	}
	difficulty := scope.Difficulty
	if difficulty == "" {
		difficulty = generatedDifficultyMixed
	}
	if !validGeneratedDifficulty(difficulty) {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("AI 出题难度不合法"))
	}
	generationMode := scope.GenerationMode
	if generationMode == "" {
		generationMode = generationModeMemory
	}
	if !validGenerationMode(generationMode) {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("AI 出题依据不合法"))
	}
	questionType := scope.QuestionType
	if questionType == "" {
		questionType = generatedQuestionTypeMixed
	}
	if !validGeneratedQuestionType(questionType) {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("AI 题型不合法"))
	}
	category := scope.Category
	if category == "" {
		category = generatedCategoryMixed
	}
	if !validGeneratedCategory(category) {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("AI 出题分类不合法"))
	}
	memory, err := learning.NewStore(s.pool).GenerationMemoryForAI(ctx, row.UserID, row.LevelID, subjectID, scope.KnowledgePointIDs, generationMode)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("读取 AI 出题记忆失败: %w", err))
	}
	if len(memory.KnowledgePoints) == 0 {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, errors.New("没有可用于 AI 出题的已审核知识点"))
	}
	var response generatedQuestionResponse
	promptVersion := questionGenerationPromptVersion
	var validationErr error
	for localAttempt := 0; localAttempt < 2; localAttempt++ {
		seed, err := randomSeed()
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		systemPrompt := questionGenerationPrompt
		feedback := ""
		temperature := 0.6
		if validationErr != nil {
			promptVersion = questionGenerationRetryPromptVersion
			feedback = shortError(validationErr)
			systemPrompt += "\n\n" + questionGenerationRetryInstructions + "\n服务端校验错误：" + feedback
			temperature = 0.2
		}
		inputJSON, _ := json.Marshal(questionGenerationInput{
			Count: row.RequestedCount, LevelID: row.LevelID, LevelCode: row.LevelCode, SubjectID: subjectID, Difficulty: difficulty,
			GenerationMode: generationMode, QuestionType: questionType, ShowFurigana: scope.ShowFurigana, Category: category,
			RandomSeed: seed, RetryFeedback: feedback, LearningMemory: memory,
		})
		out, err := s.client.RunPromptWithTemperature(ctx, row.UserID, "practice_question_generation", promptVersion,
			req.SessionID, systemPrompt, string(inputJSON), temperature)
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		if err := strictDecode(out, &response); err != nil {
			validationErr = fmt.Errorf("AI 出题输出不合法: %w", err)
			continue
		}
		response.Questions = capGeneratedQuestions(response.Questions, row.RequestedCount)
		if err := validateGeneratedQuestions(response.Questions, row.RequestedCount, difficulty, questionType, memory.KnowledgePoints); err != nil {
			validationErr = err
			continue
		}
		validationErr = nil
		break
	}
	if validationErr != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, validationErr)
	}
	if err := shuffleGeneratedChoiceOptions(response.Questions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("打乱 AI 选项失败: %w", err))
	}
	if err := s.persistGeneratedQuestions(ctx, req.SessionID, row.UserID, row.LevelID, subjectID, generationMode, promptVersion, memory.KnowledgePoints, response.Questions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("保存 AI 题目失败: %w", err))
	}
	s.logger.Info("ai_generated_practice_done", "session_id", req.SessionID, "count", len(response.Questions))
	return nil
}

func capGeneratedQuestions(questions []generatedQuestion, expected int) []generatedQuestion {
	if len(questions) > expected {
		return questions[:expected]
	}
	return questions
}

func randomSeed() (string, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return "", fmt.Errorf("生成随机种子失败: %w", err)
	}
	return hex.EncodeToString(seed[:]), nil
}

func validateGeneratedQuestions(questions []generatedQuestion, expected int, difficulty, questionType string, points []learning.AIGenerationKnowledgePoint) error {
	if len(questions) != expected {
		return fmt.Errorf("AI 出题数量不正确：需要 %d 道，实际 %d 道", expected, len(questions))
	}
	allowed := make(map[string]bool, len(points))
	for _, point := range points {
		allowed[point.ID] = true
	}
	seenStems := map[string]bool{}
	for i, question := range questions {
		if !questionTypeMatches(questionType, question.Type) || len([]rune(strings.TrimSpace(question.Stem))) < 2 {
			return fmt.Errorf("AI 第 %d 题题型或题干不合法", i+1)
		}
		stem := strings.TrimSpace(question.Stem)
		if seenStems[stem] {
			return fmt.Errorf("AI 第 %d 题与其他题目重复", i+1)
		}
		seenStems[stem] = true
		options := make([]content.Option, 0, len(question.Options))
		if content.IsChoiceType(question.Type) {
			if !choiceStemHasBlank(stem) {
				return fmt.Errorf("AI 第 %d 题选择题题干必须包含空栏（＿＿＿）", i+1)
			}
			if len(question.Options) != 4 {
				return fmt.Errorf("AI 第 %d 题必须有 4 个选项", i+1)
			}
			seenOptions := map[string]bool{}
			for _, option := range question.Options {
				if option.ID == "" || seenOptions[option.ID] || strings.TrimSpace(option.Text) == "" {
					return fmt.Errorf("AI 第 %d 题选项不合法", i+1)
				}
				seenOptions[option.ID] = true
				options = append(options, content.Option{ID: option.ID, Label: option.Label, Text: option.Text})
			}
		} else if len(question.Options) != 0 {
			return fmt.Errorf("AI 第 %d 题非选择题不能有选项", i+1)
		}
		if err := content.ValidateAnswerValue(question.Type, options, question.CorrectAnswer); err != nil {
			return fmt.Errorf("AI 第 %d 题答案不合法: %w", i+1, err)
		}
		if question.Type == "short_answer" {
			var answer struct {
				Reference string `json:"reference"`
			}
			if err := json.Unmarshal(question.CorrectAnswer, &answer); err != nil || strings.TrimSpace(answer.Reference) == "" {
				return fmt.Errorf("AI 第 %d 题简答参考答案不合法", i+1)
			}
		}
		if !difficultyMatches(difficulty, question.Difficulty) || strings.TrimSpace(question.Explanation) == "" || len([]rune(question.Explanation)) > 2000 {
			return fmt.Errorf("AI 第 %d 题难度或解析不合法", i+1)
		}
		for _, pointID := range question.KnowledgePointIDs {
			if !allowed[pointID] {
				return fmt.Errorf("AI 第 %d 题引用了未审核知识点", i+1)
			}
		}
	}
	return nil
}

func choiceStemHasBlank(stem string) bool {
	underscoreRun := 0
	for _, r := range []rune(stem) {
		if r == '_' || r == '＿' {
			underscoreRun++
			if underscoreRun >= 2 {
				return true
			}
			continue
		}
		underscoreRun = 0
	}
	runes := []rune(stem)
	for i, r := range runes {
		if r != '（' && r != '(' {
			continue
		}
		closing := '）'
		if r == '(' {
			closing = ')'
		}
		j := i + 1
		for j < len(runes) && (runes[j] == ' ' || runes[j] == '　' || runes[j] == '\t') {
			j++
		}
		if j < len(runes) && runes[j] == closing {
			return true
		}
	}
	return false
}

func questionTypeMatches(mode, questionType string) bool {
	return content.ValidType(questionType) && (mode == generatedQuestionTypeMixed || mode == questionType)
}

func difficultyMatches(mode string, difficulty int) bool {
	switch mode {
	case generatedDifficultyEasy:
		return difficulty >= 1 && difficulty <= 2
	case generatedDifficultyNormal:
		return difficulty == 3
	case generatedDifficultyHard:
		return difficulty >= 4 && difficulty <= 5
	case generatedDifficultyMixed:
		return difficulty >= 1 && difficulty <= 5
	default:
		return false
	}
}

func shuffleGeneratedChoiceOptions(questions []generatedQuestion) error {
	for i := range questions {
		if !content.IsChoiceType(questions[i].Type) {
			continue
		}
		order := make([]int, len(questions[i].Options))
		for j := range order {
			order[j] = j
		}
		for j := len(order) - 1; j > 0; j-- {
			randomIndex, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(j+1)))
			if err != nil {
				return err
			}
			k := int(randomIndex.Int64())
			order[j], order[k] = order[k], order[j]
		}
		if err := remapGeneratedChoiceOptions(&questions[i], order); err != nil {
			return fmt.Errorf("第 %d 题: %w", i+1, err)
		}
	}
	return nil
}

func remapGeneratedChoiceOptions(question *generatedQuestion, order []int) error {
	if len(order) != len(question.Options) {
		return errors.New("选项随机顺序长度不一致")
	}
	shuffled := make([]generatedOption, len(order))
	remap := make(map[string]string, len(order))
	seen := make(map[int]bool, len(order))
	for target, source := range order {
		if source < 0 || source >= len(question.Options) || seen[source] {
			return errors.New("选项随机顺序不合法")
		}
		seen[source] = true
		shuffled[target] = question.Options[target]
		shuffled[target].Text = question.Options[source].Text
		remap[question.Options[source].ID] = question.Options[target].ID
	}
	var answer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if err := json.Unmarshal(question.CorrectAnswer, &answer); err != nil {
		return fmt.Errorf("正确答案格式不合法: %w", err)
	}
	for i, id := range answer.OptionIDs {
		mapped, ok := remap[id]
		if !ok {
			return fmt.Errorf("正确答案引用了未知选项 %q", id)
		}
		answer.OptionIDs[i] = mapped
	}
	updatedAnswer, err := json.Marshal(answer)
	if err != nil {
		return fmt.Errorf("重写正确答案失败: %w", err)
	}
	question.Options = shuffled
	question.CorrectAnswer = updatedAnswer
	return nil
}

func (s *Service) persistGeneratedQuestions(ctx context.Context, sessionID, userID, levelID, subjectID, generationMode, promptVersion string, points []learning.AIGenerationKnowledgePoint, questions []generatedQuestion) error {
	return store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		pointSubjects := make(map[string]string, len(points))
		allowedSubjects := make(map[string]bool, len(points))
		for _, point := range points {
			pointSubjects[point.ID] = point.SubjectID
			allowedSubjects[point.SubjectID] = true
		}
		var sourceID, sectionID string
		if err := tx.QueryRow(ctx,
			`INSERT INTO sources (name, kind, author, internal_note, created_by)
			 VALUES ('AI 个性化练习', 'ai_generated', 'AI', '账号私有生成题目，未经人工审核，不进入普通题库。', $1)
			 RETURNING id::text`, userID).Scan(&sourceID); err != nil {
			return err
		}
		sectionName := "根据全局记忆生成"
		if generationMode == generationModeLevel {
			sectionName = "根据当前级别生成"
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO source_sections (source_id, name, sort_order) VALUES ($1, $2, 1) RETURNING id::text`, sourceID, sectionName).Scan(&sectionID); err != nil {
			return err
		}
		for i, question := range questions {
			optionsJSON, err := json.Marshal(question.Options)
			if err != nil {
				return err
			}
			var questionID, versionID string
			if err := tx.QueryRow(ctx,
				`INSERT INTO questions (status, has_answer, created_by)
				 VALUES ('draft', false, $1) RETURNING id::text`, userID).Scan(&questionID); err != nil {
				return err
			}
			questionSubjectID := subjectID
			if questionSubjectID == "" {
				for _, pointID := range question.KnowledgePointIDs {
					if pointSubject := pointSubjects[pointID]; pointSubject != "" {
						questionSubjectID = pointSubject
						break
					}
				}
			}
			if questionSubjectID == "" {
				questionSubjectID = strings.TrimSpace(question.SubjectID)
				if !allowedSubjects[questionSubjectID] {
					return errors.New("AI 题目无法确定合法科目")
				}
			}
			if err := tx.QueryRow(ctx,
				`INSERT INTO question_versions
				 (question_id, version_no, type, stem, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by)
				 VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
				 RETURNING id::text`, questionID, question.Type, strings.TrimSpace(question.Stem), optionsJSON,
				levelID, questionSubjectID, sectionID, question.Difficulty, i+1, userID).Scan(&versionID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE questions SET current_version_id = $2, updated_at = now() WHERE id = $1`, questionID, versionID); err != nil {
				return err
			}
			for _, pointID := range question.KnowledgePointIDs {
				if _, err := tx.Exec(ctx,
					`INSERT INTO question_version_knowledge_points (question_version_id, knowledge_point_id) VALUES ($1, $2)`, versionID, pointID); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO ai_generated_question_answers (question_version_id, value, explanation, prompt_version, model)
				 VALUES ($1, $2, $3, $4, $5)`, versionID, question.CorrectAnswer, strings.TrimSpace(question.Explanation), promptVersion, s.client.cfg.Model); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO practice_items (session_id, question_id, question_version_id, position) VALUES ($1, $2, $3, $4)`,
				sessionID, questionID, versionID, i+1); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx,
			`UPDATE practice_sessions SET status = 'active', updated_at = now() WHERE id = $1 AND status = 'generating'`, sessionID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO audit_logs (actor_user_id, action, object_type, object_id, detail)
			 VALUES ($1, 'ai_practice_generated', 'practice_session', $2, jsonb_build_object('count', $3::int))`, userID, sessionID, len(questions))
		return err
	})
}

func (s *Service) generationRetry(ctx context.Context, sessionID string, attempts, maxAttempts int, cause error) error {
	if attempts < maxAttempts {
		return cause
	}
	if err := s.markGenerationFailed(ctx, sessionID, cause); err != nil {
		return fmt.Errorf("标记 AI 出题失败失败: %v（原错误：%w）", err, cause)
	}
	return cause
}

func (s *Service) markGenerationFailed(ctx context.Context, sessionID string, cause error) error {
	message := "AI 出题失败，请重新开始。"
	if cause != nil {
		message += shortError(cause)
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE practice_sessions
		 SET status = 'generation_failed', ai_summary_status = 'failed', ai_summary = $2, updated_at = now()
		 WHERE id = $1 AND status = 'generating'`, sessionID, message)
	return err
}
