package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置
type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Auth      AuthConfig      `mapstructure:"auth"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
	Zhipu     ProviderConfig  `mapstructure:"zhipu"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	APIKeys []string `mapstructure:"api_keys"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`
	Burst             int     `mapstructure:"burst"`
}

// ProviderConfig AI Provider 配置
type ProviderConfig struct {
	APIKey       string `mapstructure:"api_key"`
	BaseURL      string `mapstructure:"base_url"`
	DefaultModel string `mapstructure:"default_model"`
	Timeout      int    `mapstructure:"timeout"`
}

// Load 从配置文件和环境变量加载配置
func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./gateway")

	// 支持环境变量覆盖，前缀 GATEWAY_
	v.SetEnvPrefix("GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// 默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("rate_limit.requests_per_second", 10)
	v.SetDefault("rate_limit.burst", 20)
	v.SetDefault("zhipu.base_url", "https://open.bigmodel.cn/api/paas/v4")
	v.SetDefault("zhipu.default_model", "glm-4-flash")
	v.SetDefault("zhipu.timeout", 120)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置失败: %w", err)
	}

	// 解析 ${VAR} 格式的环境变量引用
	cfg.Zhipu.APIKey = expandEnvVar(cfg.Zhipu.APIKey)

	return &cfg, nil
}

// expandEnvVar 展开 ${VAR} 格式的环境变量
func expandEnvVar(s string) string {
	if strings.HasPrefix(s, "${") && strings.HasSuffix(s, "}") {
		envKey := s[2 : len(s)-1]
		if val := os.Getenv(envKey); val != "" {
			return val
		}
	}
	return s
}
