package practice

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/jobs"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool         *pgxpool.Pool
	store        *Store
	contentStore *content.Store
}

func NewService(pool *pgxpool.Pool, contentStore *content.Store) *Service {
	return &Service{pool: pool, store: NewStore(pool), contentStore: contentStore}
}

// ---------- 创建批次 ----------

type CreateRequest struct {
	LevelID           string   `json:"levelId"`
	SubjectID         string   `json:"subjectId"`
	Mode              string   `json:"mode"`           // comprehensive | knowledge | wrong_items | review
	SelectionOrder    string   `json:"selectionOrder"` // source_order | random
	KnowledgePointIDs []string `json:"knowledgePointIds"`
	SourceID          string   `json:"sourceId"`
	SourceSectionID   string   `json:"sourceSectionId"`
	FromDate          string   `json:"from"`
	ToDate            string   `json:"to"`
	Keyword           string   `json:"keyword"`
	Count             int      `json:"count"`
}

const (
	SelectionOrderSource = "source_order"
	SelectionOrderRandom = "random"
	SelectionOrderUnseen = "unseen_first"
	maxReviewBatchCount  = 10
)

// Availability 返回当前筛选下可用于练习的题目数量。
func (s *Service) Availability(ctx context.Context, userID string, req CreateRequest) (int, error) {
	f, err := s.selectionFilter(ctx, userID, req)
	if err != nil {
		return 0, err
	}
	if req.Mode == "review" {
		items, err := s.store.DueReviewItems(ctx, userID, req.LevelID, req.SubjectID, 0)
		return len(items), err
	}
	return s.contentStore.CountPublishedVersions(ctx, f)
}

func (s *Service) PracticeSources(ctx context.Context, userID, levelID, subjectID string) ([]content.PracticeSource, error) {
	if levelID == "" {
		return nil, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
	}
	return s.contentStore.ListPracticeSources(ctx, levelID, subjectID, userID)
}

func (s *Service) selectionFilter(ctx context.Context, userID string, req CreateRequest) (content.SelectionFilter, error) {
	if req.Count < 1 || req.Count > 30 {
		return content.SelectionFilter{}, httpapi.ValidationError(map[string]string{"count": "题量必须是 1 到 30"})
	}
	if req.Mode == "" {
		req.Mode = "comprehensive"
	}
	if req.SelectionOrder == "" {
		req.SelectionOrder = SelectionOrderSource
	}
	if req.SelectionOrder != SelectionOrderSource && req.SelectionOrder != SelectionOrderRandom && req.SelectionOrder != SelectionOrderUnseen {
		return content.SelectionFilter{}, httpapi.ValidationError(map[string]string{"selectionOrder": "出题顺序不合法"})
	}
	if req.Mode == "review" && req.Count > maxReviewBatchCount {
		return content.SelectionFilter{}, httpapi.ValidationError(map[string]string{"count": "到期复习每批最多 10 题"})
	}
	f := content.SelectionFilter{
		UserID:          userID,
		LevelID:         req.LevelID,
		SubjectID:       req.SubjectID,
		SourceID:        req.SourceID,
		SourceSectionID: req.SourceSectionID,
		SelectionOrder:  req.SelectionOrder,
		Limit:           req.Count,
		ExcludeRecent:   req.Mode == "comprehensive" || req.Mode == "knowledge",
		UnseenOnly:      req.SelectionOrder == SelectionOrderUnseen,
	}
	switch req.Mode {
	case "comprehensive":
		if req.LevelID == "" {
			return f, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
		}
	case "knowledge":
		if req.LevelID == "" {
			return f, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
		}
		if len(req.KnowledgePointIDs) == 0 {
			return f, httpapi.ValidationError(map[string]string{"knowledgePointIds": "请至少选择一个知识点"})
		}
		f.KnowledgePointIDs = req.KnowledgePointIDs
	case "wrong_items":
		if req.LevelID == "" {
			return f, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
		}
		if err := validateWrongFilters(req.FromDate, req.ToDate, req.Keyword); err != nil {
			return f, err
		}
		ids, err := s.store.WrongQuestionIDs(ctx, WrongQuestionFilter{
			UserID: userID, LevelID: req.LevelID, SubjectID: req.SubjectID,
			SourceID: req.SourceID, SourceSectionID: req.SourceSectionID,
			KnowledgePointIDs: req.KnowledgePointIDs,
			FromDate:          req.FromDate, ToDate: req.ToDate, Keyword: req.Keyword,
		})
		if err != nil {
			return f, err
		}
		if len(ids) == 0 {
			return f, httpapi.E(http.StatusConflict, "no_wrong_items", "当前没有可重练的错题")
		}
		f.QuestionIDs = ids
		f.ExcludeRecent = false
	case "review":
		if req.LevelID == "" {
			return f, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
		}
		f.ExcludeRecent = false
	default:
		return f, httpapi.ValidationError(map[string]string{"mode": "练习范围不合法"})
	}
	return f, nil
}

// CreateSession 在一个事务中完成选题、写批次与题目快照引用。
func (s *Service) CreateSession(ctx context.Context, userID string, req CreateRequest) (PreSubmitSession, error) {
	f, err := s.selectionFilter(ctx, userID, req)
	if err != nil {
		return PreSubmitSession{}, err
	}
	scopeMode := req.Mode
	if scopeMode == "" {
		scopeMode = "comprehensive"
	}
	var sessionID string
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		seeds := make([]ItemSeed, 0, req.Count)
		if scopeMode == "review" {
			st := s.store.With(tx)
			if err := st.LockUser(ctx, tx, userID); err != nil {
				return err
			}
			if activeID, err := st.ActiveReviewSessionID(ctx, userID); err != nil {
				return err
			} else if activeID != "" {
				return httpapi.WithDetails(httpapi.E(http.StatusConflict, "review_in_progress", "已有一批到期复习正在进行"), map[string]any{"sessionId": activeID})
			}
			seeds, err = s.store.With(tx).DueReviewItems(ctx, userID, req.LevelID, req.SubjectID, req.Count)
		} else {
			var selected []content.SelectedQuestion
			selected, err = s.contentStore.With(tx).SelectPublishedVersions(ctx, tx, f)
			for _, item := range selected {
				seeds = append(seeds, ItemSeed{QuestionID: item.QuestionID, VersionID: item.VersionID})
			}
		}
		if err != nil {
			return err
		}
		if len(seeds) < req.Count {
			if scopeMode == "review" && len(seeds) == 0 {
				return httpapi.E(http.StatusConflict, "no_due_reviews", "当前没有到期复习题")
			}
			return httpapi.WithDetails(httpapi.E(http.StatusConflict, "insufficient_questions",
				"当前范围的可用题目不足"), map[string]any{"available": len(seeds)})
		}
		scope, _ := json.Marshal(map[string]any{
			"mode":              scopeMode,
			"selectionOrder":    f.SelectionOrder,
			"subjectId":         req.SubjectID,
			"sourceId":          req.SourceID,
			"sourceSectionId":   req.SourceSectionID,
			"knowledgePointIds": req.KnowledgePointIDs,
			"from":              req.FromDate,
			"to":                req.ToDate,
			"keyword":           req.Keyword,
		})
		var subjectIDPtr *string
		if req.SubjectID != "" {
			subjectIDPtr = &req.SubjectID
		}
		sessionID, err = s.store.With(tx).InsertSession(ctx, tx, userID, req.LevelID, subjectIDPtr, scope, req.Count)
		if err != nil {
			return err
		}
		return s.store.With(tx).InsertItems(ctx, tx, sessionID, seeds)
	})
	if err != nil {
		return PreSubmitSession{}, err
	}
	return s.GetPreSubmit(ctx, userID, sessionID)
}

func validateWrongFilters(fromDate, toDate, keyword string) error {
	if len([]rune(keyword)) > 100 {
		return httpapi.ValidationError(map[string]string{"keyword": "关键词不能超过 100 个字"})
	}
	var from, to time.Time
	var err error
	if fromDate != "" {
		from, err = time.Parse("2006-01-02", fromDate)
		if err != nil {
			return httpapi.ValidationError(map[string]string{"from": "开始日期格式不正确"})
		}
	}
	if toDate != "" {
		to, err = time.Parse("2006-01-02", toDate)
		if err != nil {
			return httpapi.ValidationError(map[string]string{"to": "结束日期格式不正确"})
		}
	}
	if !from.IsZero() && !to.IsZero() && from.After(to) {
		return httpapi.ValidationError(map[string]string{"to": "结束日期不能早于开始日期"})
	}
	return nil
}

// ---------- 读取批次 ----------

func (s *Service) GetPreSubmit(ctx context.Context, userID, sessionID string) (PreSubmitSession, error) {
	meta, err := s.store.SessionMetaForUser(ctx, sessionID, userID)
	if err != nil {
		return PreSubmitSession{}, err
	}
	items, answered, err := s.store.PreSubmitItems(ctx, sessionID)
	if err != nil {
		return PreSubmitSession{}, err
	}
	return PreSubmitSession{
		ID:                   meta.ID,
		Status:               meta.Status,
		Mode:                 meta.Mode,
		AnsweredCount:        answered,
		TotalCount:           len(items),
		GenerationCallsUsed:  meta.GenerationCallsUsed,
		GenerationCallBudget: meta.GenerationCallBudget,
		GenerationLastError:  meta.GenerationLastError,
		Items:                items,
	}, nil
}

// ---------- 自动保存 ----------

type SaveAnswerRequest struct {
	Value           json.RawMessage `json:"value"`
	MarkedForReview bool            `json:"markedForReview"`
}

func (s *Service) SaveAnswer(ctx context.Context, userID, sessionID, itemID string, req SaveAnswerRequest) (savedAt string, err error) {
	var qType string
	var optionsJSON *string
	err = s.pool.QueryRow(ctx,
		`SELECT v.type, v.options::text
		 FROM practice_items pi JOIN question_versions v ON v.id = pi.question_version_id
		 WHERE pi.id = $1 AND pi.session_id = $2`, itemID, sessionID,
	).Scan(&qType, &optionsJSON)
	if err == pgx.ErrNoRows {
		return "", httpapi.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	var valueBytes []byte
	if len(req.Value) > 0 && string(req.Value) != "null" {
		var opts []content.Option
		if optionsJSON != nil {
			_ = json.Unmarshal([]byte(*optionsJSON), &opts)
		}
		if _, verr := ParseAnswerValue(qType, opts, req.Value); verr != nil {
			return "", httpapi.ValidationError(map[string]string{"value": verr.Error()})
		}
		valueBytes = req.Value
	}
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		ok, at, err := s.store.With(tx).UpsertAnswer(ctx, tx, sessionID, itemID, userID, valueBytes, req.MarkedForReview)
		if err != nil {
			return err
		}
		if !ok {
			meta, merr := s.store.SessionMetaForUser(ctx, sessionID, userID)
			if merr != nil {
				return merr
			}
			if meta.Status != "active" {
				return httpapi.E(http.StatusConflict, "practice_not_active", "该练习已提交，不能继续修改")
			}
			return httpapi.ErrNotFound
		}
		savedAt = at
		return nil
	})
	return savedAt, err
}

// ---------- 整批提交 ----------

type SubmittedAnswer struct {
	ItemID          string          `json:"itemId"`
	Value           json.RawMessage `json:"value"`
	MarkedForReview bool            `json:"markedForReview"`
}

type SubmitRequest struct {
	Answers []SubmittedAnswer `json:"answers"`
}

var (
	// errSameResubmit 表示相同幂等键的重复提交，调用方应直接返回现有结果。
	errSameResubmit = errSameKey{}
)

type errSameKey struct{}

func (errSameKey) Error() string { return "same idempotency key" }

// Submit 在一个事务内完成：锁定批次 → 幂等校验 → 覆盖最终答案 → 可直接判分的题目判分 → AI 任务入队。
func (s *Service) Submit(ctx context.Context, userID, sessionID, idemKey, bodyHash string, req SubmitRequest) (int, error) {
	if idemKey == "" {
		return 0, httpapi.E(http.StatusBadRequest, "missing_idempotency_key", "缺少 Idempotency-Key 请求头")
	}
	aiJobs := 0
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.With(tx)
		var meta SessionMeta
		err := tx.QueryRow(ctx,
			`SELECT id::text, user_id::text, status, submit_key, submit_hash
			 FROM practice_sessions WHERE id = $1 AND user_id = $2 FOR UPDATE`,
			sessionID, userID,
		).Scan(&meta.ID, &meta.UserID, &meta.Status, &meta.SubmitKey, &meta.SubmitHash)
		if err == pgx.ErrNoRows {
			return httpapi.ErrNotFound
		}
		if err != nil {
			return err
		}
		if meta.Status != "active" {
			if meta.SubmitKey != nil && *meta.SubmitKey == idemKey {
				if meta.SubmitHash != nil && *meta.SubmitHash == bodyHash {
					return errSameResubmit
				}
				return httpapi.E(http.StatusConflict, "idempotency_conflict", "相同幂等键但提交内容不同")
			}
			return httpapi.E(http.StatusConflict, "practice_not_active", "该练习已提交")
		}

		// 校验请求中的题目与答案
		itemTypes, err := st.loadItemTypes(ctx, sessionID)
		if err != nil {
			return err
		}
		byItem := map[string]SubmittedAnswer{}
		for _, a := range req.Answers {
			t, ok := itemTypes[a.ItemID]
			if !ok {
				return httpapi.ValidationError(map[string]string{"answers": "包含不属于该批次的题目"})
			}
			if len(a.Value) > 0 && string(a.Value) != "null" {
				var opts []content.Option
				if t.optionsJSON != nil {
					_ = json.Unmarshal([]byte(*t.optionsJSON), &opts)
				}
				if _, verr := ParseAnswerValue(t.qType, opts, a.Value); verr != nil {
					return httpapi.ValidationError(map[string]string{"answers": verr.Error()})
				}
			}
			byItem[a.ItemID] = a
		}

		rows, err := st.GradeSourceItems(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		// 提交请求可能省略部分题目（未作答）：这些题目的最终答案必须清空，
		// 不能沿用自动保存的旧值。
		final := make([]SubmittedAnswer, 0, len(rows))
		for _, row := range rows {
			if a, ok := byItem[row.ItemID]; ok {
				final = append(final, a)
			} else {
				final = append(final, SubmittedAnswer{ItemID: row.ItemID})
			}
		}
		if err := st.SubmitFinalAnswers(ctx, tx, sessionID, userID, final); err != nil {
			return err
		}
		if err := st.MarkSubmitted(ctx, tx, sessionID, idemKey, bodyHash); err != nil {
			return err
		}
		for _, row := range rows {
			answer := byItem[row.ItemID]
			userValue := []byte(nil)
			if len(answer.Value) > 0 && string(answer.Value) != "null" {
				userValue = answer.Value
			}
			var key *StandardKey
			gradingSource := SourceDeterministic
			if row.KeyValue != nil && row.KeyAuthority != nil {
				key = &StandardKey{Value: jsonRaw(*row.KeyValue), Authority: *row.KeyAuthority}
			} else if row.GeneratedValue != nil {
				key = &StandardKey{Value: jsonRaw(*row.GeneratedValue)}
				gradingSource = SourceAI
			}
			outcome := Grade(row.Type, userValue, key)
			if outcome.Status == StatusPending {
				if err := st.InsertGrading(ctx, tx, GradingInsert{
					SessionID: sessionID, ItemID: row.ItemID,
					Source: SourceAI, Status: StatusPending, UserValue: userValue,
				}); err != nil {
					return err
				}
				aiJobs++
				continue
			}
			var authority *string
			if outcome.Authority != "" {
				a := outcome.Authority
				authority = &a
			}
			var explanation, explanationSource *string
			if gradingSource == SourceAI && row.GeneratedExplanation != nil && *row.GeneratedExplanation != "" {
				explanation = row.GeneratedExplanation
				source := SourceAI
				explanationSource = &source
			} else if row.Explanation != nil && *row.Explanation != "" {
				explanation = row.Explanation
				if authority != nil {
					explanationSource = authority
				}
			}
			if err := st.InsertGrading(ctx, tx, GradingInsert{
				SessionID: sessionID, ItemID: row.ItemID,
				Source: gradingSource,
				Status: outcome.Status, Authority: authority,
				CorrectValue: outcome.CorrectValue, UserValue: userValue,
				Explanation: explanation, ExplanationSource: explanationSource,
			}); err != nil {
				return err
			}
		}
		if err := jobs.EnqueueTx(ctx, tx, "analyze_practice_session_ai", map[string]string{"sessionId": sessionID}); err != nil {
			return err
		}

		if aiJobs == 0 {
			if err := st.SetSessionStatus(ctx, tx, sessionID, "completed"); err != nil {
				return err
			}
		}
		return jobs.EnqueueUserLearningRebuildTx(ctx, tx, userID)
	})
	if err == errSameResubmit {
		return 0, nil
	}
	return aiJobs, err
}

type itemTypeInfo struct {
	qType       string
	optionsJSON *string
}

func (s *Store) loadItemTypes(ctx context.Context, sessionID string) (map[string]itemTypeInfo, error) {
	rows, err := store.CollectRows[struct {
		ID      string
		Type    string
		Options *string
	}](ctx, s.db,
		`SELECT pi.id::text, v.type, v.options::text
		 FROM practice_items pi JOIN question_versions v ON v.id = pi.question_version_id
		 WHERE pi.session_id = $1`, sessionID)
	if err != nil {
		return nil, err
	}
	out := map[string]itemTypeInfo{}
	for _, r := range rows {
		out[r.ID] = itemTypeInfo{qType: r.Type, optionsJSON: r.Options}
	}
	return out, nil
}

// ---------- 结果 ----------

func (s *Service) GetResult(ctx context.Context, userID, sessionID string) (ResultSession, error) {
	meta, err := s.store.SessionMetaForUser(ctx, sessionID, userID)
	if err != nil {
		return ResultSession{}, err
	}
	if meta.Status == "active" {
		return ResultSession{}, httpapi.E(http.StatusConflict, "practice_not_submitted", "练习尚未提交")
	}
	if meta.Status == "generating" {
		return ResultSession{}, httpapi.E(http.StatusConflict, "practice_generating", "AI 题目仍在生成，请稍后查看")
	}
	if meta.Status == "generation_failed" {
		return ResultSession{}, httpapi.E(http.StatusConflict, "generation_failed", "AI 题目生成失败，请重新开始")
	}
	if meta.Status == "completed" {
		pending, err := s.store.HasPendingAIJobs(ctx, sessionID)
		if err != nil {
			return ResultSession{}, err
		}
		if pending {
			meta.Status = "grading"
		}
	}
	rows, err := s.store.ResultRows(ctx, sessionID)
	if err != nil {
		return ResultSession{}, err
	}
	sum, err := s.store.Summary(ctx, sessionID)
	if err != nil {
		return ResultSession{}, err
	}

	items := map[string]*ResultItem{}
	order := []string{}
	for _, r := range rows {
		item, ok := items[r.ID]
		if !ok {
			item = &ResultItem{
				ID: r.ID, Position: r.Position, Type: r.Type, Stem: r.Stem,
				Options: []PreSubmitOption{}, KnowledgePoints: []ResultKnowledgePoint{},
				GradingStatus: r.Status,
			}
			if r.SourceSectionName != nil {
				item.SourceSectionName = *r.SourceSectionName
			}
			src := r.Source
			item.GradingSource = &src
			if r.OptionsText != nil && *r.OptionsText != "null" {
				var opts []PreSubmitOption
				if jsonUnmarshal(*r.OptionsText, &opts) == nil {
					item.Options = opts
				}
			}
			if r.MaterialID != nil {
				item.Material = &ResultMaterial{ID: *r.MaterialID}
				if r.MaterialTitle != nil {
					item.Material.Title = *r.MaterialTitle
				}
				if r.MaterialContent != nil {
					item.Material.Content = *r.MaterialContent
				}
			}
			if r.UserValue != nil {
				item.UserAnswer = jsonRaw(*r.UserValue)
			} else {
				item.UserAnswer = jsonRaw("null")
			}
			if r.Authority != nil {
				a := *r.Authority
				item.AnswerAuthority = &a
			}
			if r.CorrectValue != nil {
				item.CorrectAnswer = jsonRaw(*r.CorrectValue)
			} else {
				item.CorrectAnswer = jsonRaw("null")
			}
			if r.Explanation != nil && *r.Explanation != "" && r.ExplanationSource != nil {
				item.Explanation = &Explanation{Text: *r.Explanation, Source: *r.ExplanationSource}
			}
			items[r.ID] = item
			order = append(order, r.ID)
		}
		if r.KPID != nil {
			item.KnowledgePoints = append(item.KnowledgePoints, ResultKnowledgePoint{ID: *r.KPID, Name: *r.KPName})
		}
	}

	out := ResultSession{
		ID: meta.ID, Status: meta.Status, CreatedAt: meta.CreatedAt, SubmittedAt: meta.SubmittedAt,
		Summary: ResultSummary{Confirmed: &ConfirmedSummary{Correct: sum.ConfirmedCorrect, Total: sum.ConfirmedTotal},
			AI: &AISummary{Correct: sum.AiCorrect, Completed: sum.AiCompleted, Pending: sum.AiPending, Failed: sum.AiFailed}},
		AIAnalysis: AIAnalysis{Status: meta.AISummaryStatus, Text: meta.AISummary},
		Items:      make([]ResultItem, 0, len(order)),
	}
	if sum.ConfirmedTotal > 0 {
		acc := float64(sum.ConfirmedCorrect) / float64(sum.ConfirmedTotal)
		out.Summary.Confirmed.Accuracy = &acc
	}
	for _, id := range order {
		out.Items = append(out.Items, *items[id])
	}
	return out, nil
}

// RetryAnalysis 恢复失败的 AI 结果，并保证同一批次最多存在一个活动中的整批任务。
func (s *Service) RetryAnalysis(ctx context.Context, userID, sessionID string) (ResultSession, error) {
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.With(tx)
		var lockedID string
		err := tx.QueryRow(ctx,
			`SELECT id::text FROM practice_sessions WHERE id = $1 AND user_id = $2 FOR UPDATE`, sessionID, userID).Scan(&lockedID)
		if err == pgx.ErrNoRows {
			return httpapi.ErrNotFound
		}
		if err != nil {
			return err
		}
		meta, err := st.SessionMetaForUser(ctx, sessionID, userID)
		if err != nil {
			return err
		}
		active, err := st.HasActiveBatchAnalysisJob(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if active {
			return nil
		}
		if meta.Status == "active" {
			return httpapi.E(http.StatusConflict, "practice_not_submitted", "练习尚未提交")
		}
		if meta.Status != "analysis_failed" && meta.AISummaryStatus != "failed" {
			return httpapi.E(http.StatusConflict, "analysis_not_failed", "当前批次没有可重试的 AI 分析")
		}
		if err := st.ResetAIAnalysisForRetry(ctx, tx, sessionID); err != nil {
			return err
		}
		return jobs.EnqueueTx(ctx, tx, "analyze_practice_session_ai", map[string]string{"sessionId": sessionID})
	})
	if err != nil {
		return ResultSession{}, err
	}
	return s.GetResult(ctx, userID, sessionID)
}

// ---------- 历史 ----------

func (s *Service) ListSessions(ctx context.Context, userID, status, mode, cursor string, limit int) ([]SessionListItem, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	if mode != "" {
		switch mode {
		case "comprehensive", "knowledge", "wrong_items", "review", "ai_generated":
		default:
			return nil, "", httpapi.ValidationError(map[string]string{"mode": "练习分类不合法"})
		}
	}
	return s.store.ListSessions(ctx, userID, status, mode, cursor, limit)
}

func (s *Service) DeleteSession(ctx context.Context, userID, sessionID string) error {
	var status string
	err := s.pool.QueryRow(ctx,
		`SELECT status FROM practice_sessions WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, sessionID, userID).Scan(&status)
	if err == pgx.ErrNoRows {
		return httpapi.ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "generating" {
		return httpapi.E(http.StatusConflict, "practice_in_progress", "正在生成的练习不能删除")
	}
	_, err = s.pool.Exec(ctx,
		`UPDATE practice_sessions SET deleted_at = now(), updated_at = now()
		 WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, sessionID, userID)
	return err
}
