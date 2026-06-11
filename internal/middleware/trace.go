package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type traceIDKey struct{}

const TraceIDHeaderKey = "X-Trace-ID"
const TraceIDContextKey = "trace_id"

// TraceIDMiddleware injects a trace ID for request correlation tracking
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(TraceIDHeaderKey)
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Inject into gin context
		c.Set(TraceIDContextKey, traceID)

		// Inject into request's standard context so standard libraries can read it
		stdCtx := context.WithValue(c.Request.Context(), traceIDKey{}, traceID)
		c.Request = c.Request.WithContext(stdCtx)

		// Inject into response header
		c.Writer.Header().Set(TraceIDHeaderKey, traceID)

		c.Next()
	}
}

// GetTraceID retrieves the trace ID from a context
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
