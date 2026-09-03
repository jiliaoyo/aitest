package auth

import (
	"context"
	"time"

	"github.com/aishuati/backend/internal/store"
)

type Store struct{ db store.DBTx }

func NewStore(db store.DBTx) *Store { return &Store{db: db} }

func (s *Store) With(db store.DBTx) *Store { return &Store{db: db} }

func (s *Store) CreateUser(ctx context.Context, email, emailNormalized, passwordHash string, role Role) (User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		`INSERT INTO users (email, email_normalized, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, role, default_level_id`,
		email, emailNormalized, passwordHash, string(role),
	).Scan(&u.ID, &u.Email, &u.Role, &u.DefaultLevelID)
	return u, err
}

func (s *Store) UserByEmail(ctx context.Context, emailNormalized string) (id, hash string, role Role, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT id, password_hash, role FROM users WHERE email_normalized = $1`,
		emailNormalized,
	).Scan(&id, &hash, &role)
	return
}

func (s *Store) UserByID(ctx context.Context, id string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		`SELECT id, email, role, default_level_id FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.Role, &u.DefaultLevelID)
	return u, err
}

func (s *Store) SetDefaultLevel(ctx context.Context, userID string, levelID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET default_level_id = $2, updated_at = now() WHERE id = $1`,
		userID, levelID)
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	return s.UpdatePasswordTx(ctx, s.db, userID, passwordHash)
}

func (s *Store) UpdatePasswordTx(ctx context.Context, db store.DBTx, userID, passwordHash string) error {
	_, err := db.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`,
		userID, passwordHash)
	return err
}

func (s *Store) CreateSession(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO auth_sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, time.Now().Add(ttl))
	return err
}

// SessionUser 通过 token 哈希查回用户；过期或已撤销的 session 一律视为不存在。
func (s *Store) SessionUser(ctx context.Context, tokenHash string) (User, error) {
	var u User
	err := s.db.QueryRow(ctx,
		`SELECT u.id, u.email, u.role, u.default_level_id
		 FROM auth_sessions s JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()`,
		tokenHash,
	).Scan(&u.ID, &u.Email, &u.Role, &u.DefaultLevelID)
	return u, err
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash)
	return err
}

func (s *Store) CreatePasswordResetToken(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, time.Now().Add(ttl))
	return err
}

// ConsumePasswordResetToken 一次性消费找回令牌，返回对应用户 ID。
func (s *Store) ConsumePasswordResetToken(ctx context.Context, tokenHash string) (string, error) {
	return s.ConsumePasswordResetTokenTx(ctx, s.db, tokenHash)
}

func (s *Store) ConsumePasswordResetTokenTx(ctx context.Context, db store.DBTx, tokenHash string) (string, error) {
	var userID string
	err := db.QueryRow(ctx,
		`UPDATE password_reset_tokens SET used_at = now()
		 WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		 RETURNING user_id`,
		tokenHash,
	).Scan(&userID)
	return userID, err
}

func (s *Store) RevokeUserSessionsTx(ctx context.Context, db store.DBTx, userID string) error {
	_, err := db.Exec(ctx,
		`UPDATE auth_sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// HitRateLimit 在固定窗口内对 key 计数；超过 limit 返回 false。
func (s *Store) HitRateLimit(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`INSERT INTO rate_limit_counters (key, window_start, count, updated_at)
		 VALUES ($1, now(), 1, now())
		 ON CONFLICT (key) DO UPDATE SET
		   count = CASE WHEN rate_limit_counters.window_start < now() - make_interval(secs => $2) THEN 1 ELSE rate_limit_counters.count + 1 END,
		   window_start = CASE WHEN rate_limit_counters.window_start < now() - make_interval(secs => $2) THEN now() ELSE rate_limit_counters.window_start END,
		   updated_at = now()
		 RETURNING count`,
		key, window.Seconds(),
	).Scan(&count)
	if err != nil {
		return true, err
	}
	return count <= limit, nil
}
