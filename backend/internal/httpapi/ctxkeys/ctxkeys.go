// Package ctxkeys 承载跨模块共享的请求上下文值：请求 ID 与当前用户。
package ctxkeys

import "context"

type key int

const (
	requestIDKey key = iota
	userIDKey
	userRoleKey
)

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func WithUser(ctx context.Context, userID, role string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, userRoleKey, role)
}

func UserID(ctx context.Context) string {
	id, _ := ctx.Value(userIDKey).(string)
	return id
}

func UserRole(ctx context.Context) string {
	role, _ := ctx.Value(userRoleKey).(string)
	return role
}
