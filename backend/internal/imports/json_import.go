package imports

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aishuati/backend/internal/content"
	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// jsonItem 是本地 OCR 服务导出的题目格式：分类用码值和名称表示，
// 服务端在这里解析成内部 ID 后复用 AI 结构化路径的全部校验。
// 级别/科目码值无法识别是硬错误；知识点名称匹配不上只记入 anomalies，
// 避免一个名称对不上导致整批导入被拒绝。
type jsonItem struct {
	RawExcerpt          string            `json:"rawExcerpt"`
	MaterialKey         string            `json:"materialKey"`
	Type                string            `json:"type"`
	Stem                string            `json:"stem"`
	Options             []content.Option  `json:"options"`
	MaterialTitle       string            `json:"materialTitle"`
	MaterialContent     string            `json:"materialContent"`
	LevelCode           string            `json:"levelCode"`
	SubjectCode         string            `json:"subjectCode"`
	Difficulty          int               `json:"difficulty"`
	KnowledgePointNames []string          `json:"knowledgePointNames"`
	SourceAnswer        json.RawMessage   `json:"sourceAnswer"`
	AISuggestedAnswer   *AnswerSuggestion `json:"aiSuggestedAnswer"`
	Anomalies           []string          `json:"anomalies"`
}

type jsonPayload struct {
	Items []jsonItem `json:"items"`
}

func buildAIDraft(raw jsonItem, levelID, subjectID string, kpIDs []string, extraAnomalies []string) aiDraft {
	anomalies := raw.Anomalies
	if anomalies == nil {
		anomalies = []string{}
	}
	anomalies = append(anomalies, extraAnomalies...)
	return aiDraft{RawExcerpt: raw.RawExcerpt, MaterialKey: raw.MaterialKey, Type: raw.Type,
		Stem: raw.Stem, Options: raw.Options, MaterialTitle: raw.MaterialTitle,
		MaterialContent: raw.MaterialContent, LevelID: levelID, SubjectID: subjectID,
		Difficulty: raw.Difficulty, KnowledgePointIDs: kpIDs,
		SourceAnswer: raw.SourceAnswer, AISuggestedAnswer: raw.AISuggestedAnswer, Anomalies: anomalies}
}

// importJSON 解析已保存的 JSON 文件并直接写入待审核条目，跳过文本提取和 AI 结构化。
func (s *Service) importJSON(ctx context.Context, adminID, name, storedPath, digest, mimeType string, size int64) (Job, error) {
	data, err := os.ReadFile(storedPath)
	if err != nil {
		return Job{}, fmt.Errorf("读取上传文件失败: %w", err)
	}
	if !utf8.Valid(data) {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "JSON 文件不是有效 UTF-8"})
	}
	var payload jsonPayload
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&payload); err != nil {
		return Job{}, httpapi.ValidationError(map[string]string{"file": "JSON 结构不符合导入格式: " + err.Error()})
	}
	if len(payload.Items) == 0 || len(payload.Items) > 500 {
		return Job{}, httpapi.ValidationError(map[string]string{"items": "题目数量必须是 1 到 500"})
	}
	items := make([]Item, 0, len(payload.Items))
	for i, raw := range payload.Items {
		item, err := s.convertJSONItem(ctx, raw)
		if err != nil {
			// 内容问题一律按 400 返回并带题号；数据库等内部错误保持 500。
			var apiErr *httpapi.APIError
			if errors.As(err, &apiErr) {
				prefixed := *apiErr
				prefixed.Message = fmt.Sprintf("第 %d 题：%s", i+1, apiErr.Message)
				return Job{}, &prefixed
			}
			return Job{}, fmt.Errorf("第 %d 题：%w", i+1, err)
		}
		item.Position = i + 1
		items = append(items, item)
	}
	var jobID string
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		jobID, err = s.store.InsertJob(ctx, tx, adminID, name, storedPath, digest, mimeType, size)
		if err != nil {
			return err
		}
		return s.store.InsertItemsAndReady(ctx, tx, jobID, items)
	})
	if err != nil {
		return Job{}, fmt.Errorf("创建导入任务失败: %w", err)
	}
	s.logger.Info("import_json_done", "job_id", jobID, "items", len(items))
	return s.store.JobByID(ctx, jobID)
}

func (s *Service) convertJSONItem(ctx context.Context, raw jsonItem) (Item, error) {
	levelID, err := s.resolveCode(ctx, `SELECT id::text FROM exam_levels WHERE code = $1`, raw.LevelCode)
	if err != nil {
		return Item{}, httpapi.ValidationError(map[string]string{"levelCode": "级别代码不存在: " + raw.LevelCode})
	}
	subjectID, err := s.resolveCode(ctx, `SELECT id::text FROM subjects WHERE code = $1`, raw.SubjectCode)
	if err != nil {
		return Item{}, httpapi.ValidationError(map[string]string{"subjectCode": "科目代码不存在: " + raw.SubjectCode})
	}
	kpIDs, kpAnomalies, err := s.resolveKnowledgePointNames(ctx, levelID, raw.KnowledgePointNames)
	if err != nil {
		return Item{}, err
	}
	draft, normalizedAnomalies, err := buildAIDraft(raw, levelID, subjectID, kpIDs, kpAnomalies).toDraft()
	if err != nil {
		return Item{}, httpapi.E(http.StatusBadRequest, "invalid_import_item", err.Error())
	}
	if err := validateDraft(&draft); err != nil {
		return Item{}, err
	}
	if err := s.validateReferences(ctx, draft); err != nil {
		return Item{}, err
	}
	rawExcerpt := strings.TrimSpace(raw.RawExcerpt)
	if rawExcerpt == "" {
		rawExcerpt = truncateRunes(draft.Stem, 500)
	}
	if len([]rune(rawExcerpt)) > 5000 {
		return Item{}, httpapi.E(http.StatusBadRequest, "invalid_import_item", "原文定位片段过长")
	}
	anomalies := raw.Anomalies
	if anomalies == nil {
		anomalies = []string{}
	}
	anomalies = append(anomalies, kpAnomalies...)
	anomalies = append(anomalies, normalizedAnomalies...)
	return Item{RawExcerpt: rawExcerpt, Draft: &draft, Anomalies: anomalies}, nil
}

func (s *Service) resolveCode(ctx context.Context, query, code string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, query, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	return id, err
}

func (s *Service) resolveKnowledgePointNames(ctx context.Context, levelID string, names []string) ([]string, []string, error) {
	ids := []string{}
	anomalies := []string{}
	seen := map[string]bool{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		matches, err := store.CollectRows[struct{ ID string }](ctx, s.pool,
			`SELECT id::text FROM knowledge_points WHERE name = $1 AND level_id = $2 AND status <> 'retired'`,
			name, levelID)
		if err != nil {
			return nil, nil, err
		}
		if len(matches) != 1 {
			anomalies = append(anomalies, "知识点未唯一匹配，已跳过: "+name)
			continue
		}
		if !seen[matches[0].ID] {
			seen[matches[0].ID] = true
			ids = append(ids, matches[0].ID)
		}
	}
	return ids, anomalies, nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) > max {
		runes = runes[:max]
	}
	return string(runes)
}

// aiDraft 是结构化 JSON 解析后的中间形态；toDraft 负责把来源答案规范化为
// 正式草稿，非标准答案对象只降级为 AI 建议答案，绝不冒充权威答案。
type aiDraft struct {
	RawExcerpt        string            `json:"rawExcerpt"`
	MaterialKey       string            `json:"materialKey"`
	Type              string            `json:"type"`
	Stem              string            `json:"stem"`
	Options           []content.Option  `json:"options"`
	MaterialTitle     string            `json:"materialTitle"`
	MaterialContent   string            `json:"materialContent"`
	LevelID           string            `json:"levelId"`
	SubjectID         string            `json:"subjectId"`
	SourceSectionID   *string           `json:"sourceSectionId"`
	Difficulty        int               `json:"difficulty"`
	KnowledgePointIDs []string          `json:"knowledgePointIds"`
	SourceAnswer      json.RawMessage   `json:"sourceAnswer"`
	AISuggestedAnswer *AnswerSuggestion `json:"aiSuggestedAnswer"`
	Anomalies         []string          `json:"anomalies"`
}

func (d aiDraft) toDraft() (Draft, []string, error) {
	draft := Draft{MaterialKey: d.MaterialKey, Type: d.Type, Stem: d.Stem, Options: d.Options,
		MaterialTitle: d.MaterialTitle, MaterialContent: d.MaterialContent, LevelID: d.LevelID,
		SubjectID: d.SubjectID, SourceSectionID: d.SourceSectionID, Difficulty: d.Difficulty,
		KnowledgePointIDs: d.KnowledgePointIDs,
		AISuggestedAnswer: d.AISuggestedAnswer}
	anomalies := []string{}
	if len(d.SourceAnswer) == 0 || string(d.SourceAnswer) == "null" {
		return draft, anomalies, nil
	}
	var sourceAnswer content.AnswerInput
	if err := strictDecode(d.SourceAnswer, &sourceAnswer); err == nil {
		draft.Answer = &sourceAnswer
		draft.SourceAnswer = &sourceAnswer
		return draft, anomalies, nil
	}
	var scalar string
	if err := json.Unmarshal(d.SourceAnswer, &scalar); err != nil || scalar == "" {
		return Draft{}, nil, errors.New("sourceAnswer 必须是标准答案对象")
	}
	for index, option := range d.Options {
		if strings.EqualFold(scalar, option.ID) || strings.EqualFold(scalar, option.Label) || scalar == strconv.Itoa(index+1) {
			value, _ := json.Marshal(map[string]any{"optionIds": []string{option.ID}})
			draft.AISuggestedAnswer = &AnswerSuggestion{Value: value, Explanation: "AI 从原文识别到候选答案，请人工确认。"}
			anomalies = append(anomalies, "AI 返回了非标准答案对象，已降级为 AI 建议答案，请人工确认。")
			return draft, anomalies, nil
		}
	}
	if d.Type == "fill_blank" || d.Type == "short_answer" {
		value, _ := json.Marshal(map[string]string{"text": scalar})
		draft.AISuggestedAnswer = &AnswerSuggestion{Value: value, Explanation: "AI 从原文识别到候选答案，请人工确认。"}
		anomalies = append(anomalies, "AI 返回了非标准答案对象，已降级为 AI 建议答案，请人工确认。")
		return draft, anomalies, nil
	}
	return Draft{}, nil, errors.New("sourceAnswer 未匹配到选项，已拒绝自动转换")
}

func validateDraft(d *Draft) error {
	if d.Difficulty == 0 {
		d.Difficulty = 3
	}
	if d.Difficulty < 1 || d.Difficulty > 5 {
		return httpapi.ValidationError(map[string]string{"difficulty": "难度必须是 1 到 5"})
	}
	if d.Options == nil {
		d.Options = []content.Option{}
	}
	if d.KnowledgePointIDs == nil {
		d.KnowledgePointIDs = []string{}
	}
	if fields := content.ValidateInput(content.QuestionInput{
		Type: d.Type, Stem: d.Stem, Options: d.Options, LevelID: d.LevelID, SubjectID: d.SubjectID,
		SourceSectionID: d.SourceSectionID, Difficulty: d.Difficulty, KnowledgePointIDs: d.KnowledgePointIDs,
		Answer: d.Answer,
	}); len(fields) > 0 {
		return httpapi.ValidationError(fields)
	}
	if d.SourceAnswer != nil {
		if d.SourceAnswer.Authority != content.AuthorityOfficial {
			return httpapi.ValidationError(map[string]string{"sourceAnswer": "来源答案必须标记为 official"})
		}
		if err := content.ValidateAnswerValue(d.Type, d.Options, d.SourceAnswer.Value); err != nil {
			return httpapi.ValidationError(map[string]string{"sourceAnswer": err.Error()})
		}
	}
	if d.AISuggestedAnswer != nil {
		if err := content.ValidateAnswerValue(d.Type, d.Options, d.AISuggestedAnswer.Value); err != nil {
			return httpapi.ValidationError(map[string]string{"aiSuggestedAnswer": err.Error()})
		}
	}
	return nil
}

func (s *Service) validateReferences(ctx context.Context, d Draft) error {
	var sameScope bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM exam_levels l JOIN subjects sub ON sub.exam_id = l.exam_id WHERE l.id = $1 AND sub.id = $2)`,
		d.LevelID, d.SubjectID).Scan(&sameScope); err != nil {
		return err
	}
	if !sameScope {
		return httpapi.ValidationError(map[string]string{"levelId": "级别与科目不存在或不属于同一考试"})
	}
	var kpCount int
	if err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM knowledge_points WHERE id = ANY($1::uuid[])`, d.KnowledgePointIDs).Scan(&kpCount); err != nil {
		return err
	}
	if kpCount != len(d.KnowledgePointIDs) {
		return httpapi.ValidationError(map[string]string{"knowledgePointIds": "包含不存在的知识点"})
	}
	if d.SourceSectionID != nil && *d.SourceSectionID != "" {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM source_sections WHERE id = $1)`, *d.SourceSectionID).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return httpapi.ValidationError(map[string]string{"sourceSectionId": "来源章节不存在"})
		}
	}
	return nil
}

func strictDecode(data []byte, dst any) error {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("JSON 不合法: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return errors.New("包含多余 JSON 内容")
	}
	return nil
}
