package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/agentic-learning/gateway/internal/model"
)

// Auth 创建 API Key 认证中间件
//
// 修复: 原实现使用 map lookup，不是常量时间比较，存在 timing attack 风险。
// 现在改为 crypto/subtle.ConstantTimeCompare 逐个比较。
func Auth(validKeys []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse(
				"缺少 Authorization 头",
				"authentication_error",
				"missing_api_key",
			))
			return
		}

		// 提取 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse(
				"Authorization 格式错误，应为 Bearer <api_key>",
				"authentication_error",
				"invalid_auth_format",
			))
			return
		}

		apiKey := parts[1]
		valid := false
		for _, k := range validKeys {
			if subtle.ConstantTimeCompare([]byte(apiKey), []byte(k)) == 1 {
				valid = true
				break
			}
		}

		if !valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, model.NewErrorResponse(
				"无效的 API Key",
				"authentication_error",
				"invalid_api_key",
			))
			return
		}

		c.Next()
	}
}
