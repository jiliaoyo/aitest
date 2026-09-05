// Package ai 封装对 AI 服务的 HTTP 调用：不绑定厂商 SDK，输出严格 JSON 校验并留审计。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	BaseURL               string
	APIKey                string
	Model                 string
	Timeout               time.Duration
	InputPricePerMillion  float64
	OutputPricePerMillion float64
}

type Client struct {
	cfg    Config
	http   *http.Client
	pool   *pgxpool.Pool
	logger *slog.Logger
}

const generatedPracticeMaxTokens = 16384

func NewClient(cfg Config, pool *pgxpool.Pool, logger *slog.Logger) *Client {
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: cfg.Timeout},
		pool:   pool,
		logger: logger,
	}
}

// RunPrompt 记录一次 ai_runs 审计并返回模型原始 JSON 输出。
func (c *Client) RunPrompt(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string) (json.RawMessage, error) {
	return c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, 0, false)
}

// RunPromptWithTemperature 用于需要随机性的内容生成；判分与统计类任务继续使用温度 0。
func (c *Client) RunPromptWithTemperature(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string, temperature float64) (json.RawMessage, error) {
	return c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, temperature, true)
}

func (c *Client) Configured() bool {
	return c.cfg.BaseURL != "" && c.cfg.APIKey != "" && c.cfg.Model != ""
}

func (c *Client) runPrompt(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string, temperature float64, disableThinking bool) (json.RawMessage, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" || c.cfg.Model == "" {
		return nil, errNotConfigured
	}
	start := time.Now()
	reqBody := map[string]any{
		"model": c.cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPayload},
		},
		"temperature":     temperature,
		"response_format": map[string]string{"type": "json_object"},
	}
	if disableThinking {
		reqBody["thinking"] = map[string]string{"type": "disabled"}
		reqBody["max_tokens"] = generatedPracticeMaxTokens
	}
	data, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		trimRight(c.cfg.BaseURL)+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, err)
		return nil, fmt.Errorf("AI 服务请求失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, err)
		return nil, fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		err := fmt.Errorf("AI 服务返回 %d", resp.StatusCode)
		c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, err)
		return nil, err
	}
	var chat struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &chat); err != nil || len(chat.Choices) == 0 {
		err := fmt.Errorf("AI 响应结构不合法")
		c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, err)
		return nil, err
	}
	content := chat.Choices[0].Message.Content
	// 模型可能用代码块包裹 JSON，剥离围栏后再交给严格解码
	content = stripFences(content)
	var out json.RawMessage
	if json.Unmarshal([]byte(content), &out) != nil {
		err := fmt.Errorf("AI 输出不是合法 JSON")
		c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, chat.Usage.PromptTokens, chat.Usage.CompletionTokens, err)
		return nil, err
	}
	c.audit(ctx, userID, kind, promptVersion, inputRef, start, out, chat.Usage.PromptTokens, chat.Usage.CompletionTokens, nil)
	return out, nil
}

func (c *Client) audit(ctx context.Context, userID, kind, promptVersion, inputRef string, start time.Time, output any, promptTokens, completionTokens int, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
		if len(errMsg) > 300 {
			errMsg = errMsg[:300]
		}
	}
	var outputArg any
	if output != nil {
		outputArg = output
	}
	var userIDArg any
	if userID != "" {
		userIDArg = userID
	}
	var costArg any
	if (c.cfg.InputPricePerMillion > 0 || c.cfg.OutputPricePerMillion > 0) && (promptTokens > 0 || completionTokens > 0) {
		costArg = (float64(promptTokens)*c.cfg.InputPricePerMillion + float64(completionTokens)*c.cfg.OutputPricePerMillion) / 1_000_000
	}
	_, e := c.pool.Exec(ctx,
		`INSERT INTO ai_runs (user_id, kind, prompt_version, model, input_ref, output, prompt_tokens, completion_tokens, duration_ms, error, estimated_cost_usd)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		userIDArg, kind, promptVersion, c.cfg.Model, inputRef, outputArg, promptTokens, completionTokens,
		time.Since(start).Milliseconds(), errMsg, costArg)
	if e != nil {
		c.logger.Error("ai_audit_failed", "error", e)
	}
}

func trimRight(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '/') {
		s = s[:len(s)-1]
	}
	return s
}

func stripFences(s string) string {
	start := 0
	end := len(s)
	for i := 0; i+3 <= len(s); i++ {
		if s[i:i+3] == "```" {
			start = i + 3
			// 跳过语言标记行
			for start < len(s) && s[start] != '\n' {
				start++
			}
			break
		}
	}
	for i := end - 3; i >= start; i-- {
		if s[i:i+3] == "```" {
			end = i
			break
		}
	}
	return s[start:end]
}

type notConfiguredError struct{}

func (notConfiguredError) Error() string { return "AI 服务未配置" }

var errNotConfigured = notConfiguredError{}
