package ai

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/aishuati/backend/internal/learning"
)

//go:embed testdata/eval_cases.json
var offlineEvalCases []byte

type OfflineEvalReport struct {
	Cases            int            `json:"cases"`
	Passed           int            `json:"passed"`
	FormatErrors     int            `json:"formatErrors"`
	SemanticErrors   int            `json:"semanticErrors"`
	FirstPassRate    float64        `json:"firstPassRate"`
	TotalCalls       int            `json:"totalCalls"`
	PromptTokens     int            `json:"promptTokens"`
	CompletionTokens int            `json:"completionTokens"`
	TotalTokens      int            `json:"totalTokens"`
	EstimatedCostUSD *float64       `json:"estimatedCostUsd"`
	Retries          int            `json:"retries"`
	DuplicateRate    float64        `json:"duplicateRate"`
	PromptVersions   map[string]int `json:"promptVersions"`
}

type offlineEvalCase struct {
	ID               string          `json:"id"`
	Kind             string          `json:"kind"`
	Level            string          `json:"level"`
	Source           string          `json:"source"`
	Input            string          `json:"input"`
	Type             string          `json:"type"`
	Options          json.RawMessage `json:"options"`
	Correctness      string          `json:"correctness"`
	Answer           json.RawMessage `json:"answer"`
	Raw              string          `json:"raw"`
	Question         json.RawMessage `json:"question"`
	QuestionType     string          `json:"questionType"`
	DifficultyMode   string          `json:"difficultyMode"`
	PromptTokens     int             `json:"promptTokens"`
	CompletionTokens int             `json:"completionTokens"`
	Expected         string          `json:"expected"`
}

// RunOfflineEval 验证固定回放样本；默认不创建 HTTP 客户端，也不调用外部模型。
func RunOfflineEval() (OfflineEvalReport, error) {
	return RunOfflineEvalWithLimit(0, 0)
}

// RunOfflineEvalWithLimit 允许评测命令限制样本数和总回放调用数；仍不联网。
func RunOfflineEvalWithLimit(limit, maxCalls int) (OfflineEvalReport, error) {
	var cases []offlineEvalCase
	if err := json.Unmarshal(offlineEvalCases, &cases); err != nil {
		return OfflineEvalReport{}, fmt.Errorf("读取 AI 离线评测样本失败: %w", err)
	}
	count := len(cases)
	if limit > 0 && limit < count {
		count = limit
	}
	if maxCalls > 0 && maxCalls < count {
		count = maxCalls
	}
	report := OfflineEvalReport{Cases: count, PromptVersions: map[string]int{}}
	seenIDs := map[string]struct{}{}
	for _, c := range cases[:count] {
		report.TotalCalls++
		report.PromptTokens += c.PromptTokens
		report.CompletionTokens += c.CompletionTokens
		report.PromptVersions["offline.fixture.v1"]++
		if _, exists := seenIDs[c.ID]; exists {
			report.DuplicateRate = float64(len(seenIDs)+1) / float64(report.Cases)
		}
		seenIDs[c.ID] = struct{}{}
		valid, kind, err := validateOfflineCase(c)
		wantValid := c.Expected == "valid"
		if valid == wantValid {
			report.Passed++
			continue
		}
		if kind == "format" {
			report.FormatErrors++
		} else {
			report.SemanticErrors++
		}
		if err == nil {
			err = fmt.Errorf("期望 %s，但校验结果相反", c.Expected)
		}
		return report, fmt.Errorf("离线评测样本 %s 失败: %w", c.ID, err)
	}
	if report.Cases > 0 {
		report.FirstPassRate = float64(report.Passed) / float64(report.Cases)
	}
	report.TotalTokens = report.PromptTokens + report.CompletionTokens
	return report, nil
}

func validateOfflineCase(c offlineEvalCase) (bool, string, error) {
	switch c.Kind {
	case "json":
		if json.Valid([]byte(c.Raw)) {
			return true, "format", nil
		}
		return false, "format", fmt.Errorf("响应不是合法 JSON")
	case "grade":
		var options []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
			Text  string `json:"text"`
		}
		if err := json.Unmarshal(c.Options, &options); err != nil {
			return false, "format", err
		}
		optionsJSON, _ := json.Marshal(options)
		err := validateAICorrectAnswer(c.Type, evalStringPtr(string(optionsJSON)), c.Answer, c.Correctness)
		if err != nil {
			return false, "semantic", err
		}
		return true, "semantic", nil
	case "generation":
		var q generatedQuestion
		if err := json.Unmarshal(c.Question, &q); err != nil {
			return false, "format", err
		}
		points := []learning.AIGenerationKnowledgePoint{{ID: "kp-1"}}
		err := validateGeneratedQuestions([]generatedQuestion{q}, 1, c.DifficultyMode, c.QuestionType, points)
		if err != nil {
			return false, "semantic", err
		}
		return true, "semantic", nil
	default:
		return false, "format", fmt.Errorf("未知评测样本类型 %q", c.Kind)
	}
}

func evalStringPtr(value string) *string { return &value }
