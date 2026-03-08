package main

import (
	"fmt"
	"log"

	"go.uber.org/zap"

	"github.com/agentic-learning/gateway/internal/config"
	"github.com/agentic-learning/gateway/internal/provider"
	"github.com/agentic-learning/gateway/internal/router"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 初始化日志
	var logger *zap.Logger
	if cfg.Server.Mode == "release" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}
	defer logger.Sync()

	logger.Info("正在启动 AI Gateway",
		zap.Int("port", cfg.Server.Port),
		zap.String("mode", cfg.Server.Mode),
	)

	// 3. 注册智谱 AI Provider
	registry := provider.NewRegistry("zhipu")
	registry.Register(provider.NewZhipuProvider(cfg.Zhipu))
	logger.Info("已注册 Provider: 智谱 AI",
		zap.String("base_url", cfg.Zhipu.BaseURL),
		zap.String("default_model", cfg.Zhipu.DefaultModel),
	)

	// 4. 设置路由并启动服务
	r := router.Setup(cfg, registry, logger)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	logger.Info("AI Gateway 已启动", zap.String("address", addr))

	if err := r.Run(addr); err != nil {
		logger.Fatal("服务启动失败", zap.Error(err))
	}
}
