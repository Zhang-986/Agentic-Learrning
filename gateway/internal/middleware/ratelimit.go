package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/agentic-learning/gateway/internal/model"
)

// RateLimit 创建基于令牌桶的限流中间件
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Limit(rps), burst)

	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, model.NewErrorResponse(
				"请求频率超限，请稍后重试",
				"rate_limit_error",
				"rate_limit_exceeded",
			))
			return
		}
		c.Next()
	}
}
