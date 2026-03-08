package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/agentic-learning/gateway/internal/model"
)

// Auth 创建 API Key 认证中间件
func Auth(validKeys []string) gin.HandlerFunc {
	keySet := make(map[string]struct{}, len(validKeys))
	for _, k := range validKeys {
		keySet[k] = struct{}{}
	}

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
		if _, ok := keySet[apiKey]; !ok {
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
