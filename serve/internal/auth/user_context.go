package auth

import (
	"context"
	"errors"
	"goera/serve/internal/database"
	"goera/serve/internal/models"
)

type contextKey string

const (
	userIDKey contextKey = "userID"
)

func UserIDFromContext(ctx context.Context) (uint, bool) {
	id, ok := ctx.Value(userIDKey).(uint)
	return id, ok
}

func GetUserFromContext(ctx context.Context) (*models.User, error) {
	userID, exists := UserIDFromContext(ctx)
	if !exists {
		return nil, errors.New("user ID not found in context")
	}

	db := database.GetDB()
	if db == nil {
		return nil, errors.New("database connection failed")
	}

	var user models.User
	result := db.First(&user, userID)
	if result.Error != nil {
		return nil, result.Error
	}

	return &user, nil
}
