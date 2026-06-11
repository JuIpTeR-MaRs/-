package middleware

import (
	"time"

	"dorm-repair-system/internal/global"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GinLogger logs HTTP requests using Zap with trace IDs and log level classification
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		traceID := GetTraceID(c.Request.Context())
		status := c.Writer.Status()

		fields := []zap.Field{
			zap.String("trace_id", traceID),
			zap.Int("status", status),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			zap.Duration("cost", cost),
		}

		if status >= 500 {
			global.Logger.Error("Request Error", fields...)
		} else if status >= 400 {
			global.Logger.Warn("Request Warning", fields...)
		} else {
			global.Logger.Info("Request Success", fields...)
		}
	}
}
