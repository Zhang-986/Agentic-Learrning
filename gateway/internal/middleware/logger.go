package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger 创建请求日志中间件
func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		// 处理请求
		c.Next()

		// 记录日志
		latency := time.Since(start)
		statusCode := c.Writer.Status()

		fields := []zap.Field{
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("body_size", c.Writer.Size()),
		}

		if len(c.Errors) > 0 {
			logger.Error("请求处理出错", append(fields, zap.String("errors", c.Errors.String()))...)
		} else if statusCode >= 500 {
			logger.Error("服务端错误", fields...)
		} else if statusCode >= 400 {
			logger.Warn("客户端错误", fields...)
		} else {
			logger.Info("请求完成", fields...)
		}
	}
}
