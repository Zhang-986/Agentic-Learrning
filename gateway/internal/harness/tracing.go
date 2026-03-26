package harness

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ==================== Structured Tracing 层 (Phase 5) ====================
//
// 对齐 OpenTelemetry GenAI Semantic Convention 的数据模型，
// 但不引入外部 OTel SDK 依赖——保持 harness 轻量、可删除。
//
// 层级结构：
//   Trace (session 级)
//     └── Span (agent step 级)
//           ├── agent_name: planner / generator / evaluator
//           ├── input_tokens / output_tokens / latency
//           ├── status: ok / error
//           └── attributes (自定义键值对)
//
// 每一轮 Plan→Execute→Evaluate 形成一组 sibling spans，
// Re-plan 生成一个新的 iteration group。

// ==================== ID 生成 ====================

// generateID 生成 16 字节 hex 字符串（对齐 OTel 的 trace_id 格式）
func generateTraceID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateSpanID 生成 8 字节 hex 字符串（对齐 OTel 的 span_id 格式）
func generateSpanID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ==================== Span ====================

// SpanStatus span 的结束状态
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "ok"
	SpanStatusError SpanStatus = "error"
)

// Span 表示一次 agent 调用或编排动作
type Span struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Name         string            `json:"name"`          // e.g. "planner.plan", "generator.execute", "evaluator.evaluate"
	AgentName    string            `json:"agent_name"`    // planner / generator / evaluator / orchestrator
	TaskID       string            `json:"task_id,omitempty"`
	Iteration    int               `json:"iteration"`     // Plan→Execute→Evaluate 的轮次编号
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time,omitempty"`
	Duration     time.Duration     `json:"duration,omitempty"`
	Status       SpanStatus        `json:"status"`
	Error        string            `json:"error,omitempty"`
	InputTokens  int               `json:"input_tokens,omitempty"`
	OutputTokens int               `json:"output_tokens,omitempty"`
	Attributes   map[string]string `json:"attributes,omitempty"` // 自定义键值对
}

// SetAttribute 设置自定义属性
func (s *Span) SetAttribute(key, value string) {
	if s.Attributes == nil {
		s.Attributes = make(map[string]string)
	}
	s.Attributes[key] = value
}

// End 结束 span 并计算耗时
func (s *Span) End(status SpanStatus, errMsg string) {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime)
	s.Status = status
	if errMsg != "" {
		s.Error = errMsg
	}
}

// ==================== Trace ====================

// Trace 表示一次完整 session 的执行追踪
type Trace struct {
	TraceID   string    `json:"trace_id"`
	SessionID string    `json:"session_id"`
	Goal      string    `json:"goal"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time,omitempty"`
	Status    string    `json:"status,omitempty"` // completed / failed
	Spans     []Span    `json:"spans"`
}

// ==================== TraceCollector ====================

// TraceCollector 收集一次 session 执行过程中的所有 span
type TraceCollector struct {
	mu        sync.Mutex
	traceID   string
	sessionID string
	goal      string
	startTime time.Time
	rootSpan  *Span // session 级 root span
	spans     []Span
	iteration int // 当前 Plan→Execute→Evaluate 轮次
}

// NewTraceCollector 创建新的 trace 收集器
func NewTraceCollector(sessionID, goal string) *TraceCollector {
	traceID := generateTraceID()
	now := time.Now()

	rootSpan := &Span{
		TraceID:   traceID,
		SpanID:    generateSpanID(),
		Name:      "session.execute",
		AgentName: "orchestrator",
		StartTime: now,
		Status:    SpanStatusOK,
	}

	return &TraceCollector{
		traceID:   traceID,
		sessionID: sessionID,
		goal:      goal,
		startTime: now,
		rootSpan:  rootSpan,
		iteration: 0,
	}
}

// GetTraceID 返回当前 trace ID
func (tc *TraceCollector) GetTraceID() string {
	return tc.traceID
}

// GetRootSpanID 返回 root span 的 ID（用作 parent）
func (tc *TraceCollector) GetRootSpanID() string {
	return tc.rootSpan.SpanID
}

// NextIteration 递增 iteration（每次 Re-plan 时调用）
func (tc *TraceCollector) NextIteration() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.iteration++
	return tc.iteration
}

// GetIteration 获取当前 iteration
func (tc *TraceCollector) GetIteration() int {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.iteration
}

// StartSpan 开始一个新的 span
func (tc *TraceCollector) StartSpan(name, agentName, taskID string) *Span {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	span := &Span{
		TraceID:      tc.traceID,
		SpanID:       generateSpanID(),
		ParentSpanID: tc.rootSpan.SpanID,
		Name:         name,
		AgentName:    agentName,
		TaskID:       taskID,
		Iteration:    tc.iteration,
		StartTime:    time.Now(),
		Status:       SpanStatusOK,
	}
	return span
}

// FinishSpan 完成一个 span 并收集它
func (tc *TraceCollector) FinishSpan(span *Span) {
	if span == nil {
		return
	}
	if span.EndTime.IsZero() {
		span.End(span.Status, span.Error)
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.spans = append(tc.spans, *span)
}

// Finalize 完成整个 trace
func (tc *TraceCollector) Finalize(status string) Trace {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 结束 root span
	tc.rootSpan.End(SpanStatus(status), "")
	tc.rootSpan.SetAttribute("session.id", tc.sessionID)
	tc.rootSpan.SetAttribute("session.goal", tc.goal)
	tc.rootSpan.SetAttribute("session.iterations", fmt.Sprintf("%d", tc.iteration+1))
	tc.rootSpan.SetAttribute("session.total_spans", fmt.Sprintf("%d", len(tc.spans)))

	// 汇总 token
	totalInput, totalOutput := 0, 0
	for _, s := range tc.spans {
		totalInput += s.InputTokens
		totalOutput += s.OutputTokens
	}
	tc.rootSpan.InputTokens = totalInput
	tc.rootSpan.OutputTokens = totalOutput

	allSpans := make([]Span, 0, len(tc.spans)+1)
	allSpans = append(allSpans, *tc.rootSpan)
	allSpans = append(allSpans, tc.spans...)

	return Trace{
		TraceID:   tc.traceID,
		SessionID: tc.sessionID,
		Goal:      tc.goal,
		StartTime: tc.startTime,
		EndTime:   time.Now(),
		Status:    status,
		Spans:     allSpans,
	}
}

// GetSpans 获取当前已收集的 span（不含 root）
func (tc *TraceCollector) GetSpans() []Span {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	cpy := make([]Span, len(tc.spans))
	copy(cpy, tc.spans)
	return cpy
}

// ==================== Trace 统计 ====================

// TraceStats 从 trace 中提取统计信息
type TraceStats struct {
	TotalSpans    int                      `json:"total_spans"`
	Iterations    int                      `json:"iterations"`
	TotalDuration time.Duration            `json:"total_duration"`
	TotalInput    int                      `json:"total_input_tokens"`
	TotalOutput   int                      `json:"total_output_tokens"`
	ByAgent       map[string]AgentStats    `json:"by_agent"`
	ErrorSpans    int                      `json:"error_spans"`
	Bottleneck    string                   `json:"bottleneck"` // 耗时最长的 span name
}

// AgentStats 按 agent 维度的统计
type AgentStats struct {
	Calls        int           `json:"calls"`
	TotalLatency time.Duration `json:"total_latency"`
	AvgLatency   time.Duration `json:"avg_latency"`
	InputTokens  int           `json:"input_tokens"`
	OutputTokens int           `json:"output_tokens"`
	Errors       int           `json:"errors"`
}

// ComputeTraceStats 从 Trace 计算统计信息
func ComputeTraceStats(t Trace) TraceStats {
	stats := TraceStats{
		TotalSpans: len(t.Spans),
		ByAgent:    make(map[string]AgentStats),
	}

	if len(t.Spans) == 0 {
		return stats
	}

	stats.TotalDuration = t.EndTime.Sub(t.StartTime)

	var maxDuration time.Duration
	maxIteration := 0

	for _, s := range t.Spans {
		if s.Name == "session.execute" {
			continue // 跳过 root span
		}

		stats.TotalInput += s.InputTokens
		stats.TotalOutput += s.OutputTokens

		if s.Status == SpanStatusError {
			stats.ErrorSpans++
		}

		if s.Iteration > maxIteration {
			maxIteration = s.Iteration
		}

		if s.Duration > maxDuration {
			maxDuration = s.Duration
			stats.Bottleneck = s.Name
		}

		// 按 agent 分组
		as := stats.ByAgent[s.AgentName]
		as.Calls++
		as.TotalLatency += s.Duration
		as.InputTokens += s.InputTokens
		as.OutputTokens += s.OutputTokens
		if s.Status == SpanStatusError {
			as.Errors++
		}
		stats.ByAgent[s.AgentName] = as
	}

	stats.Iterations = maxIteration + 1

	// 计算平均延迟
	for name, as := range stats.ByAgent {
		if as.Calls > 0 {
			as.AvgLatency = as.TotalLatency / time.Duration(as.Calls)
		}
		stats.ByAgent[name] = as
	}

	return stats
}
