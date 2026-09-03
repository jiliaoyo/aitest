package practice

import (
	"context"
	"encoding/json"
	"net/http"

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
	Mode              string   `json:"mode"`           // comprehensive | knowledge | wrong_items
	SelectionOrder    string   `json:"selectionOrder"` // source_order | random
	KnowledgePointIDs []string `json:"knowledgePointIds"`
	SourceID          string   `json:"sourceId"`
	SourceSectionID   string   `json:"sourceSectionId"`
	Count             int      `json:"count"`
}

var validCounts = map[int]bool{10: true, 20: true, 30: true}

const (
	SelectionOrderSource = "source_order"
	SelectionOrderRandom = "random"
)

// Availability 返回当前筛选下可用于练习的题目数量。
func (s *Service) Availability(ctx context.Context, userID string, req CreateRequest) (int, error) {
	f, err := s.selectionFilter(ctx, userID, req)
	if err != nil {
		return 0, err
	}
	return s.contentStore.CountPublishedVersions(ctx, f)
}

func (s *Service) PracticeSources(ctx context.Context, levelID, subjectID string) ([]content.PracticeSource, error) {
	if levelID == "" {
		return nil, httpapi.ValidationError(map[string]string{"levelId": "请选择级别"})
	}
	return s.contentStore.ListPracticeSources(ctx, levelID, subjectID)
}

func (s *Service) selectionFilter(ctx context.Context, userID string, req CreateRequest) (content.SelectionFilter, error) {
	if !validCounts[req.Count] {
		return content.SelectionFilter{}, httpapi.ValidationError(map[string]string{"count": "题量只能是 10、20 或 30"})
	}
	if req.Mode == "" {
		req.Mode = "comprehensive"
	}
	if req.SelectionOrder == "" {
		req.SelectionOrder = SelectionOrderSource
	}
	if req.SelectionOrder != SelectionOrderSource && req.SelectionOrder != SelectionOrderRandom {
		return content.SelectionFilter{}, httpapi.ValidationError(map[string]string{"selectionOrder": "出题顺序不合法"})
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
		ids, err := s.store.WrongQuestionIDs(ctx, userID, req.Count)
		if err != nil {
			return f, err
		}
		if len(ids) == 0 {
			return f, httpapi.E(http.StatusConflict, "no_wrong_items", "当前没有可重练的错题")
		}
		f.QuestionIDs = ids
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
		selected, err := s.contentStore.With(tx).SelectPublishedVersions(ctx, tx, f)
		if err != nil {
			return err
		}
		if len(selected) < req.Count {
			return httpapi.WithDetails(httpapi.E(http.StatusConflict, "insufficient_questions",
				"当前范围的可用题目不足"), map[string]any{"available": len(selected)})
		}
		scope, _ := json.Marshal(map[string]any{
			"mode":              scopeMode,
			"selectionOrder":    f.SelectionOrder,
			"subjectId":         req.SubjectID,
			"sourceId":          req.SourceID,
			"sourceSectionId":   req.SourceSectionID,
			"knowledgePointIds": req.KnowledgePointIDs,
		})
		var subjectIDPtr *string
		if req.SubjectID != "" {
			subjectIDPtr = &req.SubjectID
		}
		sessionID, err = s.store.With(tx).InsertSession(ctx, tx, userID, req.LevelID, subjectIDPtr, scope, req.Count)
		if err != nil {
			return err
		}
		seeds := make([]ItemSeed, 0, len(selected))
		for _, sel := range selected {
			seeds = append(seeds, ItemSeed{QuestionID: sel.QuestionID, VersionID: sel.VersionID})
		}
		return s.store.With(tx).InsertItems(ctx, tx, sessionID, seeds)
	})
	if err != nil {
		return PreSubmitSession{}, err
	}
	return s.GetPreSubmit(ctx, userID, sessionID)
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
		ID:            meta.ID,
		Status:        meta.Status,
		AnsweredCount: answered,
		TotalCount:    len(items),
		Items:         items,
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

// Submit 在一个事务内完成：锁定批次 → 幂等校验 → 覆盖最终答案 → 确定性判分 → AI 任务入队。
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
			if row.KeyValue != nil && row.KeyAuthority != nil {
				key = &StandardKey{Value: jsonRaw(*row.KeyValue), Authority: *row.KeyAuthority}
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
			if row.Explanation != nil && *row.Explanation != "" {
				explanation = row.Explanation
				if authority != nil {
					explanationSource = authority
				}
			}
			if err := st.InsertGrading(ctx, tx, GradingInsert{
				SessionID: sessionID, ItemID: row.ItemID,
				Source: SourceDeterministic,
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
		return jobs.EnqueueTx(ctx, tx, "rebuild_user_knowledge_stats", map[string]string{"userId": userID})
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

// ---------- 历史 ----------

func (s *Service) ListSessions(ctx context.Context, userID, status, cursor string, limit int) ([]SessionListItem, string, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	return s.store.ListSessions(ctx, userID, status, cursor, limit)
}
