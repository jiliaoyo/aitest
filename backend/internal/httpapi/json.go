package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
)

const maxBodyBytes = 1 << 20

// APIError 是带稳定错误码的业务错误；前端只依赖 Code 分支。
type APIError struct {
	Status  int    `json:"-"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

func E(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

func WithDetails(e *APIError, details any) *APIError {
	e.Details = details
	return e
}

var (
	ErrUnauthorized  = E(http.StatusUnauthorized, "unauthorized", "请先登录")
	ErrForbidden     = E(http.StatusForbidden, "forbidden", "没有执行该操作的权限")
	ErrNotFound      = E(http.StatusNotFound, "not_found", "资源不存在或无权访问")
	ErrBadRequest    = E(http.StatusBadRequest, "bad_request", "请求格式不正确")
	ErrValidation    = E(http.StatusBadRequest, "validation_failed", "请检查表单填写")
	ErrRateLimited   = E(http.StatusTooManyRequests, "rate_limited", "尝试次数过多，请稍后再试")
	ErrInternal      = E(http.StatusInternalServerError, "internal_error", "服务器内部错误")
	ErrMethodNotAllowed = E(http.StatusMethodNotAllowed, "method_not_allowed", "不支持的请求方法")
)

// ValidationError 返回带字段级错误的 400。
func ValidationError(fields map[string]string) *APIError {
	e := *ErrValidation
	return WithDetails(&e, map[string]any{"fields": fields})
}

// WriteError 把任意错误映射为统一错误响应；非 APIError 一律按内部错误处理，不外泄细节。
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		apiErr = ErrInternal
		slog.Error("internal_error",
			"error", err,
			"path", r.URL.Path,
			"request_id", RequestID(r))
	}
	body := struct {
		Error APIError `json:"error"`
	}{Error: *apiErr}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(apiErr.Status)
	_ = json.NewEncoder(w).Encode(body)
}

// WriteJSON 编码成功响应。
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

// DecodeJSON 严格解码 JSON 请求体：限制大小、要求正确 Content-Type、拒绝未知字段。
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if r.Method != http.MethodGet && r.Header.Get("Content-Type") != "application/json" {
		return E(http.StatusUnsupportedMediaType, "unsupported_media_type", "请求需要 application/json")
	}
	raw, err := ReadBody(w, r)
	if err != nil {
		return err
	}
	return DecodeRaw(raw, dst)
}

// ReadBody 读取请求体（限制大小）；提交接口需要原始字节计算幂等哈希。
func ReadBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, E(http.StatusRequestEntityTooLarge, "payload_too_large", "请求体过大")
		}
		return nil, ErrBadRequest
	}
	return raw, nil
}

// DecodeRaw 对原始字节执行严格 JSON 解码。
func DecodeRaw(raw []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return E(http.StatusBadRequest, "bad_request", "请求体不是合法的 JSON 或包含未知字段")
	}
	if dec.More() {
		return ErrBadRequest
	}
	return nil
}

// RequestID 返回当前请求的追踪 ID。
func RequestID(r *http.Request) string {
	if id, ok := r.Context().Value(ctxkeys.RequestID).(string); ok {
		return id
	}
	return ""
}
