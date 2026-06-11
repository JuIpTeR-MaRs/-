package tx

import (
	"context"

	"gorm.io/gorm"
)

type contextTxKey struct{}

// WithValue injects the transaction DB instance into Context
func WithValue(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, contextTxKey{}, tx)
}

// FromContext extracts the transaction DB instance from Context
func FromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(contextTxKey{}).(*gorm.DB)
	return tx, ok
}
