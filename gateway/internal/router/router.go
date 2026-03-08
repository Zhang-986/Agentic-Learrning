package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/agentic-learning/gateway/internal/config"
	"github.com/agentic-learning/gateway/internal/handler"
	"github.com/agentic-learning/gateway/internal/middleware"
	"github.com/agentic-learning/gateway/internal/provider"
)

// Setup 配置并返回 Gin Engine
func Setup(cfg *config.Config, registry *provider.Registry, logger *zap.Logger) *gin.Engine {
	if cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// 全局中间件
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(logger))

	// 健康检查（无需认证）
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "ok",
			"providers": registry.List(),
		})
	})

	// API 路由组（需认证 + 限流）
	v1 := r.Group("/v1")
	v1.Use(middleware.Auth(cfg.Auth.APIKeys))
	v1.Use(middleware.RateLimit(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst))
	{
		chatHandler := handler.NewChatHandler(registry)
		v1.POST("/chat/completions", chatHandler.Handle)
	}

	return r
}
