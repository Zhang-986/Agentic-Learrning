package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	// 4. 设置路由
	r := router.Setup(cfg, registry, logger)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	// 5. Graceful Shutdown
	// 修复: 原实现直接调用 r.Run()，不处理 SIGTERM/SIGINT。
	// 在容器编排（Docker/K8s）中会导致请求中断。
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 启动服务（非阻塞）
	go func() {
		logger.Info("AI Gateway 已启动", zap.String("address", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("服务启动失败", zap.Error(err))
		}
	}()

	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("收到退出信号，正在优雅关闭...", zap.String("signal", sig.String()))

	// 给 30 秒的超时，让正在处理的请求完成（特别是 SSE 长连接）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("服务关闭异常", zap.Error(err))
	}
	logger.Info("AI Gateway 已关闭")
}
