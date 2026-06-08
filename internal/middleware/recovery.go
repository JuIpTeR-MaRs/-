package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CustomRecovery intercepts panics, logs stack trace to Zap, and returns a unified JSON error response.
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the error and stack trace
				stack := string(debug.Stack())
				global.Logger.Error("[Recovery from panic]",
					zap.Any("error", err),
					zap.String("request", fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path)),
					zap.String("stack", stack),
				)

				// Respond with a standard 500 error
				c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
					Code: 500,
					Msg:  "系统繁忙",
					Data: nil,
				})
			}
		}()
		c.Next()
	}
}
