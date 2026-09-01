package content

import "encoding/json"

const (
	StatusDraft     = "draft"
	StatusInReview  = "in_review"
	StatusPublished = "published"
	StatusRetired   = "retired"

	AuthorityOfficial       = "official"
	AuthorityHumanVerified  = "human_verified"
)

type Source struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Author       string          `json:"author"`
	Publisher    string          `json:"publisher"`
	Year         *int            `json:"year"`
	LicenseNote  string          `json:"licenseNote"`
	InternalNote string          `json:"internalNote"`
	Sections     []SourceSection `json:"sections"`
}

type SourceSection struct {
	ID        string `json:"id"`
	SourceID  string `json:"sourceId"`
	Name      string `json:"name"`
	SortOrder int    `json:"-"`
}

type Option struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Text  string `json:"text"`
}

type AnswerKey struct {
	Value       json.RawMessage `json:"value"`
	Authority   string          `json:"authority"`
	Explanation string          `json:"explanation"`
}

// QuestionVersion 是题目某一不可变版本的完整内容（仅管理端可携带 AnswerKey）。
type QuestionVersion struct {
	ID               string          `json:"id"`
	QuestionID       string          `json:"questionId"`
	VersionNo        int             `json:"versionNo"`
	Type             string          `json:"type"`
	Stem             string          `json:"stem"`
	MaterialVersionID *string       `json:"-"`
	MaterialTitle    string          `json:"materialTitle,omitempty"`
	MaterialContent  string          `json:"materialContent,omitempty"`
	Options          json.RawMessage `json:"options"`
	LevelID          string          `json:"levelId"`
	SubjectID        string          `json:"subjectId"`
	SourceSectionID  *string         `json:"sourceSectionId"`
	Difficulty       int             `json:"difficulty"`
	KnowledgePointIDs []string       `json:"knowledgePointIds"`
	AnswerKey        *AnswerKey      `json:"answerKey,omitempty"`
	CreatedAt        string          `json:"createdAt"`
}

// QuestionAdmin 是管理端题目列表/详情聚合。
type QuestionAdmin struct {
	ID                 string          `json:"id"`
	Status             string          `json:"status"`
	HasAnswer          bool            `json:"hasAnswer"`
	PublishedVersionID *string         `json:"publishedVersionId"`
	CurrentVersion     *QuestionVersion `json:"currentVersion"`
	PublishedAt        *string         `json:"publishedAt"`
	RetiredAt          *string         `json:"retiredAt"`
	UpdatedAt          string          `json:"updatedAt"`
}

// QuestionInput 是管理端创建/编辑题目的请求载荷；编辑总是产生新版本。
type QuestionInput struct {
	Type              string         `json:"type"`
	Stem              string         `json:"stem"`
	Options           []Option       `json:"options"`
	MaterialID        *string        `json:"materialId"`
	MaterialTitle     string         `json:"materialTitle"`
	MaterialContent   string         `json:"materialContent"`
	LevelID           string         `json:"levelId"`
	SubjectID         string         `json:"subjectId"`
	SourceSectionID   *string        `json:"sourceSectionId"`
	Difficulty        int            `json:"difficulty"`
	KnowledgePointIDs []string       `json:"knowledgePointIds"`
	Answer            *AnswerInput   `json:"answer"`
}

type AnswerInput struct {
	Value       json.RawMessage `json:"value"`
	Authority   string          `json:"authority"`
	Explanation string          `json:"explanation"`
}

var validTypes = map[string]bool{
	"single_choice": true, "multiple_choice": true, "fill_blank": true, "short_answer": true,
}

var validAuthorities = map[string]bool{AuthorityOfficial: true, AuthorityHumanVerified: true}

func ValidType(t string) bool { return validTypes[t] }

func IsChoiceType(t string) bool { return t == "single_choice" || t == "multiple_choice" }

// ValidateInput 校验题目输入；返回字段错误映射，空 map 表示通过。
func ValidateInput(in QuestionInput) map[string]string {
	fields := map[string]string{}
	if !validTypes[in.Type] {
		fields["type"] = "题型不合法"
	}
	if len([]rune(in.Stem)) < 2 {
		fields["stem"] = "请填写题干"
	}
	if IsChoiceType(in.Type) {
		if len(in.Options) < 2 {
			fields["options"] = "选择题至少需要两个选项"
		}
		seen := map[string]bool{}
		for i, o := range in.Options {
			if o.ID == "" || seen[o.ID] {
				fields["options"] = "选项 ID 缺失或重复"
			}
			seen[o.ID] = true
			if o.Text == "" {
				fields["options"] = "选项内容不能为空"
			}
			if o.Label == "" {
				in.Options[i].Label = string(rune('A' + i))
			}
		}
	}
	if in.LevelID == "" {
		fields["levelId"] = "请选择级别"
	}
	if in.SubjectID == "" {
		fields["subjectId"] = "请选择科目"
	}
	if in.Difficulty < 1 || in.Difficulty > 5 {
		in.Difficulty = 3
	}
	if in.Answer != nil {
		if !validAuthorities[in.Answer.Authority] {
			fields["answer"] = "答案来源必须是 official 或 human_verified"
		} else if err := ValidateAnswerValue(in.Type, in.Options, in.Answer.Value); err != nil {
			fields["answer"] = err.Error()
		}
	}
	return fields
}

// ValidateAnswerValue 按题型校验标准答案结构。
func ValidateAnswerValue(qType string, options []Option, value json.RawMessage) error {
	if len(value) == 0 {
		return errAnswer("答案不能为空")
	}
	var v map[string]any
	if err := json.Unmarshal(value, &v); err != nil {
		return errAnswer("答案不是合法 JSON")
	}
	switch qType {
	case "single_choice", "multiple_choice":
		raw, ok := v["optionIds"]
		if !ok {
			return errAnswer("选择题答案必须包含 optionIds")
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			return errAnswer("optionIds 必须是非空数组")
		}
		valid := map[string]bool{}
		for _, o := range options {
			valid[o.ID] = true
		}
		seen := map[string]bool{}
		for _, idAny := range list {
			id, ok := idAny.(string)
			if !ok || !valid[id] || seen[id] {
				return errAnswer("答案引用了不存在或重复的选项")
			}
			seen[id] = true
		}
		if qType == "single_choice" && len(list) != 1 {
			return errAnswer("单选题答案只能有一个选项")
		}
	case "fill_blank":
		raw, ok := v["acceptable"]
		if !ok {
			return errAnswer("填空题答案必须包含 acceptable 可接受值数组")
		}
		list, ok := raw.([]any)
		if !ok || len(list) == 0 {
			return errAnswer("acceptable 必须是非空数组")
		}
		for _, a := range list {
			s, ok := a.(string)
			if !ok || s == "" {
				return errAnswer("可接受答案必须是非空字符串")
			}
		}
	case "short_answer":
		if ref, ok := v["reference"]; ok {
			if s, isStr := ref.(string); !isStr || s == "" {
				return errAnswer("reference 必须是非空字符串")
			}
		}
	}
	return nil
}

type answerError struct{ msg string }

func (e answerError) Error() string { return e.msg }

func errAnswer(msg string) error { return answerError{msg: msg} }
