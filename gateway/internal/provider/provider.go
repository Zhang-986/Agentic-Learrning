package provider

import (
	"context"

	"github.com/agentic-learning/gateway/internal/model"
)

// Provider AI 服务提供商接口
type Provider interface {
	// Name 返回 provider 名称
	Name() string

	// ChatCompletion 非流式聊天补全
	ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error)

	// StreamChatCompletion 流式聊天补全，通过 channel 返回 SSE 数据块
	StreamChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamChunk, <-chan error)
}

// Registry Provider 注册表
type Registry struct {
	providers map[string]Provider
	fallback  string
}

// NewRegistry 创建 Provider 注册表
func NewRegistry(defaultProvider string) *Registry {
	return &Registry{
		providers: make(map[string]Provider),
		fallback:  defaultProvider,
	}
}

// Register 注册一个 Provider
func (r *Registry) Register(p Provider) {
	r.providers[p.Name()] = p
}

// Get 获取指定名称的 Provider，如果为空则返回默认 Provider
func (r *Registry) Get(name string) (Provider, bool) {
	if name == "" {
		name = r.fallback
	}
	p, ok := r.providers[name]
	return p, ok
}

// List 列出所有已注册的 Provider 名称
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}
