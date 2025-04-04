package auth

import "context"

type contextKey string

const (
	userIDKey contextKey = "userID"
)

func UserIDFromContext(ctx context.Context) (uint, bool) {
	id, ok := ctx.Value(userIDKey).(uint)
	return id, ok
}
