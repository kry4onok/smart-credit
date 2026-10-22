package authmiddleware

import (
	"context"

	"github.com/google/uuid"
)

type userIDContextKey struct{}
type analystIDContextKey struct{}

func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

func UserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(uuid.UUID)
	return userID, ok
}

func WithAnalystID(ctx context.Context, analystID uuid.UUID) context.Context {
	return context.WithValue(WithUserID(ctx, analystID), analystIDContextKey{}, analystID)
}

func AnalystID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(analystIDContextKey{}).(uuid.UUID)
	return id, ok && id != uuid.Nil
}
