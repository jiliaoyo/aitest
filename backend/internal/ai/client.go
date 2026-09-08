// Package ai 封装对 AI 服务的 HTTP 调用：不绑定厂商 SDK，输出严格 JSON 校验并留审计。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const (
	maxAIResponseBytes = 1 << 20
	businessPending    = "pending"
	businessSucceeded  = "succeeded"
	businessFailed     = "failed"
	businessNotApplied = "not_applicable"
	failureTransport   = "transport"
	failureHTTP        = "http"
	failureRateLimit   = "rate_limit"
	failureStructure   = "response_structure"
	failureJSON        = "invalid_json"
	failureTruncated   = "truncated"
)

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
	out, _, err := c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, 0, false)
	return out, err
}

// RunPromptWithAudit 与 RunPrompt 相同，但返回可用于补写业务校验结果的 run ID。
func (c *Client) RunPromptWithAudit(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string) (json.RawMessage, string, error) {
	return c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, 0, false)
}

// RunPromptWithTemperature 用于需要随机性的内容生成；判分与统计类任务继续使用温度 0。
func (c *Client) RunPromptWithTemperature(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string, temperature float64) (json.RawMessage, error) {
	out, _, err := c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, temperature, true)
	return out, err
}

// RunPromptWithTemperatureAndAudit 返回随机出题调用的审计 run ID。
func (c *Client) RunPromptWithTemperatureAndAudit(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string, temperature float64) (json.RawMessage, string, error) {
	return c.runPrompt(ctx, userID, kind, promptVersion, inputRef, systemPrompt, userPayload, temperature, true)
}

func (c *Client) Configured() bool {
	return c.cfg.BaseURL != "" && c.cfg.APIKey != "" && c.cfg.Model != ""
}

func (c *Client) runPrompt(ctx context.Context, userID, kind, promptVersion, inputRef string, systemPrompt, userPayload string, temperature float64, disableThinking bool) (json.RawMessage, string, error) {
	if c.cfg.BaseURL == "" || c.cfg.APIKey == "" || c.cfg.Model == "" {
		return nil, "", errNotConfigured
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
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, nil, businessNotApplied, failureTransport, err)
		return nil, runID, fmt.Errorf("AI 服务请求失败: %w", err)
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAIResponseBytes+1))
	if err != nil {
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, &statusCode, businessNotApplied, failureTransport, err)
		return nil, runID, fmt.Errorf("读取 AI 响应失败: %w", err)
	}
	if len(body) > maxAIResponseBytes {
		err := errors.New("AI 响应超过大小限制")
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, &statusCode, businessNotApplied, failureTruncated, err)
		return nil, runID, err
	}
	if resp.StatusCode >= 400 {
		err := &httpResponseError{status: resp.StatusCode}
		failureKind := failureHTTP
		if resp.StatusCode == http.StatusTooManyRequests {
			failureKind = failureRateLimit
		}
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, &statusCode, businessNotApplied, failureKind, err)
		return nil, runID, err
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
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, 0, 0, &statusCode, businessNotApplied, failureStructure, err)
		return nil, runID, err
	}
	content := chat.Choices[0].Message.Content
	// 模型可能用代码块包裹 JSON，剥离围栏后再交给严格解码
	content = stripFences(content)
	var out json.RawMessage
	if json.Unmarshal([]byte(content), &out) != nil {
		err := fmt.Errorf("AI 输出不是合法 JSON")
		runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, nil, chat.Usage.PromptTokens, chat.Usage.CompletionTokens, &statusCode, businessNotApplied, failureJSON, err)
		return nil, runID, err
	}
	runID := c.audit(ctx, userID, kind, promptVersion, inputRef, start, out, chat.Usage.PromptTokens, chat.Usage.CompletionTokens, &statusCode, businessPending, "", nil)
	return out, runID, nil
}

func (c *Client) audit(ctx context.Context, userID, kind, promptVersion, inputRef string, start time.Time, output any, promptTokens, completionTokens int, httpStatus *int, businessStatus, failureKind string, err error) string {
	if c.pool == nil {
		return ""
	}
	errMsg := ""
	if err != nil {
		errMsg = trimError(err)
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
	auditCtx := ctx
	cancel := func() {}
	if ctx.Err() != nil {
		auditCtx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
	}
	defer cancel()
	var runID string
	e := c.pool.QueryRow(auditCtx,
		`INSERT INTO ai_runs (user_id, kind, prompt_version, model, input_ref, output, prompt_tokens, completion_tokens, duration_ms, error, estimated_cost_usd, http_status, business_status, failure_kind)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id::text`,
		userIDArg, kind, promptVersion, c.cfg.Model, inputRef, outputArg, promptTokens, completionTokens,
		time.Since(start).Milliseconds(), errMsg, costArg, httpStatus, businessStatus, failureKind).Scan(&runID)
	if e != nil && c.logger != nil {
		c.logger.Error("ai_audit_failed", "error", e)
	}
	return runID
}

// MarkBusinessSuccess/Failure 在模型返回合法 JSON 后补写业务校验结果。
func (c *Client) MarkBusinessSuccess(ctx context.Context, runID string) error {
	return c.markBusinessResult(ctx, runID, businessSucceeded, "", nil)
}

func (c *Client) MarkBusinessFailure(ctx context.Context, runID, failureKind string, cause error) error {
	return c.markBusinessResult(ctx, runID, businessFailed, failureKind, cause)
}

func (c *Client) markBusinessResult(ctx context.Context, runID, status, failureKind string, cause error) error {
	if runID == "" || c.pool == nil {
		return nil
	}
	errMsg := ""
	if cause != nil {
		errMsg = trimError(cause)
	}
	_, err := c.pool.Exec(ctx,
		`UPDATE ai_runs
		 SET business_status = $2, failure_kind = $3,
		     error = CASE WHEN $4 = '' THEN error ELSE $4 END
		 WHERE id = $1 AND business_status = $5`, runID, status, failureKind, errMsg, businessPending)
	return err
}

func trimError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > 300 {
		msg = msg[:300]
	}
	return msg
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

type httpResponseError struct{ status int }

func (e *httpResponseError) Error() string { return fmt.Sprintf("AI 服务返回 %d", e.status) }

func (e *httpResponseError) StatusCode() int { return e.status }

var errNotConfigured = notConfiguredError{}
