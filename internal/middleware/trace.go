package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type traceIDKey struct{}

const TraceIDHeaderKey = "X-Trace-ID"
const TraceIDContextKey = "trace_id"

// TraceIDMiddleware 注入链路追踪 TraceID
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeaderKey)
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// 注入到 gin context
		c.Set(TraceIDContextKey, traceID)

		// 注入到标准库的 context 中
		stdCtx := context.WithValue(c.Request.Context(), traceIDKey{}, traceID)
		c.Request = c.Request.WithContext(stdCtx)

		// 在响应头中返回 TraceID
		c.Writer.Header().Set(TraceIDHeaderKey, traceID)

		c.Next()
	}
}

// GetTraceID 从 context 中获取 TraceID
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	// Try reading from traceIDKey
	if val, ok := ctx.Value(traceIDKey{}).(string); ok {
		return val
	}
	// Try reading from gin context key (if context is a gin.Context)
	if gc, ok := ctx.(*gin.Context); ok {
		if val, exists := gc.Get(TraceIDContextKey); exists {
			if s, ok := val.(string); ok {
				return s
			}
		}
	}
	return ""
}
