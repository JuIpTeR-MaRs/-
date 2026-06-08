package middleware

import (
	"strings"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/response"
	"dorm-repair-system/pkg/utils"


	"github.com/gin-gonic/gin"
)

// JWTAuth middleware checks the JWT token
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.ErrorWithStatus(c, 401, response.CodeError, "Authorization header is required")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			response.ErrorWithStatus(c, 401, response.CodeError, "Authorization header format must be Bearer {token}")
			c.Abort()
			return
		}

		claims, err := utils.ParseToken(parts[1])
		if err != nil {
			response.ErrorWithStatus(c, 401, response.CodeError, "Invalid or expired token")
			c.Abort()
			return
		}

		// Set user info to context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// CasbinRBAC middleware checks permissions using Casbin
func CasbinRBAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			response.ErrorWithStatus(c, 401, response.CodeError, "Unauthorized")
			c.Abort()
			return
		}

		obj := c.Request.URL.Path
		act := c.Request.Method
		sub := role.(string)

		ok, err := global.Enforcer.Enforce(sub, obj, act)
		if err != nil {
			global.Logger.Error("Casbin enforce error: " + err.Error())
			response.ErrorWithStatus(c, 500, response.CodeError, "Internal server error")
			c.Abort()
			return
		}

		if !ok {
			response.ErrorWithStatus(c, 403, response.CodeError, "Forbidden")
			c.Abort()
			return
		}

		c.Next()
	}
}
