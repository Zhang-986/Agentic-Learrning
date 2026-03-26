package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentic-learning/gateway/internal/model"
)

// ==================== Middleware / Hooks 层 ====================
// 对标 LangChain deepagents 的 Middleware 模式：
// 在每次 LLM 调用前后插入可组合的 Hook，实现日志、校验、熔断等横切关注点。

// LLMCallRecord 单次 LLM 调用的完整记录
type LLMCallRecord struct {
	Agent        string        `json:"agent"`
	TaskID       string        `json:"task_id,omitempty"`
	SessionID    string        `json:"session_id"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	Latency      time.Duration `json:"latency"`
	InputTokens  int           `json:"input_tokens,omitempty"`
	OutputTokens int           `json:"output_tokens,omitempty"`
	Success      bool          `json:"success"`
	Error        string        `json:"error,omitempty"`
	RawInput     string        `json:"raw_input,omitempty"`
	RawOutput    string        `json:"raw_output,omitempty"`
}

// BeforeHook 在 LLM 调用前执行。返回 error 时中止该次调用。
type BeforeHook func(ctx context.Context, agentName string, req *model.ChatCompletionRequest) error

// AfterHook 在 LLM 调用后执行。可修改 response 或记录日志。
type AfterHook func(ctx context.Context, record *LLMCallRecord, resp *model.ChatCompletionResponse) error

// MiddlewareChain 管理所有 Hook 的执行链
type MiddlewareChain struct {
	mu          sync.RWMutex
	beforeHooks []BeforeHook
	afterHooks  []AfterHook
	callLog     []LLMCallRecord
	maxLogSize  int
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		maxLogSize: 1000,
	}
}

// AddBeforeHook 注册调用前 Hook
func (m *MiddlewareChain) AddBeforeHook(hook BeforeHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beforeHooks = append(m.beforeHooks, hook)
}

// AddAfterHook 注册调用后 Hook
func (m *MiddlewareChain) AddAfterHook(hook AfterHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.afterHooks = append(m.afterHooks, hook)
}

// RunBefore 执行所有 BeforeHook
func (m *MiddlewareChain) RunBefore(ctx context.Context, agentName string, req *model.ChatCompletionRequest) error {
	m.mu.RLock()
	hooks := make([]BeforeHook, len(m.beforeHooks))
	copy(hooks, m.beforeHooks)
	m.mu.RUnlock()

	for i, hook := range hooks {
		if err := hook(ctx, agentName, req); err != nil {
			return fmt.Errorf("before hook #%d failed: %w", i, err)
		}
	}
	return nil
}

// RunAfter 执行所有 AfterHook 并记录到结构化日志
func (m *MiddlewareChain) RunAfter(ctx context.Context, record *LLMCallRecord, resp *model.ChatCompletionResponse) error {
	m.mu.RLock()
	hooks := make([]AfterHook, len(m.afterHooks))
	copy(hooks, m.afterHooks)
	m.mu.RUnlock()

	for i, hook := range hooks {
		if err := hook(ctx, record, resp); err != nil {
			return fmt.Errorf("after hook #%d failed: %w", i, err)
		}
	}

	// 写入结构化日志
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.callLog) >= m.maxLogSize {
		m.callLog = m.callLog[1:]
	}
	m.callLog = append(m.callLog, *record)
	return nil
}

// GetCallLog 获取所有调用记录（事后回放/审计用）
func (m *MiddlewareChain) GetCallLog() []LLMCallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	log := make([]LLMCallRecord, len(m.callLog))
	copy(log, m.callLog)
	return log
}

// GetCallStats 获取统计摘要
func (m *MiddlewareChain) GetCallStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalLatency := time.Duration(0)
	totalInputTokens, totalOutputTokens, failures := 0, 0, 0
	byAgent := make(map[string]int)

	for _, r := range m.callLog {
		totalLatency += r.Latency
		totalInputTokens += r.InputTokens
		totalOutputTokens += r.OutputTokens
		if !r.Success {
			failures++
		}
		byAgent[r.Agent]++
	}

	return map[string]interface{}{
		"total_calls":        len(m.callLog),
		"total_latency_ms":   totalLatency.Milliseconds(),
		"total_input_tokens": totalInputTokens,
		"total_output_tokens": totalOutputTokens,
		"total_tokens":       totalInputTokens + totalOutputTokens,
		"failures":           failures,
		"calls_by_agent":     byAgent,
	}
}

// ==================== 内置 Hook 实现 ====================

// TokenTrackingHook 提取 token 用量到 record（AfterHook）
func TokenTrackingHook() AfterHook {
	return func(ctx context.Context, record *LLMCallRecord, resp *model.ChatCompletionResponse) error {
		if resp != nil && resp.Usage != nil {
			record.InputTokens = resp.Usage.PromptTokens
			record.OutputTokens = resp.Usage.CompletionTokens
		}
		return nil
	}
}

// CircuitBreaker 熔断器：连续失败 threshold 次后拒绝请求，直到 cooldown 过期
type CircuitBreaker struct {
	mu              sync.Mutex
	threshold       int
	cooldown        time.Duration
	consecutiveFail int
	lastFailTime    time.Time
	tripped         bool
}

func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown}
}

// BeforeHook 拦截检查
func (cb *CircuitBreaker) BeforeHook() BeforeHook {
	return func(ctx context.Context, agentName string, req *model.ChatCompletionRequest) error {
		cb.mu.Lock()
		defer cb.mu.Unlock()

		if cb.tripped {
			if time.Since(cb.lastFailTime) > cb.cooldown {
				cb.tripped = false
				cb.consecutiveFail = 0
			} else {
				return fmt.Errorf("circuit breaker tripped: %d consecutive failures, cooldown remaining %v",
					cb.consecutiveFail, cb.cooldown-time.Since(cb.lastFailTime))
			}
		}
		return nil
	}
}

// AfterHook 追踪失败计数
func (cb *CircuitBreaker) AfterHook() AfterHook {
	return func(ctx context.Context, record *LLMCallRecord, resp *model.ChatCompletionResponse) error {
		cb.mu.Lock()
		defer cb.mu.Unlock()

		if !record.Success {
			cb.consecutiveFail++
			cb.lastFailTime = record.EndTime
			if cb.consecutiveFail >= cb.threshold {
				cb.tripped = true
			}
		} else {
			cb.consecutiveFail = 0
		}
		return nil
	}
}
