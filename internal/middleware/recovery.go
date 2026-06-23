package middleware

import (
	"fmt"
	"runtime/debug"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CustomRecovery 捕获异常恢复中间件，记录 panic 堆栈并统一返回 500 错误
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				traceID := GetTraceID(c.Request.Context())
				stack := string(debug.Stack())

				global.Logger.Error("[Recovery from panic]",
					zap.String("trace_id", traceID),
					zap.Any("error", err),
					zap.String("request", fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)),
					zap.String("stack", stack),
				)

				// Respond with a standard 500 error mapped from e.ServerPanic
				response.Fail(c, e.ServerPanic)
				c.Abort()
			}
		}()
		c.Next()
	}
}
