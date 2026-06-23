package middleware

import (
	"fmt"
	"net/http"
	"time"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
)

// RateLimiterMiddleware 基于 Redis 令牌桶算法的 IP 限流中间件
// capacity: 桶的最大容量, rate: 每秒新增令牌速率
func RateLimiterMiddleware(capacity int64, rate int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		clientIP := c.ClientIP()
		limitKey := fmt.Sprintf("ratelimit:%s", clientIP)
		now := time.Now().Unix()

		// Read current bucket state from Redis
		vals, err := global.Redis.HMGet(ctx, limitKey, "last_time", "tokens").Result()
		
		var lastTime, tokens int64
		if err == nil && len(vals) == 2 && vals[0] != nil && vals[1] != nil {
			fmt.Sscanf(vals[0].(string), "%d", &lastTime)
			fmt.Sscanf(vals[1].(string), "%d", &tokens)
		} else {
			lastTime = now
			tokens = capacity
		}

		// Calculate refilled tokens based on time elapsed
		elapsed := now - lastTime
		if elapsed > 0 {
			tokens += elapsed * rate
			if tokens > capacity {
				tokens = capacity
			}
			lastTime = now
		}

		// Check and consume token
		if tokens > 0 {
			tokens--
			global.Redis.HMSet(ctx, limitKey, "last_time", lastTime, "tokens", tokens)
			global.Redis.Expire(ctx, limitKey, 10*time.Minute)
			c.Next()
		} else {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, response.Response{
				Code: e.ErrCode(42901),
				Msg:  "请求过于频繁，请稍候再试",
			})
		}
	}
}
