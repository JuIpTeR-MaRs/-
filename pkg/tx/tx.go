package tx

import (
	"context"

	"gorm.io/gorm"
)

type contextTxKey struct{}

// WithValue 将事务 DB 实例注入 context
func WithValue(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, contextTxKey{}, tx)
}

// FromContext 从 context 中提取事务 DB 实例
func FromContext(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(contextTxKey{}).(*gorm.DB)
	return tx, ok
}
