package middleware

import (
	"strings"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"
	"dorm-repair-system/pkg/utils"

	"github.com/gin-gonic/gin"
)

// JWTAuth 中间件：校验请求中的 JWT 令牌
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Fail(c, e.Unauthorized, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.Fail(c, e.Unauthorized, "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			response.Fail(c, e.Unauthorized, "Invalid or expired token")
			c.Abort()
			return
		}

		// 将用户信息存入 context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// CasbinRBAC 中间件：使用 Casbin 进行基于角色的访问控制
func CasbinRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.Fail(c, e.Unauthorized, "Unauthorized")
			c.Abort()
			return
		}

		obj := c.Request.URL.Path
		act := c.Request.Method
		sub := role.(string)

		ok, err := global.Enforcer.Enforce(sub, obj, act)
		if err != nil {
			global.Logger.Error("Casbin enforce error: " + err.Error())
			response.Fail(c, e.ServerPanic, "Internal server error")
			c.Abort()
			return
		}

		if !ok {
			response.Fail(c, e.Forbidden, "Forbidden")
			c.Abort()
			return
		}

		c.Next()
	}
}
