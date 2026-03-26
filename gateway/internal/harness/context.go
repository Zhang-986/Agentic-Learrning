package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ==================== Context Engineering 层 (Phase 4) ====================
//
// 核心理念转变：不做上下文压缩（Compaction），做上下文交接（Handoff）。
//
// 依据：
// - Anthropic: "Compaction isn't sufficient. Even with compaction, which doesn't
//   always pass perfectly clear instructions to the next agent."
//   → 用 Handoff Artifact 取代 CompactedSummary
//
// - Bassel Haidar: "A handoff is not a summary, it is a control document."
//   → HandoffArtifact 包含决策日志、失败教训、下一步建议
//
// - Anthropic context engineering: "Like Claude Code creating a to-do list,
//   or your custom agent maintaining a NOTES.md file"
//   → ProgressJournal（叙事日志）+ FeatureList（结构化清单）+ DecisionLog（决策记录）
//
// - Phil Schmid: "Do not build massive control flows. Provide robust atomic tools.
//   Let the model make the plan."
//   → ContextWindow 简化为纯粹的当前 session 工作记忆，不做任何压缩
//
// 架构：Priming Protocol → Session 执行 → Handoff Artifact 生成 → 持久化
//       下一个 Session → Priming Protocol 读取上次 Handoff → 快速进入状态

// ==================== Handoff Artifact ====================

// HandoffArtifact 是 session 结束时生成的结构化交接文档。
// 对标 Anthropic 的 claude-progress.txt，但更结构化。
// 它不是"摘要"，而是"控制文档"——下一个 session 的启动指令。
type HandoffArtifact struct {
	SessionID    string           `json:"session_id"`
	Goal         string           `json:"goal"`
	CreatedAt    time.Time        `json:"created_at"`
	FinalStatus  string           `json:"final_status"` // completed / failed / interrupted
	WhatChanged  []ChangeRecord   `json:"what_changed"`
	WhatVerified []VerifyRecord   `json:"what_verified"`
	WhatFailed   []FailureRecord  `json:"what_failed"`
	Decisions    []DecisionRecord `json:"decisions"`
	NextActions  []NextAction     `json:"next_actions"`
	DoNotRepeat  []string         `json:"do_not_repeat"` // 踩过的坑，避免重复
	Environment  EnvironmentState `json:"environment"`   // 环境健康快照
}

// ChangeRecord 记录本次 session 做了什么变更
type ChangeRecord struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
	Evidence    string `json:"evidence,omitempty"` // commit hash, test output 等
}

// VerifyRecord 记录哪些经过验证
type VerifyRecord struct {
	TaskID   string `json:"task_id"`
	Method   string `json:"method"` // evaluator / test / manual
	Score    int    `json:"score,omitempty"`
	Feedback string `json:"feedback,omitempty"`
}

// FailureRecord 记录失败及原因
type FailureRecord struct {
	TaskID    string `json:"task_id"`
	Error     string `json:"error"`
	RootCause string `json:"root_cause,omitempty"`
	Attempts  int    `json:"attempts"`
}

// DecisionRecord 记录关键决策及理由
type DecisionRecord struct {
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
	TaskID    string `json:"task_id,omitempty"`
}

// NextAction 下一步建议
type NextAction struct {
	Action   string `json:"action"`
	Priority string `json:"priority"` // critical / normal / optional
	Context  string `json:"context,omitempty"`
}

// EnvironmentState 环境健康快照
type EnvironmentState struct {
	Healthy     bool   `json:"healthy"`
	LastError   string `json:"last_error,omitempty"`
	OpenIssues  int    `json:"open_issues"`
	BudgetLeft  string `json:"budget_left,omitempty"`
}

// ==================== Handoff Builder ====================

// HandoffBuilder 在 session 执行过程中逐步收集交接信息
type HandoffBuilder struct {
	mu       sync.Mutex
	artifact HandoffArtifact
}

func NewHandoffBuilder(sessionID, goal string) *HandoffBuilder {
	return &HandoffBuilder{
		artifact: HandoffArtifact{
			SessionID:   sessionID,
			Goal:        goal,
			CreatedAt:   time.Now(),
			WhatChanged: []ChangeRecord{},
			WhatVerified: []VerifyRecord{},
			WhatFailed:  []FailureRecord{},
			Decisions:   []DecisionRecord{},
			NextActions: []NextAction{},
			DoNotRepeat: []string{},
		},
	}
}

// RecordChange 记录一次变更
func (b *HandoffBuilder) RecordChange(taskID, description, evidence string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.WhatChanged = append(b.artifact.WhatChanged, ChangeRecord{
		TaskID:      taskID,
		Description: description,
		Evidence:    evidence,
	})
}

// RecordVerification 记录一次验证
func (b *HandoffBuilder) RecordVerification(taskID, method string, score int, feedback string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.WhatVerified = append(b.artifact.WhatVerified, VerifyRecord{
		TaskID:   taskID,
		Method:   method,
		Score:    score,
		Feedback: feedback,
	})
}

// RecordFailure 记录一次失败
func (b *HandoffBuilder) RecordFailure(taskID, errMsg, rootCause string, attempts int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.WhatFailed = append(b.artifact.WhatFailed, FailureRecord{
		TaskID:    taskID,
		Error:     errMsg,
		RootCause: rootCause,
		Attempts:  attempts,
	})
}

// RecordDecision 记录一次关键决策
func (b *HandoffBuilder) RecordDecision(decision, rationale, taskID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.Decisions = append(b.artifact.Decisions, DecisionRecord{
		Decision:  decision,
		Rationale: rationale,
		TaskID:    taskID,
	})
}

// AddNextAction 添加下一步建议
func (b *HandoffBuilder) AddNextAction(action, priority, ctx string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.NextActions = append(b.artifact.NextActions, NextAction{
		Action:   action,
		Priority: priority,
		Context:  ctx,
	})
}

// AddDoNotRepeat 记录踩过的坑
func (b *HandoffBuilder) AddDoNotRepeat(lesson string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.DoNotRepeat = append(b.artifact.DoNotRepeat, lesson)
}

// SetEnvironment 设置环境健康快照
func (b *HandoffBuilder) SetEnvironment(env EnvironmentState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.Environment = env
}

// Build 构建最终的 Handoff Artifact
func (b *HandoffBuilder) Build(finalStatus string) HandoffArtifact {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.artifact.FinalStatus = finalStatus
	return b.artifact
}

// ==================== Handoff Persistence ====================

// HandoffStore 管理 Handoff Artifact 的持久化和读取
type HandoffStore struct {
	dir string
}

func NewHandoffStore(dir string) *HandoffStore {
	return &HandoffStore{dir: dir}
}

// Save 持久化 Handoff Artifact
func (s *HandoffStore) Save(artifact HandoffArtifact) error {
	if s.dir == "" {
		return nil
	}
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("handoff_%s_%s.json",
		artifact.SessionID,
		artifact.CreatedAt.Format("20060102_150405"),
	)
	path := filepath.Join(s.dir, filename)

	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadLatest 读取最近一次 Handoff Artifact
func (s *HandoffStore) LoadLatest() (*HandoffArtifact, error) {
	if s.dir == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// 按文件名倒序找最新的 handoff 文件
	var latest string
	for i := len(entries) - 1; i >= 0; i-- {
		name := entries[i].Name()
		if strings.HasPrefix(name, "handoff_") && strings.HasSuffix(name, ".json") {
			latest = name
			break
		}
	}
	if latest == "" {
		return nil, nil
	}

	data, err := os.ReadFile(filepath.Join(s.dir, latest))
	if err != nil {
		return nil, err
	}

	var artifact HandoffArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

// ==================== Priming Protocol ====================

// PrimingProtocol 在每个 session 启动时构建初始上下文。
// 不是压缩历史，而是分层加载交接制品。
//
// Layer 1: Feature List — 还剩什么没做
// Layer 2: Last Handoff — 上一班做了什么、踩了什么坑
// Layer 3: Progress Journal — 全局进度时间线
// Layer 4: Environment Check — 上一班有没有留下问题
type PrimingProtocol struct{}

func NewPrimingProtocol() *PrimingProtocol {
	return &PrimingProtocol{}
}

// BuildPrimingContext 构建启动上下文
// 返回一段结构化文本，注入到 Planner 的 system prompt 中
func (p *PrimingProtocol) BuildPrimingContext(
	features *FeatureTracker,
	lastHandoff *HandoffArtifact,
	progress *ProgressTracker,
) string {
	var sb strings.Builder

	// Layer 1: Feature List
	if features != nil {
		pending := features.GetPending()
		prog := features.GetProgress()
		if total, ok := prog["total"].(int); ok && total > 0 {
			sb.WriteString("=== FEATURE STATUS ===\n")
			sb.WriteString(fmt.Sprintf("Total: %v | Passed: %v | Remaining: %v | Completion: %.1f%%\n",
				prog["total"], prog["passed"], prog["remaining"], prog["completion_rate"]))
			if len(pending) > 0 {
				sb.WriteString("Pending features:\n")
				for _, f := range pending {
					sb.WriteString(fmt.Sprintf("  - [%s] %s\n", f.ID, f.Description))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Layer 2: Last Handoff Artifact
	if lastHandoff != nil {
		sb.WriteString("=== LAST SESSION HANDOFF ===\n")
		sb.WriteString(fmt.Sprintf("Session: %s | Status: %s | Time: %s\n",
			lastHandoff.SessionID, lastHandoff.FinalStatus,
			lastHandoff.CreatedAt.Format("2006-01-02 15:04:05")))

		if len(lastHandoff.WhatChanged) > 0 {
			sb.WriteString("What was done:\n")
			for _, c := range lastHandoff.WhatChanged {
				sb.WriteString(fmt.Sprintf("  ✓ [%s] %s", c.TaskID, c.Description))
				if c.Evidence != "" {
					sb.WriteString(fmt.Sprintf(" (evidence: %s)", c.Evidence))
				}
				sb.WriteString("\n")
			}
		}

		if len(lastHandoff.WhatFailed) > 0 {
			sb.WriteString("What failed:\n")
			for _, f := range lastHandoff.WhatFailed {
				sb.WriteString(fmt.Sprintf("  ✗ [%s] %s (root cause: %s, attempts: %d)\n",
					f.TaskID, f.Error, f.RootCause, f.Attempts))
			}
		}

		if len(lastHandoff.DoNotRepeat) > 0 {
			sb.WriteString("DO NOT REPEAT (lessons learned):\n")
			for _, lesson := range lastHandoff.DoNotRepeat {
				sb.WriteString(fmt.Sprintf("  ⚠ %s\n", lesson))
			}
		}

		if len(lastHandoff.NextActions) > 0 {
			sb.WriteString("Recommended next actions:\n")
			for _, a := range lastHandoff.NextActions {
				sb.WriteString(fmt.Sprintf("  → [%s] %s", a.Priority, a.Action))
				if a.Context != "" {
					sb.WriteString(fmt.Sprintf(" (%s)", a.Context))
				}
				sb.WriteString("\n")
			}
		}

		if len(lastHandoff.Decisions) > 0 {
			sb.WriteString("Key decisions made:\n")
			for _, d := range lastHandoff.Decisions {
				sb.WriteString(fmt.Sprintf("  • %s — reason: %s\n", d.Decision, d.Rationale))
			}
		}

		// Environment check
		if !lastHandoff.Environment.Healthy {
			sb.WriteString(fmt.Sprintf("⚠ ENVIRONMENT WARNING: %s (open issues: %d)\n",
				lastHandoff.Environment.LastError, lastHandoff.Environment.OpenIssues))
		}
		sb.WriteString("\n")
	}

	// Layer 3: Progress Journal (compact timeline)
	if progress != nil {
		entries := progress.GetEntries()
		if len(entries) > 0 {
			sb.WriteString("=== PROGRESS JOURNAL ===\n")
			// 只展示最近 20 条，避免膨胀
			start := 0
			if len(entries) > 20 {
				start = len(entries) - 20
				sb.WriteString(fmt.Sprintf("(showing last 20 of %d entries)\n", len(entries)))
			}
			for _, e := range entries[start:] {
				sb.WriteString(fmt.Sprintf("[%s] %s | %s | %s\n",
					e.Timestamp.Format("15:04:05"),
					e.SessionID,
					e.Action,
					e.Summary,
				))
			}
			sb.WriteString("\n")
		}
	}

	result := sb.String()
	if result == "" {
		return "No prior session context. This is the first session."
	}
	return result
}

// ==================== Progress Tracker (Anthropic 模式) ====================

// ProgressEntry 单条进度记录（对标 claude-progress.txt）
type ProgressEntry struct {
	SessionID   string    `json:"session_id"`
	Timestamp   time.Time `json:"timestamp"`
	TaskID      string    `json:"task_id,omitempty"`
	Action      string    `json:"action"`   // planned / started / completed / failed / re_planned
	Summary     string    `json:"summary"`
	ArtifactRef string    `json:"artifact_ref,omitempty"` // 指向 Artifact 快照
}

// ProgressTracker 跨 session 进度追踪
type ProgressTracker struct {
	mu      sync.Mutex
	entries []ProgressEntry
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{}
}

func (p *ProgressTracker) Record(entry ProgressEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry.Timestamp = time.Now()
	p.entries = append(p.entries, entry)
}

// GetProgressSummary 生成供下一个 session/context window 读取的进度摘要
// 对标 Anthropic 的 "Read the git logs and progress files to get up to speed"
func (p *ProgressTracker) GetProgressSummary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.entries) == 0 {
		return "无历史进度记录。这是第一个 session。"
	}

	var sb strings.Builder
	sb.WriteString("=== 历史执行进度 ===\n")
	for _, e := range p.entries {
		sb.WriteString(fmt.Sprintf("[%s] %s | Task: %s | %s\n",
			e.Timestamp.Format("15:04:05"),
			e.Action,
			e.TaskID,
			e.Summary,
		))
	}
	return sb.String()
}

// GetEntries 获取原始记录
func (p *ProgressTracker) GetEntries() []ProgressEntry {
	p.mu.Lock()
	defer p.mu.Unlock()
	cpy := make([]ProgressEntry, len(p.entries))
	copy(cpy, p.entries)
	return cpy
}

// ==================== Context Window (简化版) ====================
//
// 不再做任何压缩。ContextWindow 是纯粹的"当前 session 工作记忆"。
// 当 session 结束时，重要信息通过 HandoffArtifact 交接给下一个 session，
// 而不是在一个窗口内反复压缩。

// ContextWindow 管理单次 session 的工作上下文
type ContextWindow struct {
	mu sync.Mutex

	// 上下文组件（对标 Google ADK Working Context）
	SystemInstruction string            // 系统指令（始终保留）
	ToolDefinitions   []string          // 工具定义（始终保留）
	TaskContext       string            // 当前任务上下文
	History           []HistoryEntry    // 当前 session 的执行历史（不压缩）
	Memory            map[string]string // 长期记忆键值对
	PrimingContext    string            // 由 PrimingProtocol 生成的启动上下文
}

// HistoryEntry 历史条目
type HistoryEntry struct {
	Role      string    `json:"role"` // system / user / assistant / tool
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
	TokenEst  int       `json:"token_est"` // token 估算
}

func NewContextWindow() *ContextWindow {
	return &ContextWindow{
		Memory: make(map[string]string),
	}
}

// AddHistory 添加历史条目（不做任何压缩）
func (w *ContextWindow) AddHistory(role, content string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := HistoryEntry{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
		TokenEst:  estimateTokens(content),
	}
	w.History = append(w.History, entry)
}

// BuildPromptContext 构建最终传给 LLM 的上下文
// 对标 Google ADK "Working Context" — 每次 model call 只看到最小必要上下文
func (w *ContextWindow) BuildPromptContext() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	var parts []string

	// 1. 系统指令（始终在最前面）
	if w.SystemInstruction != "" {
		parts = append(parts, w.SystemInstruction)
	}

	// 2. Priming Context（交接上下文，来自上一个 session 的 Handoff）
	if w.PrimingContext != "" {
		parts = append(parts, fmt.Sprintf("[SESSION CONTEXT]\n%s", w.PrimingContext))
	}

	// 3. 长期记忆
	if len(w.Memory) > 0 {
		memParts := make([]string, 0, len(w.Memory))
		for k, v := range w.Memory {
			memParts = append(memParts, fmt.Sprintf("- %s: %s", k, v))
		}
		parts = append(parts, fmt.Sprintf("[MEMORY]\n%s", strings.Join(memParts, "\n")))
	}

	// 4. 当前任务上下文
	if w.TaskContext != "" {
		parts = append(parts, fmt.Sprintf("[CURRENT TASK]\n%s", w.TaskContext))
	}

	// 5. 当前 session 历史（全量，不压缩）
	if len(w.History) > 0 {
		histParts := make([]string, 0, len(w.History))
		for _, h := range w.History {
			histParts = append(histParts, fmt.Sprintf("[%s] %s", h.Role, h.Content))
		}
		parts = append(parts, fmt.Sprintf("[HISTORY]\n%s", strings.Join(histParts, "\n")))
	}

	return strings.Join(parts, "\n\n")
}

// GetTokenEstimate 估算当前上下文的总 token 数
func (w *ContextWindow) GetTokenEstimate() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.getTokenEstimateUnsafe()
}

func (w *ContextWindow) getTokenEstimateUnsafe() int {
	total := estimateTokens(w.SystemInstruction)
	total += estimateTokens(w.PrimingContext)
	total += estimateTokens(w.TaskContext)
	for _, h := range w.History {
		total += h.TokenEst
	}
	for _, v := range w.Memory {
		total += estimateTokens(v)
	}
	return total
}

// Stats 上下文状态统计
func (w *ContextWindow) Stats() map[string]interface{} {
	w.mu.Lock()
	defer w.mu.Unlock()

	return map[string]interface{}{
		"history_count":      len(w.History),
		"has_priming":        w.PrimingContext != "",
		"memory_keys":        len(w.Memory),
		"total_token_est":    w.getTokenEstimateUnsafe(),
	}
}

// ==================== Feature Tracker (Anthropic 模式) ====================

// FeatureStatus 功能完成状态（对标 Anthropic feature_list.json）
type FeatureStatus struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Category    string `json:"category"` // functional / non_functional / quality
	Passes      bool   `json:"passes"`
	TestedAt    string `json:"tested_at,omitempty"`
	TestedBy    string `json:"tested_by,omitempty"` // 哪个 session 测试的
}

// FeatureTracker 管理功能清单和完成状态
type FeatureTracker struct {
	mu       sync.Mutex
	features []FeatureStatus
}

func NewFeatureTracker() *FeatureTracker {
	return &FeatureTracker{}
}

// AddFeatures 批量添加功能（由 Planner/Initializer 设置）
func (f *FeatureTracker) AddFeatures(features []FeatureStatus) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.features = append(f.features, features...)
}

// MarkPassed 标记通过（对标 Anthropic "only mark features as passing after careful testing"）
func (f *FeatureTracker) MarkPassed(featureID string, sessionID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.features {
		if f.features[i].ID == featureID {
			f.features[i].Passes = true
			f.features[i].TestedAt = time.Now().Format(time.RFC3339)
			f.features[i].TestedBy = sessionID
			return true
		}
	}
	return false
}

// GetPending 获取未完成的功能列表
func (f *FeatureTracker) GetPending() []FeatureStatus {
	f.mu.Lock()
	defer f.mu.Unlock()

	var pending []FeatureStatus
	for _, feat := range f.features {
		if !feat.Passes {
			pending = append(pending, feat)
		}
	}
	return pending
}

// GetProgress 获取完成进度
func (f *FeatureTracker) GetProgress() map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	total := len(f.features)
	passed := 0
	for _, feat := range f.features {
		if feat.Passes {
			passed++
		}
	}

	return map[string]interface{}{
		"total":           total,
		"passed":          passed,
		"remaining":       total - passed,
		"completion_rate": func() float64 {
			if total == 0 {
				return 0
			}
			return float64(passed) / float64(total) * 100
		}(),
	}
}

// ToJSON 序列化为 JSON（供持久化/传递）
func (f *FeatureTracker) ToJSON() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := json.MarshalIndent(f.features, "", "  ")
	return string(data), err
}

// ==================== 工具函数 ====================

// estimateTokens 粗估 token 数（中英文混合场景）
func estimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	// 中文约 1 char = 1 token, 英文约 4 chars = 1 token
	// 取中间值: ~2 chars per token
	return len(s)/2 + 1
}
