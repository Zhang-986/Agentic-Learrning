package middleware

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/agentic-learning/gateway/internal/model"
)

// RateLimit 创建基于令牌桶的限流中间件（per-client IP）
//
// 修复: 原实现全局共享单个 limiter，高并发下一个用户的突发请求
// 会耗尽所有用户的配额。现在改为 per-IP 独立桶。
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	var (
		mu       sync.Mutex
		limiters = make(map[string]*rate.Limiter)
	)

	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if lim, ok := limiters[key]; ok {
			return lim
		}
		lim := rate.NewLimiter(rate.Limit(rps), burst)
		limiters[key] = lim
		return lim
	}

	return func(c *gin.Context) {
		key := c.ClientIP()
		limiter := getLimiter(key)

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
