package auth

import (
	"net/http"
	"net/netip"
	"strings"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/httpapi/ctxkeys"
)

const (
	accessCookie  = "access_token"
	refreshCookie = "refresh_token"
	legacyCookie  = "session"
)

type Handler struct {
	service           *Service
	appEnv            string
	secure            bool
	trustedProxyCIDRs []netip.Prefix
}

func NewHandler(service *Service, appEnv string, secureCookie bool, trustedProxyCIDRs []netip.Prefix) *Handler {
	return &Handler{service: service, appEnv: appEnv, secure: secureCookie, trustedProxyCIDRs: trustedProxyCIDRs}
}

// RegisterRoutes 挂载认证路由。mux 的路径已包含 /api/v1 前缀。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/register", h.register)
	mux.HandleFunc("POST /api/v1/auth/login", h.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.logout)
	mux.HandleFunc("POST /api/v1/auth/change-password", h.changePassword)
	mux.HandleFunc("POST /api/v1/auth/password-reset/request", h.requestReset)
	mux.HandleFunc("POST /api/v1/auth/password-reset/confirm", h.confirmReset)
	mux.HandleFunc("GET /api/v1/me", h.me)
	mux.HandleFunc("PATCH /api/v1/me", h.updateMe)
}

func (h *Handler) setAuthCookies(w http.ResponseWriter, tokens Tokens) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessCookie,
		Value:    tokens.AccessToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.service.accessTokenTTL.Seconds()),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    tokens.RefreshToken,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(h.service.refreshTokenTTL.Seconds()),
	})
	// 兼容旧版 session Cookie；新登录后立即清除，避免两套令牌并存。
	h.clearCookie(w, legacyCookie, "/")
}

func (h *Handler) clearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: path, HttpOnly: true,
		Secure: h.secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
}

func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	h.clearCookie(w, accessCookie, "/")
	h.clearCookie(w, refreshCookie, "/api/v1/auth")
	h.clearCookie(w, legacyCookie, "/")
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	u, tokens, err := h.service.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.setAuthCookies(w, tokens)
	httpapi.WriteJSON(w, http.StatusCreated, meResponse{u})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	u, tokens, err := h.service.Login(r.Context(), req.Email, req.Password, clientIP(r, h.trustedProxyCIDRs))
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.setAuthCookies(w, tokens)
	httpapi.WriteJSON(w, http.StatusOK, meResponse{u})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	u, tokens, err := h.service.Refresh(r.Context(), RefreshToken(r))
	if err != nil {
		h.clearAuthCookies(w)
		httpapi.WriteError(w, r, httpapi.ErrUnauthorized)
		return
	}
	h.setAuthCookies(w, tokens)
	httpapi.WriteJSON(w, http.StatusOK, meResponse{u})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.service.LogoutTokens(r.Context(), Token(r), RefreshToken(r)); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	h.clearAuthCookies(w)
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.service.ChangePassword(r.Context(), ctxkeys.UserID(r.Context()), Token(r), req.CurrentPassword, req.NewPassword); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) requestReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	token, err := h.service.RequestPasswordReset(r.Context(), req.Email, h.appEnv)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	resp := map[string]any{"ok": true}
	if token != "" {
		// dev 环境没有邮件通道，直接返回令牌便于联调；prod 永不返回。
		resp["resetToken"] = token
	}
	httpapi.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) confirmReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.service.ConfirmPasswordReset(r.Context(), req.Token, req.Password); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type meResponse struct {
	User User `json:"user"`
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	userID := ctxkeys.UserID(r.Context())
	if userID == "" {
		httpapi.WriteError(w, r, httpapi.ErrUnauthorized)
		return
	}
	u, err := h.service.store.UserByID(r.Context(), userID)
	if err != nil {
		httpapi.WriteError(w, r, httpapi.ErrUnauthorized)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, meResponse{u})
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	userID := ctxkeys.UserID(r.Context())
	if userID == "" {
		httpapi.WriteError(w, r, httpapi.ErrUnauthorized)
		return
	}
	var req struct {
		DefaultLevelID *string `json:"defaultLevelId"`
		ShowFurigana   *bool   `json:"showFurigana"`
		FuriganaSize   *int    `json:"furiganaSize"`
	}
	if err := httpapi.DecodeJSON(w, r, &req); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	if err := h.service.UpdatePreferences(r.Context(), userID, req.DefaultLevelID, req.ShowFurigana, req.FuriganaSize); err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	u, err := h.service.store.UserByID(r.Context(), userID)
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, meResponse{u})
}

func clientIP(r *http.Request, trustedProxyCIDRs []netip.Prefix) string {
	remoteIP, ok := parseRemoteIP(r.RemoteAddr)
	if !ok {
		return ""
	}
	if !isTrustedProxy(remoteIP, trustedProxyCIDRs) {
		return remoteIP.String()
	}
	for _, candidate := range strings.Split(r.Header.Get("X-Forwarded-For"), ",") {
		if ip, err := netip.ParseAddr(strings.TrimSpace(candidate)); err == nil {
			return ip.String()
		}
	}
	return remoteIP.String()
}

func parseRemoteIP(remoteAddr string) (netip.Addr, bool) {
	if addr, err := netip.ParseAddrPort(remoteAddr); err == nil {
		return addr.Addr(), true
	}
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr, true
	}
	return netip.Addr{}, false
}

func isTrustedProxy(remoteIP netip.Addr, trustedProxyCIDRs []netip.Prefix) bool {
	for _, prefix := range trustedProxyCIDRs {
		if prefix.Contains(remoteIP) {
			return true
		}
	}
	return false
}

// Token 从请求中读取 session token，供鉴权中间件调用。
func Token(r *http.Request) string {
	for _, name := range []string{accessCookie, legacyCookie} {
		if c, err := r.Cookie(name); err == nil {
			return c.Value
		}
	}
	return ""
}

func RefreshToken(r *http.Request) string {
	c, err := r.Cookie(refreshCookie)
	if err != nil {
		return ""
	}
	return c.Value
}
