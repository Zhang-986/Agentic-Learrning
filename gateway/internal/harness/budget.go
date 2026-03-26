package harness

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/agentic-learning/gateway/internal/model"
)

// ==================== Budget Control 层 ====================
// 对标 nxcode.io / Anthropic 的预算管理：
// 每个 Session 设置 Token 预算和时间预算，超出即熔断整个 Session。

// BudgetConfig 预算配置
type BudgetConfig struct {
	MaxTotalTokens int           // 整个 Session 的最大 Token 消耗（0 = 不限制）
	MaxSessionTime time.Duration // 整个 Session 的最大运行时间（0 = 不限制）
	MaxLLMCalls    int           // 最大 LLM 调用次数（0 = 不限制）
}

func DefaultBudgetConfig() BudgetConfig {
	return BudgetConfig{
		MaxTotalTokens: 50000,
		MaxSessionTime: 10 * time.Minute,
		MaxLLMCalls:    30,
	}
}

// BudgetTracker 预算追踪器
type BudgetTracker struct {
	mu          sync.Mutex
	config      BudgetConfig
	startTime   time.Time
	totalTokens int
	llmCalls    int
	exhausted   bool
	reason      string
}

func NewBudgetTracker(config BudgetConfig) *BudgetTracker {
	return &BudgetTracker{
		config:    config,
		startTime: time.Now(),
	}
}

// BudgetStatus 预算状态快照
type BudgetStatus struct {
	TotalTokensUsed  int           `json:"total_tokens_used"`
	TotalTokensLimit int           `json:"total_tokens_limit"`
	TokenUtilization float64       `json:"token_utilization"`
	LLMCalls         int           `json:"llm_calls"`
	LLMCallsLimit    int           `json:"llm_calls_limit"`
	ElapsedTime      time.Duration `json:"elapsed_time"`
	TimeLimit        time.Duration `json:"time_limit"`
	TimeUtilization  float64       `json:"time_utilization"`
	Exhausted        bool          `json:"exhausted"`
	Reason           string        `json:"reason,omitempty"`
}

// RecordUsage 记录一次 LLM 调用的 token 消耗
func (b *BudgetTracker) RecordUsage(inputTokens, outputTokens int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.exhausted {
		return fmt.Errorf("budget exhausted: %s", b.reason)
	}

	b.totalTokens += inputTokens + outputTokens
	b.llmCalls++

	if b.config.MaxTotalTokens > 0 && b.totalTokens >= b.config.MaxTotalTokens {
		b.exhausted = true
		b.reason = fmt.Sprintf("token budget exceeded: %d/%d", b.totalTokens, b.config.MaxTotalTokens)
		return fmt.Errorf(b.reason)
	}

	if b.config.MaxLLMCalls > 0 && b.llmCalls >= b.config.MaxLLMCalls {
		b.exhausted = true
		b.reason = fmt.Sprintf("LLM call budget exceeded: %d/%d", b.llmCalls, b.config.MaxLLMCalls)
		return fmt.Errorf(b.reason)
	}

	if b.config.MaxSessionTime > 0 && time.Since(b.startTime) >= b.config.MaxSessionTime {
		b.exhausted = true
		b.reason = fmt.Sprintf("time budget exceeded: %v/%v", time.Since(b.startTime), b.config.MaxSessionTime)
		return fmt.Errorf(b.reason)
	}

	return nil
}

// Check 仅检查是否超预算（不记录消耗）
func (b *BudgetTracker) Check() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.exhausted {
		return fmt.Errorf("budget exhausted: %s", b.reason)
	}

	if b.config.MaxSessionTime > 0 && time.Since(b.startTime) >= b.config.MaxSessionTime {
		b.exhausted = true
		b.reason = fmt.Sprintf("time budget exceeded: %v/%v", time.Since(b.startTime), b.config.MaxSessionTime)
		return fmt.Errorf(b.reason)
	}

	return nil
}

// Status 获取当前预算状态
func (b *BudgetTracker) Status() BudgetStatus {
	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := time.Since(b.startTime)
	status := BudgetStatus{
		TotalTokensUsed:  b.totalTokens,
		TotalTokensLimit: b.config.MaxTotalTokens,
		LLMCalls:         b.llmCalls,
		LLMCallsLimit:    b.config.MaxLLMCalls,
		ElapsedTime:      elapsed,
		TimeLimit:        b.config.MaxSessionTime,
		Exhausted:        b.exhausted,
		Reason:           b.reason,
	}
	if b.config.MaxTotalTokens > 0 {
		status.TokenUtilization = float64(b.totalTokens) / float64(b.config.MaxTotalTokens)
	}
	if b.config.MaxSessionTime > 0 {
		status.TimeUtilization = float64(elapsed) / float64(b.config.MaxSessionTime)
	}
	return status
}

// IsExhausted 是否已耗尽
func (b *BudgetTracker) IsExhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exhausted
}

// BudgetCheckBeforeHook 作为 MiddlewareChain 的 BeforeHook 使用
func BudgetCheckBeforeHook(tracker *BudgetTracker) BeforeHook {
	return func(ctx context.Context, agentName string, req *model.ChatCompletionRequest) error {
		return tracker.Check()
	}
}

// BudgetRecordAfterHook 作为 MiddlewareChain 的 AfterHook，记录消耗
func BudgetRecordAfterHook(tracker *BudgetTracker) AfterHook {
	return func(ctx context.Context, record *LLMCallRecord, resp *model.ChatCompletionResponse) error {
		return tracker.RecordUsage(record.InputTokens, record.OutputTokens)
	}
}
