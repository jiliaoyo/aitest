package ai

import (
	"bytes"
	"context"
	"crypto/md5"
	cryptorand "crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

const questionGenerationPromptVersion = "practice_question_generation.v12"
const questionGenerationRetryPromptVersion = "practice_question_generation.v12.retry"

const questionGenerationRetryInstructions = `上一轮输出没有通过服务端结构校验。本轮必须重新生成完整的一组题目，不能只返回修改后的题目；请优先修正下面的服务端错误，并再次逐题检查题量、题型、答案结构和解析。`

//go:embed prompts/practice_question_generation.v12.md
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

const maxRecentGeneratedStemsInPrompt = 20
const maxGenerationCalls = 6

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
	AvoidStems     []string                    `json:"avoidStems,omitempty"`
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

type generatedStemRow struct {
	Stem string
}

type generatedQuestionHistoryRow struct {
	LevelID    string
	SubjectID  string
	Type       string
	Stem       string
	Options    *string
	Answer     *string
	Difficulty int
}

func (s *Service) loadGeneratedStems(ctx context.Context, db store.DBTx, userID, levelID, subjectID string, limit int) ([]string, error) {
	rows, err := store.CollectRows[generatedStemRow](ctx, db,
		`SELECT v.stem
		 FROM practice_items pi
		 JOIN practice_sessions ps ON ps.id = pi.session_id
		 JOIN question_versions v ON v.id = pi.question_version_id
		 JOIN source_sections ss ON ss.id = v.source_section_id
		 JOIN sources src ON src.id = ss.source_id
		 WHERE src.kind = 'ai_generated' AND ps.user_id = $1
		   AND v.level_id::text = $2
		   AND ($3 = '' OR v.subject_id::text = $3)
		 ORDER BY pi.created_at DESC, pi.id DESC
		 LIMIT CASE WHEN $4 > 0 THEN $4 ELSE NULL END`, userID, levelID, subjectID, limit)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(rows))
	stems := make([]string, 0, len(rows))
	for _, row := range rows {
		stem := strings.TrimSpace(row.Stem)
		key := normalizeGeneratedStem(stem)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		stems = append(stems, stem)
	}
	return stems, nil
}

func (s *Service) loadGeneratedQuestionKeys(ctx context.Context, db store.DBTx, userID, levelID, subjectID string) ([]string, error) {
	rows, err := store.CollectRows[generatedQuestionHistoryRow](ctx, db,
		`SELECT v.level_id::text, v.subject_id::text, v.type, v.stem, v.options::text, aga.value::text, v.difficulty
		 FROM practice_items pi
		 JOIN practice_sessions ps ON ps.id = pi.session_id
		 JOIN question_versions v ON v.id = pi.question_version_id
		 JOIN source_sections ss ON ss.id = v.source_section_id
		 JOIN sources src ON src.id = ss.source_id
		 LEFT JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		 WHERE src.kind = 'ai_generated' AND ps.user_id = $1
		   AND v.level_id::text = $2
		   AND ($3 = '' OR v.subject_id::text = $3)`, userID, levelID, subjectID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		question, err := generatedQuestionFromHistoryRow(row)
		if err != nil {
			return nil, err
		}
		keys = append(keys, generatedQuestionReuseKey(row.LevelID, row.SubjectID, question))
	}
	return uniqueGeneratedKeys(keys), nil
}

func generatedQuestionFromHistoryRow(row generatedQuestionHistoryRow) (generatedQuestion, error) {
	question := generatedQuestion{Type: row.Type, Stem: row.Stem, Difficulty: row.Difficulty}
	if row.Options != nil && strings.TrimSpace(*row.Options) != "" {
		if err := json.Unmarshal([]byte(*row.Options), &question.Options); err != nil {
			return generatedQuestion{}, fmt.Errorf("解析历史 AI 题目选项失败: %w", err)
		}
	}
	if row.Answer != nil {
		question.CorrectAnswer = json.RawMessage(*row.Answer)
	}
	return question, nil
}

func uniqueGeneratedKeys(keys []string) []string {
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
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
	avoidStems, err := s.loadGeneratedStems(ctx, s.pool, row.UserID, row.LevelID, subjectID, maxRecentGeneratedStemsInPrompt)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("读取历史 AI 题干失败: %w", err))
	}
	// 全量指纹只用于服务端精确去重，不放进提示词，避免历史增长后消耗大量 token。
	existingKeys, err := s.loadGeneratedQuestionKeys(ctx, s.pool, row.UserID, row.LevelID, subjectID)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("读取全部历史 AI 题目失败: %w", err))
	}
	generatedQuestions := make([]generatedQuestion, 0, row.RequestedCount)
	generatedQuestionPoints := memory.KnowledgePoints
	lastPromptVersion := questionGenerationPromptVersion
	var validationErr error
	retryNote := ""
	for generationAttempt := 0; generationAttempt < maxGenerationCalls && len(generatedQuestions) < row.RequestedCount; generationAttempt++ {
		remaining := row.RequestedCount - len(generatedQuestions)
		seed, err := randomSeed()
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		systemPrompt := questionGenerationPrompt
		feedback := ""
		temperature := 0.6
		promptVersion := questionGenerationPromptVersion
		if generationAttempt > 0 {
			promptVersion = questionGenerationRetryPromptVersion
			if validationErr != nil {
				feedback = shortError(validationErr)
				systemPrompt += "\n\n" + questionGenerationRetryInstructions + "\n服务端校验错误：" + feedback
				temperature = 0.2
			} else if retryNote != "" {
				feedback = retryNote
				systemPrompt += "\n\n" + feedback
				temperature = 0.8
			}
		}
		inputJSON, _ := json.Marshal(questionGenerationInput{
			Count: remaining, LevelID: row.LevelID, LevelCode: row.LevelCode, SubjectID: subjectID, Difficulty: difficulty,
			GenerationMode: generationMode, QuestionType: questionType, ShowFurigana: scope.ShowFurigana, Category: category,
			RandomSeed: seed, RetryFeedback: feedback, AvoidStems: avoidStems, LearningMemory: memory,
		})
		out, err := s.client.RunPromptWithTemperature(ctx, row.UserID, "practice_question_generation", promptVersion,
			req.SessionID, systemPrompt, string(inputJSON), temperature)
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		var response generatedQuestionResponse
		if err := strictDecode(out, &response); err != nil {
			validationErr = fmt.Errorf("AI 出题输出不合法: %w", err)
			retryNote = ""
			continue
		}
		questions := capGeneratedQuestions(response.Questions, remaining)
		if err := validateGeneratedQuestions(questions, remaining, difficulty, questionType, memory.KnowledgePoints); err != nil {
			validationErr = err
			retryNote = ""
			continue
		}
		blockedKeys := append([]string{}, existingKeys...)
		generatedKeys, err := generatedQuestionKeys(row.LevelID, subjectID, generatedQuestions, generatedQuestionPoints)
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		blockedKeys = append(blockedKeys, generatedKeys...)
		uniqueQuestions, duplicates, err := filterGeneratedQuestionDuplicates(questions, row.LevelID, subjectID, generatedQuestionPoints, blockedKeys)
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		if len(duplicates) > 0 {
			// 只把本轮实际命中的旧题干加入重试上下文，避免把全部历史题干发给模型。
			avoidStems = appendUniqueGeneratedStems(avoidStems, duplicates)
			retryNote = fmt.Sprintf("上一轮返回的 %d 道题中有 %d 道与历史题目重复，已剔除；本轮只需补充剩余题目，并更换句式、场景和词汇。", len(questions), len(duplicates))
		} else {
			retryNote = ""
		}
		generatedQuestions = append(generatedQuestions, uniqueQuestions...)
		avoidStems = appendUniqueGeneratedStems(avoidStems, generatedQuestionStems(uniqueQuestions))
		lastPromptVersion = promptVersion
		validationErr = nil
	}
	if len(generatedQuestions) != row.RequestedCount {
		if validationErr == nil {
			validationErr = fmt.Errorf("AI 题目去重后数量不足：需要 %d 道，实际 %d 道", row.RequestedCount, len(generatedQuestions))
		}
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, validationErr)
	}
	if err := shuffleGeneratedChoiceOptions(generatedQuestions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("打乱 AI 选项失败: %w", err))
	}
	if err := s.persistGeneratedQuestions(ctx, req.SessionID, row.UserID, row.LevelID, subjectID, generationMode, lastPromptVersion, memory.KnowledgePoints, generatedQuestions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("保存 AI 题目失败: %w", err))
	}
	s.logger.Info("ai_generated_practice_done", "session_id", req.SessionID, "count", len(generatedQuestions))
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
		stemKey := normalizeGeneratedStem(stem)
		if seenStems[stemKey] {
			return fmt.Errorf("AI 第 %d 题与其他题目重复", i+1)
		}
		seenStems[stemKey] = true
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

func normalizeGeneratedStem(stem string) string {
	return strings.Join(strings.Fields(stem), "")
}

func generatedQuestionReuseKey(levelID, subjectID string, question generatedQuestion) string {
	options := append([]generatedOption(nil), question.Options...)
	sort.SliceStable(options, func(i, j int) bool {
		return options[i].ID < options[j].ID
	})
	canonical := struct {
		LevelID   string            `json:"levelId"`
		SubjectID string            `json:"subjectId"`
		Type      string            `json:"type"`
		Stem      string            `json:"stem"`
		Options   []generatedOption `json:"options"`
		Answer    json.RawMessage   `json:"answer"`
	}{
		LevelID: levelID, SubjectID: subjectID, Type: question.Type,
		Stem:    normalizeGeneratedStem(question.Stem),
		Options: options, Answer: canonicalJSON(question.CorrectAnswer),
	}
	data, _ := json.Marshal(canonical)
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

func canonicalJSON(raw json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return bytes.TrimSpace(raw)
	}
	data, err := json.Marshal(value)
	if err != nil {
		return bytes.TrimSpace(raw)
	}
	return data
}

func resolveGeneratedQuestionSubject(defaultSubject string, question generatedQuestion, points []learning.AIGenerationKnowledgePoint) (string, error) {
	if defaultSubject != "" {
		return defaultSubject, nil
	}
	pointSubjects := make(map[string]string, len(points))
	allowedSubjects := make(map[string]struct{}, len(points))
	for _, point := range points {
		pointSubjects[point.ID] = point.SubjectID
		allowedSubjects[point.SubjectID] = struct{}{}
	}
	for _, pointID := range question.KnowledgePointIDs {
		if subjectID := pointSubjects[pointID]; subjectID != "" {
			return subjectID, nil
		}
	}
	if subjectID := strings.TrimSpace(question.SubjectID); subjectID != "" {
		if _, ok := allowedSubjects[subjectID]; ok {
			return subjectID, nil
		}
	}
	return "", errors.New("AI 题目无法确定合法科目")
}

func generatedQuestionKeys(levelID, subjectID string, questions []generatedQuestion, points []learning.AIGenerationKnowledgePoint) ([]string, error) {
	keys := make([]string, 0, len(questions))
	for _, question := range questions {
		questionSubjectID, err := resolveGeneratedQuestionSubject(subjectID, question, points)
		if err != nil {
			return nil, err
		}
		keys = append(keys, generatedQuestionReuseKey(levelID, questionSubjectID, question))
	}
	return keys, nil
}

func filterGeneratedQuestionDuplicates(questions []generatedQuestion, levelID, subjectID string, points []learning.AIGenerationKnowledgePoint, existingKeys []string) ([]generatedQuestion, []string, error) {
	keys := make(map[string]struct{}, len(existingKeys))
	for _, key := range existingKeys {
		keys[key] = struct{}{}
	}
	filtered := make([]generatedQuestion, 0, len(questions))
	duplicates := make([]string, 0)
	seenDuplicates := make(map[string]struct{})
	for _, question := range questions {
		questionSubjectID, err := resolveGeneratedQuestionSubject(subjectID, question, points)
		if err != nil {
			return nil, nil, err
		}
		key := generatedQuestionReuseKey(levelID, questionSubjectID, question)
		if _, ok := keys[key]; ok {
			if _, seen := seenDuplicates[key]; !seen {
				duplicates = append(duplicates, strings.TrimSpace(question.Stem))
				seenDuplicates[key] = struct{}{}
			}
			continue
		}
		filtered = append(filtered, question)
		keys[key] = struct{}{}
	}
	return filtered, duplicates, nil
}

func generatedQuestionStems(questions []generatedQuestion) []string {
	stems := make([]string, 0, len(questions))
	for _, question := range questions {
		stems = append(stems, question.Stem)
	}
	return stems
}

func appendUniqueGeneratedStems(stems, additions []string) []string {
	seen := make(map[string]struct{}, len(stems)+len(additions))
	out := make([]string, 0, len(stems)+len(additions))
	for _, stem := range append(stems, additions...) {
		stem = strings.TrimSpace(stem)
		key := normalizeGeneratedStem(stem)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, stem)
	}
	return out
}

type generatedReuseCandidateRow struct {
	QuestionID string
	VersionID  string
	Type       string
	Stem       string
	Options    *string
	Answer     *string
	Difficulty int
}

func (s *Service) findOrAdoptGeneratedQuestion(ctx context.Context, tx pgx.Tx, key, levelID, subjectID string, question generatedQuestion) (questionID, versionID string, reused bool, err error) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
		return "", "", false, fmt.Errorf("锁定 AI 题目复用键失败: %w", err)
	}
	const keyedQuery = `SELECT q.id::text, v.id::text
		FROM question_versions v
		JOIN questions q ON q.id = v.question_id
		JOIN source_sections ss ON ss.id = v.source_section_id
		JOIN sources src ON src.id = ss.source_id
		WHERE src.kind = 'ai_generated' AND v.ai_reuse_key = $1
		LIMIT 1`
	if err := tx.QueryRow(ctx, keyedQuery, key).Scan(&questionID, &versionID); err == nil {
		return questionID, versionID, true, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, err
	}

	candidates, err := store.CollectRows[generatedReuseCandidateRow](ctx, tx,
		`SELECT q.id::text, v.id::text, v.type, v.stem, v.options::text, aga.value::text, v.difficulty
		 FROM question_versions v
		 JOIN questions q ON q.id = v.question_id
		 JOIN source_sections ss ON ss.id = v.source_section_id
		 JOIN sources src ON src.id = ss.source_id
		 LEFT JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		 WHERE src.kind = 'ai_generated' AND v.ai_reuse_key IS NULL
		   AND v.level_id::text = $1 AND v.subject_id::text = $2 AND v.type = $3`, levelID, subjectID, question.Type)
	if err != nil {
		return "", "", false, fmt.Errorf("查找历史 AI 题目失败: %w", err)
	}
	for _, candidate := range candidates {
		history, err := generatedQuestionFromHistoryRow(generatedQuestionHistoryRow{
			LevelID: levelID, SubjectID: subjectID, Type: candidate.Type, Stem: candidate.Stem,
			Options: candidate.Options, Answer: candidate.Answer, Difficulty: candidate.Difficulty,
		})
		if err != nil {
			return "", "", false, err
		}
		if generatedQuestionReuseKey(levelID, subjectID, history) != key {
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE question_versions SET ai_reuse_key = $1 WHERE id = $2 AND ai_reuse_key IS NULL`, key, candidate.VersionID); err != nil {
			return "", "", false, fmt.Errorf("登记历史 AI 题目复用键失败: %w", err)
		}
		return candidate.QuestionID, candidate.VersionID, true, nil
	}
	return "", "", false, nil
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
		existingKeys, err := s.loadGeneratedQuestionKeys(ctx, tx, userID, levelID, subjectID)
		if err != nil {
			return fmt.Errorf("检查历史 AI 题目失败: %w", err)
		}
		if _, duplicates, err := filterGeneratedQuestionDuplicates(questions, levelID, subjectID, points, existingKeys); err != nil {
			return err
		} else if len(duplicates) > 0 {
			return fmt.Errorf("AI 题目与历史完全重复：%s", strings.Join(duplicates, "；"))
		}
		sectionName := "根据全局记忆生成"
		if generationMode == generationModeLevel {
			sectionName = "根据当前级别生成"
		}
		var sourceID, sectionID string
		ensureSourceSection := func() error {
			if sectionID != "" {
				return nil
			}
			if err := tx.QueryRow(ctx,
				`INSERT INTO sources (name, kind, author, internal_note, created_by)
				 VALUES ('AI 个性化练习', 'ai_generated', 'AI', '账号私有生成题目，未经人工审核，不进入普通题库。', $1)
				 RETURNING id::text`, userID).Scan(&sourceID); err != nil {
				return err
			}
			return tx.QueryRow(ctx,
				`INSERT INTO source_sections (source_id, name, sort_order) VALUES ($1, $2, 1) RETURNING id::text`, sourceID, sectionName).Scan(&sectionID)
		}
		for i, question := range questions {
			optionsJSON, err := json.Marshal(question.Options)
			if err != nil {
				return err
			}
			questionSubjectID, err := resolveGeneratedQuestionSubject(subjectID, question, points)
			if err != nil {
				return err
			}
			key := generatedQuestionReuseKey(levelID, questionSubjectID, question)
			questionID, versionID, reused, err := s.findOrAdoptGeneratedQuestion(ctx, tx, key, levelID, questionSubjectID, question)
			if err != nil {
				return err
			}
			if !reused {
				if err := ensureSourceSection(); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO questions (status, has_answer, created_by)
					 VALUES ('draft', false, $1) RETURNING id::text`, userID).Scan(&questionID); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO question_versions
					 (question_id, version_no, type, stem, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by, ai_reuse_key)
					 VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
					 RETURNING id::text`, questionID, question.Type, strings.TrimSpace(question.Stem), optionsJSON,
					levelID, questionSubjectID, sectionID, question.Difficulty, i+1, userID, key).Scan(&versionID); err != nil {
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
		_, err = tx.Exec(ctx,
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
