package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aishuati/backend/internal/httpapi"
	"github.com/aishuati/backend/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLen         = 8
	maxPasswordByte        = 72
	resetTokenTTL          = time.Hour
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 720 * time.Hour
	loginRateLimit         = 10
	resetRateLimit         = 5
	rateLimitWindow        = 15 * time.Minute
)

type Service struct {
	store           *Store
	pool            *pgxpool.Pool
	logger          *slog.Logger
	sessionTTL      time.Duration
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func NewService(store *Store, pool *pgxpool.Pool, logger *slog.Logger, sessionTTL time.Duration, tokenTTLs ...time.Duration) *Service {
	accessTTL, refreshTTL := defaultAccessTokenTTL, defaultRefreshTokenTTL
	if len(tokenTTLs) > 0 && tokenTTLs[0] > 0 {
		accessTTL = tokenTTLs[0]
	}
	if len(tokenTTLs) > 1 && tokenTTLs[1] > 0 {
		refreshTTL = tokenTTLs[1]
	} else if sessionTTL > 0 {
		refreshTTL = sessionTTL
	}
	return &Service{store: store, pool: pool, logger: logger, sessionTTL: sessionTTL, accessTokenTTL: accessTTL, refreshTokenTTL: refreshTTL}
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机令牌失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateSession 供鉴权中间件使用；无有效会话返回空用户 ID。
func (s *Service) ValidateSession(ctx context.Context, token string) (User, bool) {
	if token == "" {
		return User{}, false
	}
	u, err := s.store.SessionUser(ctx, HashToken(token))
	if err != nil {
		return User{}, false
	}
	return u, true
}

func (s *Service) Register(ctx context.Context, email, password string) (User, Tokens, error) {
	normalized := NormalizeEmail(email)
	fields := map[string]string{}
	if !ValidEmail(normalized) {
		fields["email"] = "请输入有效的邮箱地址"
	}
	if err := validatePassword(password); err != nil {
		fields["password"] = err.Message
	}
	if len(fields) > 0 {
		return User{}, Tokens{}, httpapi.ValidationError(fields)
	}
	if ok, err := s.store.HitRateLimit(ctx, "register:"+normalized, 5, rateLimitWindow); err != nil {
		return User{}, Tokens{}, fmt.Errorf("记录限流计数失败: %w", err)
	} else if !ok {
		return User{}, Tokens{}, httpapi.ErrRateLimited
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, Tokens{}, fmt.Errorf("密码哈希失败: %w", err)
	}
	u, err := s.store.CreateUser(ctx, email, normalized, string(hash), RoleLearner)
	if err != nil {
		if isUniqueViolation(err) {
			return User{}, Tokens{}, httpapi.ValidationError(map[string]string{"email": "该邮箱已注册"})
		}
		return User{}, Tokens{}, fmt.Errorf("创建用户失败: %w", err)
	}
	tokens, err := s.issueSession(ctx, u.ID)
	if err != nil {
		return User{}, Tokens{}, err
	}
	return u, tokens, nil
}

func (s *Service) Login(ctx context.Context, email, password, ip string) (User, Tokens, error) {
	normalized := NormalizeEmail(email)
	if !ValidEmail(normalized) || password == "" {
		return User{}, Tokens{}, httpapi.ValidationError(map[string]string{"email": "请输入邮箱和密码"})
	}
	for _, key := range []string{"login:ip:" + ip, "login:email:" + normalized} {
		ok, err := s.store.HitRateLimit(ctx, key, loginRateLimit, rateLimitWindow)
		if err != nil {
			return User{}, Tokens{}, fmt.Errorf("记录限流计数失败: %w", err)
		}
		if !ok {
			return User{}, Tokens{}, httpapi.ErrRateLimited
		}
	}
	id, hash, _, err := s.store.UserByEmail(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, Tokens{}, EInvalidCredentials()
	}
	if err != nil {
		return User{}, Tokens{}, fmt.Errorf("查询用户失败: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return User{}, Tokens{}, EInvalidCredentials()
	}
	u, err := s.store.UserByID(ctx, id)
	if err != nil {
		return User{}, Tokens{}, fmt.Errorf("读取用户失败: %w", err)
	}
	tokens, err := s.issueSession(ctx, u.ID)
	if err != nil {
		return User{}, Tokens{}, err
	}
	return u, tokens, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (User, Tokens, error) {
	if refreshToken == "" {
		return User{}, Tokens{}, httpapi.ErrUnauthorized
	}
	accessToken, err := NewToken()
	if err != nil {
		return User{}, Tokens{}, err
	}
	nextRefreshToken, err := NewToken()
	if err != nil {
		return User{}, Tokens{}, err
	}
	var u User
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		u, err = s.store.With(tx).RotateSessionTx(ctx, tx, HashToken(refreshToken), HashToken(accessToken), HashToken(nextRefreshToken), time.Now().Add(s.accessTokenTTL), time.Now().Add(s.refreshTokenTTL))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, Tokens{}, httpapi.ErrUnauthorized
	}
	if err != nil {
		return User{}, Tokens{}, fmt.Errorf("刷新登录令牌失败: %w", err)
	}
	return u, Tokens{AccessToken: accessToken, RefreshToken: nextRefreshToken}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.store.RevokeSession(ctx, HashToken(token))
}

func (s *Service) LogoutTokens(ctx context.Context, accessToken, refreshToken string) error {
	if err := s.Logout(ctx, accessToken); err != nil {
		return err
	}
	if refreshToken != "" {
		return s.store.RevokeRefreshSession(ctx, HashToken(refreshToken))
	}
	return nil
}

func validFuriganaSize(size int) bool { return size >= 50 && size <= 100 }

func (s *Service) UpdatePreferences(ctx context.Context, userID string, levelID *string, showFurigana *bool, furiganaSize *int) error {
	if furiganaSize != nil && !validFuriganaSize(*furiganaSize) {
		return httpapi.ValidationError(map[string]string{"furiganaSize": "假名字号必须在 50% 到 100% 之间"})
	}
	return s.store.UpdatePreferences(ctx, userID, levelID, showFurigana, furiganaSize)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentToken, currentPassword, newPassword string) error {
	if currentPassword == "" {
		return httpapi.ValidationError(map[string]string{"currentPassword": "请输入当前密码"})
	}
	if err := validatePassword(newPassword); err != nil {
		return httpapi.ValidationError(map[string]string{"newPassword": err.Message})
	}

	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var currentHash string
		if err := tx.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&currentHash); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpapi.ErrUnauthorized
			}
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(currentPassword)) != nil {
			return httpapi.E(http.StatusBadRequest, "invalid_current_password", "当前密码不正确")
		}
		newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("密码哈希失败: %w", err)
		}
		if err := s.store.UpdatePasswordTx(ctx, tx, userID, string(newHash)); err != nil {
			return err
		}
		if err := s.store.RevokeOtherUserSessionsTx(ctx, tx, userID, HashToken(currentToken)); err != nil {
			return err
		}
		return nil
	})
	return err
}

// RequestPasswordReset 创建找回令牌。首版没有邮件通道：dev 环境把令牌返回给调用方，
// prod 只记录日志并统一响应成功，不暴露令牌是否存在。
func (s *Service) RequestPasswordReset(ctx context.Context, email, appEnv string) (string, error) {
	normalized := NormalizeEmail(email)
	if ok, err := s.store.HitRateLimit(ctx, "reset:"+normalized, resetRateLimit, rateLimitWindow); err != nil {
		return "", fmt.Errorf("记录限流计数失败: %w", err)
	} else if !ok {
		return "", httpapi.ErrRateLimited
	}
	id, _, _, err := s.store.UserByEmail(ctx, normalized)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("查询用户失败: %w", err)
	}
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	if err := s.store.CreatePasswordResetToken(ctx, id, HashToken(token), resetTokenTTL); err != nil {
		return "", fmt.Errorf("创建找回令牌失败: %w", err)
	}
	if appEnv != "dev" {
		s.logger.Info("password_reset_requested", "user_id", id)
		return "", nil
	}
	return token, nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}
	var userID string
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st := s.store.With(tx)
		userID, err = st.ConsumePasswordResetTokenTx(ctx, tx, HashToken(token))
		if err != nil {
			return err
		}
		if err := st.UpdatePasswordTx(ctx, tx, userID, string(hash)); err != nil {
			return err
		}
		return st.RevokeUserSessionsTx(ctx, tx, userID)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return httpapi.E(http.StatusBadRequest, "invalid_reset_token", "找回链接无效或已过期")
	}
	if err != nil {
		return fmt.Errorf("完成密码重置失败: %w", err)
	}
	return nil
}

func validatePassword(password string) *httpapi.APIError {
	if len([]byte(password)) < minPasswordLen {
		return httpapi.ValidationError(map[string]string{"password": "密码至少 8 位"})
	}
	if len([]byte(password)) > maxPasswordByte {
		return httpapi.ValidationError(map[string]string{"password": "密码不能超过 72 字节"})
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, userID string) (Tokens, error) {
	accessToken, err := NewToken()
	if err != nil {
		return Tokens{}, err
	}
	refreshToken, err := NewToken()
	if err != nil {
		return Tokens{}, err
	}
	if err := s.store.CreateSessionPair(ctx, userID, HashToken(accessToken), HashToken(refreshToken), s.accessTokenTTL, s.refreshTokenTTL); err != nil {
		return Tokens{}, fmt.Errorf("创建会话失败: %w", err)
	}
	return Tokens{AccessToken: accessToken, RefreshToken: refreshToken}, nil
}

func EInvalidCredentials() *httpapi.APIError {
	return httpapi.E(401, "invalid_credentials", "邮箱或密码不正确")
}

func isUniqueViolation(err error) bool {
	type coder interface{ SQLState() string }
	var c coder
	return errors.As(err, &c) && c.SQLState() == "23505"
}
