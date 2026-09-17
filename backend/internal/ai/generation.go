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
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/learning"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

const questionGenerationPromptVersion = "practice_question_generation.v20"
const questionGenerationRetryPromptVersion = "practice_question_generation.v20.retry"

const questionGenerationRetryInstructions = `上一轮部分或全部候选题没有通过服务端逐题校验。本轮只生成输入 JSON 中 count 指定的剩余题目；请优先修正下面的服务端错误，并再次逐题检查题型、答案结构和解析。`

const questionGenerationPromptAddendum = `

服务端附加硬性约束：
1. 输入 JSON 的 curriculumScope 是当前 JLPT 级别的边界说明，优先级高于常见教材记忆；不要把更高级别的句型带入低级别。
2. questionType=mixed 时，输入 JSON 的 questionTypePlan 是本次响应必须严格满足的题型数量。四种题型都要按计划出现，不能用多道 single_choice 代替其他题型。
3. category=grammar_modality 表示愿望、计划与基础推量；N5 不生成意志形（～（よ）う）或 ～ために。具体级别边界仍以 curriculumScope 为准。
4. 题目中的干扰项也必须属于当前级别和科目；不能用高等级句型充当错误选项。
5. short_answer 的 correctAnswer 必须严格是 {"reference":"非空字符串"}；fill_blank 必须严格是 {"acceptable":["非空字符串"]}，不要使用 text、null、数组对象或其他结构。
6. 当 subjectCode=reading，或 category 以 reading_ 开头时，生成 2～6 篇彼此独立的公共阅读材料；20 道题优先生成 4 篇、每篇约 5 道小题，10 道题约 2 篇，30 道题约 6 篇。材料对应题量不要求完全均匀，但每篇至少对应 2 道题，任何一篇不要承载超过整批约 60% 的题目。同一篇材料的所有题必须逐字复用相同的 material.title 和 material.content；不要让整批题目只共用一篇材料，也不要每道题单独生成一篇材料。每道题仍必须输出 material，题干必须围绕对应材料。阅读选择题是材料理解题，可以使用完整疑问句和选项回答，不要强行插入语法填空空栏。
`

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
const maxRecentGeneratedStemsForSimilarity = 200
const maxGenerationCalls = 6
const generatedStemSimilarityThreshold = 0.62

var errGenerationBudgetExceeded = errors.New("AI 生成批次已达到模型调用上限")

var generatedCategories = map[string]struct{}{
	generatedCategoryMixed:  {},
	"grammar_case_particle": {}, "grammar_conjunctive_particle": {}, "grammar_adverbial_particle": {}, "grammar_final_particle": {},
	"grammar_auxiliary": {}, "grammar_verb": {}, "grammar_adjective": {}, "grammar_adverb": {}, "grammar_conjunction": {},
	"grammar_adnominal": {}, "grammar_sentence_pattern": {}, "grammar_tense_aspect": {}, "grammar_condition": {},
	"grammar_voice": {}, "grammar_benefactive": {}, "grammar_honorific": {}, "grammar_negation": {}, "grammar_modality": {},
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
	req.KnowledgePointIDs = normalizeKnowledgePointIDs(req.KnowledgePointIDs)
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
		// 用户行锁把“已有生成批次”和“今日配额”检查与创建串成一个原子边界。
		var lockedUserID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id::text = $1 FOR UPDATE`, userID).Scan(&lockedUserID); err != nil {
			return err
		}
		var existingID, existingScope string
		var sameScope bool
		err := tx.QueryRow(ctx,
			`SELECT id::text, scope::text, scope = $2::jsonb
			 FROM practice_sessions
			 WHERE user_id = $1 AND status = 'generating'
			 ORDER BY created_at DESC, id DESC
			 LIMIT 1 FOR UPDATE`, userID, string(scope),
		).Scan(&existingID, &existingScope, &sameScope)
		if err == nil {
			if sameScope {
				out = AIGeneratedSession{ID: existingID, Status: "generating"}
				return nil
			}
			return httpapi.E(http.StatusConflict, "ai_generation_in_progress", "已有一批 AI 题目正在生成，请等待完成后再开始。")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var used int
		var resetAt time.Time
		if err := tx.QueryRow(ctx,
			`SELECT count(*)::int,
			        (date_trunc('day', now() AT TIME ZONE 'UTC') + INTERVAL '1 day') AT TIME ZONE 'UTC'
			 FROM practice_sessions
			 WHERE user_id = $1 AND scope->>'mode' = 'ai_generated'
			   AND created_at >= (date_trunc('day', now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')`, userID,
		).Scan(&used, &resetAt); err != nil {
			return err
		}
		if s.generationDailyLimit > 0 && used >= s.generationDailyLimit {
			resetText := resetAt.UTC().Format(time.RFC3339)
			return httpapi.WithDetails(
				httpapi.E(http.StatusTooManyRequests, "ai_generation_daily_limit", fmt.Sprintf("今日 AI 出题次数已用完，配额将在 %s（UTC）重置。", resetText)),
				map[string]any{"dailyLimit": s.generationDailyLimit, "used": used, "resetAt": resetText},
			)
		}
		var subjectID any
		if req.SubjectID != "" {
			subjectID = req.SubjectID
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO practice_sessions (user_id, status, level_id, subject_id, scope, requested_count, ai_generation_call_budget)
			 VALUES ($1, 'generating', $2, $3, $4, $5, $6) RETURNING id::text`,
			userID, req.LevelID, subjectID, scope, req.Count, s.generationCallBudget).Scan(&out.ID); err != nil {
			return err
		}
		out.Status = "generating"
		return jobs.EnqueueTx(ctx, tx, "generate_ai_practice_session", map[string]string{"sessionId": out.ID})
	})
	return out, err
}

func validGeneratedCount(count int) bool { return count == 10 || count == 20 || count == 30 }

func normalizeKnowledgePointIDs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

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

func isReadingGeneration(subjectCode, category string) bool {
	return subjectCode == "reading" || strings.HasPrefix(category, "reading_")
}

var generatedQuestionTypes = []string{"single_choice", "multiple_choice", "fill_blank", "short_answer"}

// mixedQuestionTypePlan 按固定顺序分配余数，保证 10/20/30 题都覆盖四种题型。
func mixedQuestionTypePlan(count int) map[string]int {
	plan := make(map[string]int, len(generatedQuestionTypes))
	for i := 0; i < count; i++ {
		plan[generatedQuestionTypes[i%len(generatedQuestionTypes)]]++
	}
	return plan
}

func questionTypePlanFor(total int, existing []generatedQuestion) map[string]int {
	plan := mixedQuestionTypePlan(total)
	for _, question := range existing {
		if plan[question.Type] > 0 {
			plan[question.Type]--
		}
	}
	return plan
}

func validateGeneratedQuestionTypePlan(questions []generatedQuestion, mode string, plan map[string]int) error {
	if mode != generatedQuestionTypeMixed {
		return nil
	}
	counts := make(map[string]int, len(plan))
	for _, question := range questions {
		counts[question.Type]++
	}
	for questionType, expected := range plan {
		if counts[questionType] != expected {
			return fmt.Errorf("AI 混合题型分布不合法：%s 需要 %d 道，实际 %d 道", questionType, expected, counts[questionType])
		}
	}
	return nil
}

var generatedCategoryMinimumLevel = map[string]int{
	"grammar_condition":    4,
	"grammar_voice":        3,
	"grammar_honorific":    4,
	"vocabulary_polysemy":  3,
	"vocabulary_compound":  3,
	"vocabulary_affix":     2,
	"vocabulary_honorific": 3,
	"reading_reference":    4,
	"reading_paraphrase":   4,
	"reading_logic":        4,
	"reading_inference":    3,
	"reading_author":       3,
	"reading_structure":    3,
	"reading_style":        2,
}

func generatedCategorySet(categories ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(categories))
	for _, category := range categories {
		set[category] = struct{}{}
	}
	return set
}

// 级别白名单与前端分类列表保持一致；分类是出题范围，不是模型自由发挥的标签。
var generatedCategoryAllowlist = map[string]map[string]struct{}{
	"n5": generatedCategorySet(
		"mixed", "grammar_sentence_pattern", "grammar_case_particle", "grammar_verb", "grammar_adjective", "grammar_tense_aspect", "grammar_auxiliary", "grammar_modality", "grammar_conjunctive_particle", "grammar_benefactive", "grammar_negation",
		"vocabulary_kanji", "vocabulary_noun", "vocabulary_verb", "vocabulary_adjective", "vocabulary_adverb", "vocabulary_pronoun", "vocabulary_counter", "vocabulary_time_number", "vocabulary_collocation", "vocabulary_katakana", "vocabulary_usage",
		"reading_information", "reading_main_idea", "reading_vocabulary", "reading_chart_notice",
	),
	"n4": generatedCategorySet(
		"mixed", "grammar_sentence_pattern", "grammar_case_particle", "grammar_adverbial_particle", "grammar_final_particle", "grammar_auxiliary", "grammar_verb", "grammar_adjective", "grammar_adverb", "grammar_conjunctive_particle", "grammar_conjunction", "grammar_adnominal", "grammar_tense_aspect", "grammar_modality", "grammar_condition", "grammar_benefactive", "grammar_honorific", "grammar_negation",
		"vocabulary_kanji", "vocabulary_noun", "vocabulary_verb", "vocabulary_adjective", "vocabulary_adverb", "vocabulary_conjunction", "vocabulary_pronoun", "vocabulary_counter", "vocabulary_time_number", "vocabulary_synonym", "vocabulary_collocation", "vocabulary_compound", "vocabulary_onoma", "vocabulary_katakana", "vocabulary_usage",
		"reading_information", "reading_main_idea", "reading_reference", "reading_paraphrase", "reading_logic", "reading_vocabulary", "reading_chart_notice",
	),
	"n3": generatedCategorySet(
		"mixed", "grammar_case_particle", "grammar_conjunctive_particle", "grammar_adverbial_particle", "grammar_final_particle", "grammar_auxiliary", "grammar_verb", "grammar_adjective", "grammar_adverb", "grammar_conjunction", "grammar_adnominal", "grammar_sentence_pattern", "grammar_tense_aspect", "grammar_modality", "grammar_condition", "grammar_voice", "grammar_benefactive", "grammar_honorific", "grammar_negation",
		"vocabulary_kanji", "vocabulary_noun", "vocabulary_verb", "vocabulary_adjective", "vocabulary_adverb", "vocabulary_conjunction", "vocabulary_pronoun", "vocabulary_counter", "vocabulary_time_number", "vocabulary_synonym", "vocabulary_polysemy", "vocabulary_collocation", "vocabulary_compound", "vocabulary_onoma", "vocabulary_katakana", "vocabulary_honorific", "vocabulary_usage",
		"reading_information", "reading_main_idea", "reading_reference", "reading_paraphrase", "reading_logic", "reading_inference", "reading_author", "reading_vocabulary", "reading_structure", "reading_chart_notice",
	),
	"n2": generatedCategorySet(
		"mixed", "grammar_sentence_pattern", "grammar_case_particle", "grammar_conjunctive_particle", "grammar_adverbial_particle", "grammar_auxiliary", "grammar_verb", "grammar_adjective", "grammar_adverb", "grammar_conjunction", "grammar_tense_aspect", "grammar_modality", "grammar_condition", "grammar_voice", "grammar_benefactive", "grammar_honorific", "grammar_negation",
		"vocabulary_kanji", "vocabulary_noun", "vocabulary_verb", "vocabulary_adjective", "vocabulary_adverb", "vocabulary_conjunction", "vocabulary_pronoun", "vocabulary_counter", "vocabulary_time_number", "vocabulary_synonym", "vocabulary_polysemy", "vocabulary_collocation", "vocabulary_compound", "vocabulary_affix", "vocabulary_katakana", "vocabulary_usage",
		"reading_information", "reading_main_idea", "reading_reference", "reading_paraphrase", "reading_logic", "reading_inference", "reading_author", "reading_vocabulary", "reading_structure", "reading_chart_notice",
	),
	"n1": generatedCategorySet(
		"mixed", "grammar_case_particle", "grammar_conjunctive_particle", "grammar_adverbial_particle", "grammar_final_particle", "grammar_auxiliary", "grammar_verb", "grammar_adjective", "grammar_adverb", "grammar_conjunction", "grammar_adnominal", "grammar_sentence_pattern", "grammar_tense_aspect", "grammar_modality", "grammar_condition", "grammar_voice", "grammar_benefactive", "grammar_honorific", "grammar_negation",
		"vocabulary_kanji", "vocabulary_noun", "vocabulary_verb", "vocabulary_adjective", "vocabulary_adverb", "vocabulary_conjunction", "vocabulary_pronoun", "vocabulary_counter", "vocabulary_time_number", "vocabulary_synonym", "vocabulary_polysemy", "vocabulary_collocation", "vocabulary_compound", "vocabulary_affix", "vocabulary_onoma", "vocabulary_katakana", "vocabulary_honorific", "vocabulary_usage",
		"reading_information", "reading_main_idea", "reading_reference", "reading_paraphrase", "reading_logic", "reading_inference", "reading_author", "reading_vocabulary", "reading_structure", "reading_chart_notice", "reading_style",
	),
}

func validGeneratedCategoryForLevel(category, levelCode string) bool {
	if allowed, ok := generatedCategoryAllowlist[strings.ToLower(levelCode)]; ok {
		_, found := allowed[category]
		return found
	}
	minimum, ok := generatedCategoryMinimumLevel[category]
	if !ok || len(levelCode) < 2 || levelCode[0] != 'n' {
		return true
	}
	level, err := strconv.Atoi(levelCode[1:])
	return err != nil || level >= 1 && level <= 5 && level <= minimum
}

func validGeneratedCategoryForSubject(category, subjectCode string) bool {
	return category == generatedCategoryMixed || subjectCode == "" || strings.HasPrefix(category, subjectCode+"_")
}

type generationCurriculum struct {
	Scope          string
	ForbiddenForms []string
}

var generationCurricula = map[string]generationCurriculum{
	"n5": {
		Scope:          "初级基础范围：假名和基础汉字、名词/い形容词/な形容词、动词ます形与基础活用、基本助词、存在句、时间地点、比较、简单请求/许可/禁止、愿望与基础推量。不要生成意志形（～（よ）う）、可能/被动/使役、～ために、复杂条件或高级书面表达。",
		ForbiddenForms: []string{"ために", "ことができ", "ようになる", "ようにする", "ておく", "てしまう", "ばかり", "わけにはいか", "させられ"},
	},
	"n4": {
		Scope:          "初中级范围：以 N5 基础为前提，加入常见条件、原因转折、先后、经验、目的、授受、请求许可和基础敬语。不要生成 N3 以上的书面句型、复杂复合表达或使役被动。",
		ForbiddenForms: []string{"わけにはいか", "かねない", "かねる", "つつ", "ずには", "ざるを得", "ものなら", "ばかりに", "に違いない", "ことなく", "させられ"},
	},
	"n3": {
		Scope:          "中级范围：以 N4 基础为前提，覆盖复合句、间接表达、变化、状态、可能/被动/使役、条件和语气辨析；可使用常见书面表达，但不要越过 N2/N1 的高阶惯用句型。",
		ForbiddenForms: []string{"かねない", "かねる", "つつある", "ずには", "ざるを得", "ものなら", "ばかりに", "ことなく", "にわたって", "を問わず", "に違いない"},
	},
	"n2": {
		Scope:          "中高级范围：覆盖复杂从句、书面语、抽象语义、语气和正式表达；保留 N5-N3 基础，但不要把 N1 特有的古雅、极正式或固定惯用句型当作常规考点。",
		ForbiddenForms: []string{"が最後", "こととて", "かたわら", "が早いか", "そばから", "ずくめ", "たるもの", "あっての", "いかんに", "を余儀なく"},
	},
	"n1": {
		Scope: "高级范围：允许正式、书面、抽象、惯用和较少见的 JLPT 高级表达，但题目仍需自洽、可解释，并避免生造不存在的句型。",
	},
}

func generationCurriculumScope(levelCode string) string {
	if curriculum, ok := generationCurricula[strings.ToLower(levelCode)]; ok {
		return curriculum.Scope
	}
	return "以输入的 JLPT 级别和已审核知识点为唯一范围，不要自由扩展到更高或更低级别。"
}

func validateGeneratedQuestionLevel(levelCode, subjectCode string, questions []generatedQuestion) error {
	if subjectCode != "grammar" {
		return nil
	}
	curriculum, ok := generationCurricula[strings.ToLower(levelCode)]
	if !ok || len(curriculum.ForbiddenForms) == 0 {
		return nil
	}
	for i, question := range questions {
		parts := []string{question.Stem}
		for _, option := range question.Options {
			parts = append(parts, option.Text)
		}
		parts = append(parts, string(question.CorrectAnswer))
		text := strings.Join(parts, "\n")
		for _, form := range curriculum.ForbiddenForms {
			if strings.Contains(text, form) {
				return fmt.Errorf("AI 第 %d 题包含 %s，超出 %s 语法范围", i+1, form, strings.ToUpper(levelCode))
			}
		}
	}
	return nil
}

func validateGeneratedReadingQuestions(subjectCode, category string, questions []generatedQuestion) error {
	if !isReadingGeneration(subjectCode, category) {
		return nil
	}
	materialCounts := make(map[string]int)
	for i, question := range questions {
		if question.Material == nil {
			return fmt.Errorf("AI 第 %d 题缺少公共阅读材料", i+1)
		}
		content := strings.TrimSpace(question.Material.Content)
		if len([]rune(content)) < 40 || len([]rune(content)) > 5000 {
			return fmt.Errorf("AI 第 %d 题公共材料长度不合法", i+1)
		}
		title := strings.TrimSpace(question.Material.Title)
		if len([]rune(title)) > 100 {
			return fmt.Errorf("AI 第 %d 题公共材料标题过长", i+1)
		}
		key := normalizeGeneratedStem(title + "\n" + content)
		materialCounts[key]++
	}
	if len(questions) >= 10 && len(materialCounts) < 2 {
		return fmt.Errorf("AI 阅读题需要至少两篇公共材料，每篇材料对应多道题")
	}
	if len(materialCounts) > 6 {
		return fmt.Errorf("AI 阅读题公共材料数量不能超过 6 篇")
	}
	if len(questions) >= 6 {
		maxQuestionsPerMaterial := (len(questions)*3 + 4) / 5
		for _, count := range materialCounts {
			if count < 2 {
				return fmt.Errorf("AI 阅读题每篇公共材料至少应对应两道题")
			}
			if len(questions) >= 10 && count > maxQuestionsPerMaterial {
				return fmt.Errorf("AI 阅读题单篇材料题量过于集中")
			}
		}
	}
	return nil
}

func (s *Service) validateGenerationScope(ctx context.Context, req AIGenerateRequest) error {
	var levelCode string
	if err := s.pool.QueryRow(ctx, `SELECT code FROM exam_levels WHERE id::text = $1`, req.LevelID).Scan(&levelCode); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpapi.ErrNotFound
		}
		return err
	}
	if !validGeneratedCategoryForLevel(req.Category, levelCode) {
		return httpapi.ValidationError(map[string]string{"category": fmt.Sprintf("%s 不适用于 %s 级别，请选择当前级别支持的分类", req.Category, strings.ToUpper(levelCode))})
	}
	if req.SubjectID != "" {
		var subjectCode string
		if err := s.pool.QueryRow(ctx,
			`SELECT sub.code
			 FROM exam_levels l JOIN subjects sub ON sub.exam_id = l.exam_id
			 WHERE l.id::text = $1 AND sub.id::text = $2`, req.LevelID, req.SubjectID).Scan(&subjectCode); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpapi.ErrNotFound
			}
			return err
		}
		if !validGeneratedCategoryForSubject(req.Category, subjectCode) {
			return httpapi.ValidationError(map[string]string{"category": "出题分类与所选科目不一致"})
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
	Count            int                         `json:"count"`
	LevelID          string                      `json:"levelId"`
	LevelCode        string                      `json:"levelCode"`
	SubjectID        string                      `json:"subjectId,omitempty"`
	SubjectCode      string                      `json:"subjectCode,omitempty"`
	Difficulty       string                      `json:"difficulty"`
	GenerationMode   string                      `json:"generationMode"`
	QuestionType     string                      `json:"questionType"`
	ShowFurigana     bool                        `json:"showFurigana"`
	Category         string                      `json:"category"`
	RandomSeed       string                      `json:"randomSeed"`
	RetryFeedback    string                      `json:"retryFeedback,omitempty"`
	AvoidStems       []string                    `json:"avoidStems,omitempty"`
	QuestionTypePlan map[string]int              `json:"questionTypePlan,omitempty"`
	DiversityPlan    []generatedDiversitySlot    `json:"diversityPlan,omitempty"`
	CurriculumScope  string                      `json:"curriculumScope,omitempty"`
	LearningMemory   learning.AIGenerationMemory `json:"learningMemory"`
}

type generatedDiversitySlot struct {
	Context          string `json:"context"`
	Presentation     string `json:"presentation"`
	KnowledgePointID string `json:"knowledgePointId,omitempty"`
}

var generatedDiversityContexts = []string{
	"家庭与日常生活", "学校与学习", "工作与职场", "购物与餐饮", "交通与出行",
	"旅行与住宿", "公共服务", "健康与运动", "天气与休闲", "朋友与社交",
}

var generatedDiversityPresentations = []string{
	"单句叙述", "两人对话", "短信或邮件", "通知或告示",
	"计划或日程", "请求或建议", "经历或回忆", "比较或选择",
}

func generatedDiversityPlan(count, start int, seed string, points []learning.AIGenerationKnowledgePoint, assignKnowledgePoints bool) []generatedDiversitySlot {
	if count <= 0 {
		return nil
	}
	offset := 0
	if decoded, err := hex.DecodeString(seed); err == nil && len(decoded) > 0 {
		offset = int(decoded[0])
	}
	plan := make([]generatedDiversitySlot, count)
	for i := range plan {
		position := start + i
		plan[i] = generatedDiversitySlot{
			Context:      generatedDiversityContexts[(offset+position)%len(generatedDiversityContexts)],
			Presentation: generatedDiversityPresentations[(offset/len(generatedDiversityContexts)+position*3)%len(generatedDiversityPresentations)],
		}
		if assignKnowledgePoints && len(points) > 0 {
			plan[i].KnowledgePointID = points[position%len(points)].ID
		}
	}
	return plan
}

func validateGeneratedDiversityPlan(questions []generatedQuestion, plan []generatedDiversitySlot) error {
	if len(plan) == 0 {
		return nil
	}
	if len(questions) != len(plan) {
		return errors.New("AI 多样性计划与题量不一致")
	}
	for i, slot := range plan {
		if slot.KnowledgePointID != "" && !slices.Contains(questions[i].KnowledgePointIDs, slot.KnowledgePointID) {
			return fmt.Errorf("AI 第 %d 题未覆盖多样性计划指定的知识点", i+1)
		}
	}
	return nil
}

type generatedOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type generatedMaterial struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type generatedQuestion struct {
	Type              string             `json:"type"`
	Stem              string             `json:"stem"`
	Material          *generatedMaterial `json:"material,omitempty"`
	Options           []generatedOption  `json:"options"`
	CorrectAnswer     json.RawMessage    `json:"correctAnswer"`
	Explanation       string             `json:"explanation"`
	KnowledgePointIDs []string           `json:"knowledgePointIds"`
	SubjectID         string             `json:"subjectId"`
	Difficulty        int                `json:"difficulty"`
}

type generatedQuestionResponse struct {
	Questions []generatedQuestion `json:"questions"`
}

var questionGenerationJSONSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"questions": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"type": map[string]any{"type": "string", "enum": []string{"single_choice", "multiple_choice", "fill_blank", "short_answer"}},
					"stem": map[string]any{"type": "string"},
					"material": map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"title": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}},
						"required":             []string{"title", "content"},
						"additionalProperties": false,
					},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"properties":           map[string]any{"id": map[string]any{"type": "string"}, "label": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}},
							"required":             []string{"id", "text"},
							"additionalProperties": false,
						},
					},
					"correctAnswer": map[string]any{
						"type":                 "object",
						"properties":           map[string]any{"optionIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "acceptable": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "reference": map[string]any{"type": "string"}, "text": map[string]any{"type": "string"}},
						"additionalProperties": false,
					},
					"explanation":       map[string]any{"type": "string"},
					"knowledgePointIds": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"subjectId":         map[string]any{"type": "string"},
					"difficulty":        map[string]any{"type": "integer", "minimum": 1, "maximum": 5},
				},
				"required":             []string{"type", "stem", "options", "correctAnswer", "explanation", "knowledgePointIds", "difficulty"},
				"additionalProperties": false,
			},
		},
	},
	"required":             []string{"questions"},
	"additionalProperties": false,
}

type generationSessionRow struct {
	UserID         string
	LevelID        string
	LevelCode      string
	SubjectID      *string
	SubjectCode    string
	RequestedCount int
	Scope          string
	Status         string
}

type generatedStemRow struct {
	Stem string
}

type generatedQuestionHistoryRow struct {
	LevelID       string
	SubjectID     string
	Type          string
	Stem          string
	Options       *string
	MaterialTitle *string
	Material      *string
	Answer        *string
	Difficulty    int
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
		`SELECT v.level_id::text, v.subject_id::text, v.type, v.stem, v.options::text, mv.title, mv.content, aga.value::text, v.difficulty
		 FROM practice_items pi
		 JOIN practice_sessions ps ON ps.id = pi.session_id
		 JOIN question_versions v ON v.id = pi.question_version_id
		 JOIN source_sections ss ON ss.id = v.source_section_id
		 JOIN sources src ON src.id = ss.source_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
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
	if row.Material != nil && strings.TrimSpace(*row.Material) != "" {
		question.Material = &generatedMaterial{Content: *row.Material}
		if row.MaterialTitle != nil {
			question.Material.Title = *row.MaterialTitle
		}
	}
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
		`SELECT ps.user_id::text, ps.level_id::text, l.code, ps.subject_id::text, coalesce(sub.code, ''), ps.requested_count, ps.scope::text, ps.status
		 FROM practice_sessions ps
		 JOIN exam_levels l ON l.id = ps.level_id
		 LEFT JOIN subjects sub ON sub.id = ps.subject_id
		 WHERE ps.id = $1`, req.SessionID,
	).Scan(&row.UserID, &row.LevelID, &row.LevelCode, &row.SubjectID, &row.SubjectCode, &row.RequestedCount, &row.Scope, &row.Status)
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
	recentStems, err := s.loadGeneratedStems(ctx, s.pool, row.UserID, row.LevelID, subjectID, maxRecentGeneratedStemsForSimilarity)
	if err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, fmt.Errorf("读取历史 AI 题干失败: %w", err))
	}
	avoidStems := append([]string(nil), recentStems[:min(len(recentStems), maxRecentGeneratedStemsInPrompt)]...)
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
		var questionTypePlan map[string]int
		if questionType == generatedQuestionTypeMixed {
			questionTypePlan = questionTypePlanFor(row.RequestedCount, generatedQuestions)
		}
		seed, err := randomSeed()
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		var diversityPlan []generatedDiversitySlot
		if !isReadingGeneration(row.SubjectCode, category) {
			diversityPlan = generatedDiversityPlan(remaining, len(generatedQuestions), seed, memory.KnowledgePoints,
				category == generatedCategoryMixed)
		}
		systemPrompt := questionGenerationPrompt + questionGenerationPromptAddendum
		feedback := ""
		temperature := 0.7
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
			Count: remaining, LevelID: row.LevelID, LevelCode: row.LevelCode, SubjectID: subjectID, SubjectCode: row.SubjectCode, Difficulty: difficulty,
			GenerationMode: generationMode, QuestionType: questionType, ShowFurigana: scope.ShowFurigana, Category: category,
			RandomSeed: seed, RetryFeedback: feedback, AvoidStems: avoidStems, QuestionTypePlan: questionTypePlan, DiversityPlan: diversityPlan,
			CurriculumScope: generationCurriculumScope(row.LevelCode), LearningMemory: memory,
		})
		reserved, err := s.reserveGenerationCall(ctx, req.SessionID)
		if err != nil {
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		if !reserved {
			cause := fmt.Errorf("本批 AI 模型调用已达到 %d 次上限，已停止继续重试", s.generationCallBudget)
			s.recordGenerationError(ctx, req.SessionID, cause)
			if err := s.markGenerationFailed(ctx, req.SessionID, cause); err != nil {
				return err
			}
			return nil
		}
		out, runID, err := s.client.RunPromptForGenerationAndAudit(ctx, row.UserID, "practice_question_generation", promptVersion,
			req.SessionID, systemPrompt, string(inputJSON), questionGenerationJSONSchema, temperature)
		if err != nil {
			s.recordGenerationError(ctx, req.SessionID, err)
			if nonRetryableGenerationError(err) {
				if markErr := s.markGenerationFailed(ctx, req.SessionID, err); markErr != nil {
					return markErr
				}
				return nil
			}
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		var response generatedQuestionResponse
		if err := strictDecode(out, &response); err != nil {
			s.markBusinessFailure(ctx, runID, "business_structure", err)
			s.recordGenerationError(ctx, req.SessionID, err)
			validationErr = fmt.Errorf("AI 出题输出不合法: %w", err)
			retryNote = ""
			continue
		}
		questions := capGeneratedQuestions(response.Questions, remaining)
		questions, candidateValidationErr := validateGeneratedQuestionCandidates(questions, remaining, difficulty, questionType,
			memory.KnowledgePoints, row.LevelCode, row.SubjectCode, category, diversityPlan, questionTypePlan)
		// 阅读材料的数量、复用和题量分布是跨题约束，无法安全地逐题拆开。
		if isReadingGeneration(row.SubjectCode, category) && candidateValidationErr == nil {
			candidateValidationErr = validateGeneratedReadingQuestions(row.SubjectCode, category, questions)
		}
		if len(questions) == 0 || (isReadingGeneration(row.SubjectCode, category) && candidateValidationErr != nil) {
			if candidateValidationErr == nil {
				candidateValidationErr = errors.New("AI 本轮没有返回可验收的题目")
			}
			s.markBusinessFailure(ctx, runID, "business_semantic", candidateValidationErr)
			s.recordGenerationError(ctx, req.SessionID, candidateValidationErr)
			validationErr = candidateValidationErr
			retryNote = ""
			continue
		}
		blockedKeys := append([]string{}, existingKeys...)
		generatedKeys, err := generatedQuestionKeys(row.LevelID, subjectID, generatedQuestions, generatedQuestionPoints)
		if err != nil {
			s.markBusinessFailure(ctx, runID, "business_semantic", err)
			s.recordGenerationError(ctx, req.SessionID, err)
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		blockedKeys = append(blockedKeys, generatedKeys...)
		blockedStems := append([]string(nil), recentStems...)
		blockedStems = append(blockedStems, generatedQuestionStems(generatedQuestions)...)
		uniqueQuestions, duplicates, err := filterGeneratedQuestionDuplicates(questions, row.LevelID, subjectID, generatedQuestionPoints, blockedKeys, blockedStems)
		if err != nil {
			s.markBusinessFailure(ctx, runID, "business_semantic", err)
			s.recordGenerationError(ctx, req.SessionID, err)
			return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
		}
		if candidateValidationErr != nil {
			s.markBusinessFailure(ctx, runID, "business_semantic", candidateValidationErr)
			s.recordGenerationError(ctx, req.SessionID, candidateValidationErr)
		} else {
			s.markBusinessSuccess(ctx, runID)
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
		validationErr = candidateValidationErr
	}
	if len(generatedQuestions) != row.RequestedCount {
		if validationErr == nil {
			validationErr = fmt.Errorf("AI 题目去重后数量不足：需要 %d 道，实际 %d 道", row.RequestedCount, len(generatedQuestions))
		}
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, validationErr)
	}
	if err := validateGeneratedQuestionTypePlan(generatedQuestions, questionType, mixedQuestionTypePlan(row.RequestedCount)); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
	}
	if err := validateGeneratedReadingQuestions(row.SubjectCode, category, generatedQuestions); err != nil {
		return s.generationRetry(ctx, req.SessionID, attempts, maxAttempts, err)
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

func validateGeneratedQuestionCandidates(questions []generatedQuestion, expected int, difficulty, questionType string,
	points []learning.AIGenerationKnowledgePoint, levelCode, subjectCode, category string,
	diversityPlan []generatedDiversitySlot, questionTypePlan map[string]int,
) ([]generatedQuestion, error) {
	accepted := make([]generatedQuestion, 0, len(questions))
	typeCounts := make(map[string]int, len(questionTypePlan))
	rejected := expected - len(questions)
	var firstErr error
	if rejected > 0 {
		firstErr = fmt.Errorf("模型只返回了 %d/%d 道题", len(questions), expected)
	}
	for i := range questions {
		candidate := questions[i : i+1]
		err := normalizeGeneratedQuestionAnswers(candidate)
		if err == nil {
			err = validateGeneratedQuestions(candidate, 1, difficulty, questionType, points, subjectCode, category)
		}
		if err == nil && len(diversityPlan) > 0 {
			if i >= len(diversityPlan) {
				err = errors.New("缺少对应的多样性计划")
			} else {
				err = validateGeneratedDiversityPlan(candidate, diversityPlan[i:i+1])
			}
		}
		if err == nil && questionType == generatedQuestionTypeMixed && len(questionTypePlan) > 0 && typeCounts[questions[i].Type] >= questionTypePlan[questions[i].Type] {
			err = fmt.Errorf("题型 %s 超出本轮配额", questions[i].Type)
		}
		if err == nil {
			err = validateGeneratedQuestionLevel(levelCode, subjectCode, candidate)
		}
		if err == nil {
			err = validateGeneratedReadingQuestions(subjectCode, category, candidate)
		}
		if err != nil {
			rejected++
			if firstErr == nil {
				firstErr = fmt.Errorf("第 %d 道候选题：%w", i+1, err)
			}
			continue
		}
		typeCounts[questions[i].Type]++
		accepted = append(accepted, questions[i])
	}
	if rejected > 0 {
		return accepted, fmt.Errorf("本轮 %d 道候选题未通过逐题验收，首个问题：%w", rejected, firstErr)
	}
	return accepted, nil
}

// normalizeGeneratedQuestionAnswers 兼容模型把答题 DTO 的 text 字段误用于简答题，
// 但只接受非空字符串，不替模型猜答案。
func normalizeGeneratedQuestionAnswers(questions []generatedQuestion) error {
	for i := range questions {
		question := &questions[i]
		var value map[string]json.RawMessage
		if err := json.Unmarshal(question.CorrectAnswer, &value); err != nil {
			continue
		}
		if question.Type == "short_answer" {
			if raw, ok := value["reference"]; ok {
				var reference string
				if err := json.Unmarshal(raw, &reference); err == nil && strings.TrimSpace(reference) != "" {
					continue
				}
			}
			if raw, ok := value["text"]; ok {
				var text string
				if err := json.Unmarshal(raw, &text); err == nil && strings.TrimSpace(text) != "" {
					updated, _ := json.Marshal(map[string]string{"reference": strings.TrimSpace(text)})
					question.CorrectAnswer = updated
					continue
				}
			}
			return fmt.Errorf("AI 第 %d 题简答答案必须是非空字符串 reference", i+1)
		}
		if question.Type == "fill_blank" {
			if raw, ok := value["acceptable"]; ok {
				var answer string
				if err := json.Unmarshal(raw, &answer); err == nil && strings.TrimSpace(answer) != "" {
					updated, _ := json.Marshal(map[string][]string{"acceptable": {strings.TrimSpace(answer)}})
					question.CorrectAnswer = updated
				}
			}
		}
	}
	return nil
}

func randomSeed() (string, error) {
	var seed [16]byte
	if _, err := cryptorand.Read(seed[:]); err != nil {
		return "", fmt.Errorf("生成随机种子失败: %w", err)
	}
	return hex.EncodeToString(seed[:]), nil
}

func validateGeneratedQuestions(questions []generatedQuestion, expected int, difficulty, questionType string, points []learning.AIGenerationKnowledgePoint, readingContext ...string) error {
	subjectCode, category := "", ""
	if len(readingContext) > 0 {
		subjectCode = readingContext[0]
	}
	if len(readingContext) > 1 {
		category = readingContext[1]
	}
	if len(questions) != expected {
		return fmt.Errorf("AI 出题数量不正确：需要 %d 道，实际 %d 道", expected, len(questions))
	}
	allowed := make(map[string]bool, len(points))
	for _, point := range points {
		allowed[point.ID] = true
	}
	for i := range questions {
		questions[i].Explanation = sanitizeAIExplanation(strings.TrimSpace(questions[i].Explanation))
		question := questions[i]
		if !questionTypeMatches(questionType, question.Type) || len([]rune(strings.TrimSpace(question.Stem))) < 2 {
			return fmt.Errorf("AI 第 %d 题题型或题干不合法", i+1)
		}
		stem := strings.TrimSpace(question.Stem)
		options := make([]content.Option, 0, len(question.Options))
		if content.IsChoiceType(question.Type) {
			if !isReadingGeneration(subjectCode, category) && !choiceStemHasBlank(stem) {
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
		if !difficultyMatches(difficulty, question.Difficulty) {
			return fmt.Errorf("AI 第 %d 题难度不合法", i+1)
		}
		if !validAIExplanation(question.Explanation) {
			return fmt.Errorf("AI 第 %d 题解析必须以“%s”开头且不超过 2000 字", i+1, aiTranslationPrefix)
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

var generatedFuriganaPattern = regexp.MustCompile(`（[ぁ-ゖゝゞー]+）`)
var generatedBlankReplacer = strings.NewReplacer("＿＿＿", "□", "___", "□", "（　）", "□", "（ ）", "□", "()", "□")

func normalizeGeneratedStemForSimilarity(stem string) []rune {
	stem = generatedBlankReplacer.Replace(generatedFuriganaPattern.ReplaceAllString(stem, ""))
	var normalized strings.Builder
	for _, r := range strings.ToLower(stem) {
		if r == '□' || unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(r)
		}
	}
	return []rune(normalized.String())
}

// ponytail: character bigrams catch cheap template swaps; use semantic embeddings only if measured false negatives justify the cost.
func generatedStemSimilarity(first, second string) float64 {
	a, b := normalizeGeneratedStemForSimilarity(first), normalizeGeneratedStemForSimilarity(second)
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	if string(a) == string(b) {
		return 1
	}
	if len(a) < 8 || len(b) < 8 {
		return 0
	}
	shingles := func(value []rune) map[string]struct{} {
		out := make(map[string]struct{}, len(value)-1)
		for i := 0; i < len(value)-1; i++ {
			out[string(value[i:i+2])] = struct{}{}
		}
		return out
	}
	firstShingles, secondShingles := shingles(a), shingles(b)
	intersection := 0
	for shingle := range firstShingles {
		if _, ok := secondShingles[shingle]; ok {
			intersection++
		}
	}
	return float64(2*intersection) / float64(len(firstShingles)+len(secondShingles))
}

func generatedStemTooSimilar(stem string, existing []string) bool {
	for _, candidate := range existing {
		if generatedStemSimilarity(stem, candidate) >= generatedStemSimilarityThreshold {
			return true
		}
	}
	return false
}

func generatedQuestionReuseKey(levelID, subjectID string, question generatedQuestion) string {
	correctOptionIDs := map[string]struct{}{}
	var optionAnswer struct {
		OptionIDs []string `json:"optionIds"`
	}
	if json.Unmarshal(question.CorrectAnswer, &optionAnswer) == nil {
		for _, id := range optionAnswer.OptionIDs {
			correctOptionIDs[id] = struct{}{}
		}
	}
	type optionKey struct {
		Text    string `json:"text"`
		Correct bool   `json:"correct"`
	}
	options := make([]optionKey, 0, len(question.Options))
	for _, option := range question.Options {
		_, correct := correctOptionIDs[option.ID]
		options = append(options, optionKey{Text: strings.TrimSpace(option.Text), Correct: correct})
	}
	sort.SliceStable(options, func(i, j int) bool {
		if options[i].Text != options[j].Text {
			return options[i].Text < options[j].Text
		}
		return !options[i].Correct && options[j].Correct
	})
	answer := canonicalJSON(question.CorrectAnswer)
	if len(correctOptionIDs) > 0 {
		answer = nil
	}
	var material *generatedMaterial
	if question.Material != nil {
		material = &generatedMaterial{
			Title: strings.TrimSpace(question.Material.Title), Content: strings.TrimSpace(question.Material.Content),
		}
	}
	canonical := struct {
		LevelID   string             `json:"levelId"`
		SubjectID string             `json:"subjectId"`
		Type      string             `json:"type"`
		Stem      string             `json:"stem"`
		Material  *generatedMaterial `json:"material,omitempty"`
		Options   []optionKey        `json:"options"`
		Answer    json.RawMessage    `json:"answer"`
	}{
		LevelID: levelID, SubjectID: subjectID, Type: question.Type,
		Stem: normalizeGeneratedStem(question.Stem), Material: material,
		Options: options, Answer: answer,
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

func filterGeneratedQuestionDuplicates(questions []generatedQuestion, levelID, subjectID string, points []learning.AIGenerationKnowledgePoint, existingKeys, existingStems []string) ([]generatedQuestion, []string, error) {
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
		_, exactDuplicate := keys[key]
		nearDuplicate := question.Material == nil && generatedStemTooSimilar(question.Stem, existingStems)
		if exactDuplicate || nearDuplicate {
			if _, seen := seenDuplicates[key]; !seen {
				duplicates = append(duplicates, strings.TrimSpace(question.Stem))
				seenDuplicates[key] = struct{}{}
			}
			continue
		}
		filtered = append(filtered, question)
		keys[key] = struct{}{}
		existingStems = append(existingStems, question.Stem)
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
	QuestionID    string
	VersionID     string
	Type          string
	Stem          string
	Options       *string
	MaterialTitle *string
	Material      *string
	Answer        *string
	Difficulty    int
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
		`SELECT q.id::text, v.id::text, v.type, v.stem, v.options::text, mv.title, mv.content, aga.value::text, v.difficulty
		 FROM question_versions v
		 JOIN questions q ON q.id = v.question_id
		 JOIN source_sections ss ON ss.id = v.source_section_id
		 JOIN sources src ON src.id = ss.source_id
		 LEFT JOIN material_versions mv ON mv.id = v.material_version_id
		 LEFT JOIN ai_generated_question_answers aga ON aga.question_version_id = v.id
		 WHERE src.kind = 'ai_generated' AND v.ai_reuse_key IS NULL
		   AND v.level_id::text = $1 AND v.subject_id::text = $2 AND v.type = $3`, levelID, subjectID, question.Type)
	if err != nil {
		return "", "", false, fmt.Errorf("查找历史 AI 题目失败: %w", err)
	}
	for _, candidate := range candidates {
		history, err := generatedQuestionFromHistoryRow(generatedQuestionHistoryRow{
			LevelID: levelID, SubjectID: subjectID, Type: candidate.Type, Stem: candidate.Stem,
			Options: candidate.Options, MaterialTitle: candidate.MaterialTitle, Material: candidate.Material,
			Answer: candidate.Answer, Difficulty: candidate.Difficulty,
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
		if err := jobs.GuardLease(ctx, tx); err != nil {
			return err
		}
		existingKeys, err := s.loadGeneratedQuestionKeys(ctx, tx, userID, levelID, subjectID)
		if err != nil {
			return fmt.Errorf("检查历史 AI 题目失败: %w", err)
		}
		recentStems, err := s.loadGeneratedStems(ctx, tx, userID, levelID, subjectID, maxRecentGeneratedStemsForSimilarity)
		if err != nil {
			return fmt.Errorf("读取近期 AI 题干失败: %w", err)
		}
		if _, duplicates, err := filterGeneratedQuestionDuplicates(questions, levelID, subjectID, points, existingKeys, recentStems); err != nil {
			return err
		} else if len(duplicates) > 0 {
			return fmt.Errorf("AI 题目与历史重复或高度相似：%s", strings.Join(duplicates, "；"))
		}
		sectionName := "根据全局记忆生成"
		if generationMode == generationModeLevel {
			sectionName = "根据当前级别生成"
		}
		var sourceID, sectionID string
		materialVersionIDs := map[string]string{}
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
				var materialVersionID *string
				if question.Material != nil {
					materialKey := normalizeGeneratedStem(strings.TrimSpace(question.Material.Title) + "\n" + strings.TrimSpace(question.Material.Content))
					versionID, ok := materialVersionIDs[materialKey]
					if !ok {
						_, createdVersionID, err := content.NewStore(tx).CreateMaterial(ctx, strings.TrimSpace(question.Material.Title), strings.TrimSpace(question.Material.Content), userID)
						if err != nil {
							return fmt.Errorf("保存 AI 阅读材料失败: %w", err)
						}
						versionID = createdVersionID
						materialVersionIDs[materialKey] = versionID
					}
					materialVersionID = &versionID
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO questions (status, has_answer, created_by)
					 VALUES ('draft', false, $1) RETURNING id::text`, userID).Scan(&questionID); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx,
					`INSERT INTO question_versions
					 (question_id, version_no, type, stem, material_version_id, options, level_id, subject_id, source_section_id, difficulty, source_order, created_by, ai_reuse_key)
					 VALUES ($1, 1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
					RETURNING id::text`, questionID, question.Type, strings.TrimSpace(question.Stem), materialVersionID, optionsJSON,
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
			`UPDATE practice_sessions SET status = 'active', ai_generation_last_error = '', updated_at = now() WHERE id = $1 AND status = 'generating'`, sessionID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO audit_logs (actor_user_id, action, object_type, object_id, detail)
			 VALUES ($1, 'ai_practice_generated', 'practice_session', $2, jsonb_build_object('count', $3::int))`, userID, sessionID, len(questions))
		return err
	})
}

func (s *Service) generationRetry(ctx context.Context, sessionID string, attempts, maxAttempts int, cause error) error {
	s.recordGenerationError(ctx, sessionID, cause)
	if attempts < maxAttempts {
		return cause
	}
	if err := s.markGenerationFailed(ctx, sessionID, cause); err != nil {
		return fmt.Errorf("标记 AI 出题失败失败: %v（原错误：%w）", err, cause)
	}
	return cause
}

func (s *Service) reserveGenerationCall(ctx context.Context, sessionID string) (bool, error) {
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := jobs.GuardLease(ctx, tx); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`UPDATE practice_sessions
			 SET ai_generation_calls_used = ai_generation_calls_used + 1, updated_at = now()
			 WHERE id = $1 AND status = 'generating'
			   AND ai_generation_calls_used < ai_generation_call_budget`, sessionID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errGenerationBudgetExceeded
		}
		return nil
	})
	if errors.Is(err, errGenerationBudgetExceeded) {
		return false, nil
	}
	return err == nil, err
}

func (s *Service) recordGenerationError(ctx context.Context, sessionID string, cause error) {
	if cause == nil {
		return
	}
	message := shortError(cause)
	_ = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := jobs.GuardLease(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE practice_sessions SET ai_generation_last_error = left($2, 500), updated_at = now()
			 WHERE id = $1 AND status = 'generating'`, sessionID, message)
		return err
	})
}

func nonRetryableGenerationError(err error) bool {
	var responseErr *httpResponseError
	if errors.As(err, &responseErr) {
		return responseErr.status >= 400 && responseErr.status < 500 && responseErr.status != http.StatusRequestTimeout && responseErr.status != http.StatusTooManyRequests
	}
	var notConfigured notConfiguredError
	return errors.As(err, &notConfigured)
}

func (s *Service) markGenerationFailed(ctx context.Context, sessionID string, cause error) error {
	message := "AI 出题失败，请重新开始。"
	if cause != nil {
		message += shortError(cause)
	}
	return store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := jobs.GuardLease(ctx, tx); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE practice_sessions
			 SET status = 'generation_failed', ai_summary_status = 'failed', ai_summary = $2,
			     ai_generation_last_error = left($2, 500), updated_at = now()
			 WHERE id = $1 AND status = 'generating'`, sessionID, message)
		return err
	})
}
