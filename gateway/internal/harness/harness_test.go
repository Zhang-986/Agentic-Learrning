package harness

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentic-learning/gateway/internal/model"
)

// ==================== Middleware Tests ====================

func TestMiddleware_BeforeHookReject(t *testing.T) {
	mw := NewMiddlewareChain()
	mw.AddBeforeHook(func(ctx context.Context, agent string, req *model.ChatCompletionRequest) error {
		if agent == "blocked" {
			return fmt.Errorf("agent '%s' is blocked", agent)
		}
		return nil
	})

	// 正常通过
	err := mw.RunBefore(context.Background(), "planner", nil)
	if err != nil {
		t.Fatalf("expected no error for planner, got: %v", err)
	}

	// 被拦截
	err = mw.RunBefore(context.Background(), "blocked", nil)
	if err == nil {
		t.Fatal("expected error for blocked agent")
	}
	t.Logf("✅ BeforeHook correctly rejected: %v", err)
}

func TestMiddleware_AfterHookLogging(t *testing.T) {
	mw := NewMiddlewareChain()
	mw.AddAfterHook(TokenTrackingHook())

	record := &LLMCallRecord{
		Agent:     "generator",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Success:   true,
	}
	resp := &model.ChatCompletionResponse{
		Usage: &model.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}

	err := mw.RunAfter(context.Background(), record, resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", record.InputTokens)
	}
	if record.OutputTokens != 50 {
		t.Errorf("expected 50 output tokens, got %d", record.OutputTokens)
	}

	// 检查日志记录
	log := mw.GetCallLog()
	if len(log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(log))
	}
	if log[0].Agent != "generator" {
		t.Errorf("expected agent 'generator', got '%s'", log[0].Agent)
	}

	t.Log("✅ AfterHook logging works")
}

func TestMiddleware_CallStats(t *testing.T) {
	mw := NewMiddlewareChain()
	mw.AddAfterHook(TokenTrackingHook())

	for i := 0; i < 5; i++ {
		record := &LLMCallRecord{
			Agent:     "planner",
			Success:   i != 3, // 第 4 次失败
			StartTime: time.Now(),
			EndTime:   time.Now().Add(100 * time.Millisecond),
			Latency:   100 * time.Millisecond,
		}
		resp := &model.ChatCompletionResponse{
			Usage: &model.Usage{
				PromptTokens:     200,
				CompletionTokens: 100,
			},
		}
		_ = mw.RunAfter(context.Background(), record, resp)
	}

	stats := mw.GetCallStats()
	if stats["total_calls"].(int) != 5 {
		t.Errorf("expected 5 total calls, got %v", stats["total_calls"])
	}
	if stats["failures"].(int) != 1 {
		t.Errorf("expected 1 failure, got %v", stats["failures"])
	}
	if stats["total_tokens"].(int) != 1500 {
		t.Errorf("expected 1500 total tokens, got %v", stats["total_tokens"])
	}

	t.Log("✅ CallStats works")
}

// ==================== CircuitBreaker Tests ====================

func TestCircuitBreaker_Tripping(t *testing.T) {
	cb := NewCircuitBreaker(3, 500*time.Millisecond)
	mw := NewMiddlewareChain()
	mw.AddBeforeHook(cb.BeforeHook())
	mw.AddAfterHook(cb.AfterHook())

	// 3 次失败应触发熔断
	for i := 0; i < 3; i++ {
		record := &LLMCallRecord{Success: false, EndTime: time.Now()}
		_ = mw.RunAfter(context.Background(), record, nil)
	}

	// 第 4 次应该被拦截
	err := mw.RunBefore(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected circuit breaker to trip")
	}
	t.Logf("✅ CircuitBreaker tripped: %v", err)

	// 等待冷却后恢复
	time.Sleep(600 * time.Millisecond)
	err = mw.RunBefore(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("expected circuit breaker to recover, got: %v", err)
	}
	t.Log("✅ CircuitBreaker recovered after cooldown")
}

// ==================== Validation Tests ====================

func TestValidator_PlannerOutput(t *testing.T) {
	schema := PlannerSchema()

	// 合法输出
	valid := `{"id": "t1", "title": "Research", "description": "Research the topic"}`
	r := schema.Validate(valid)
	if !r.Valid {
		t.Fatalf("expected valid, got errors: %s", r.ErrorString())
	}

	// 缺少必填字段
	missing := `{"id": "t1", "title": "Research"}`
	r = schema.Validate(missing)
	if r.Valid {
		t.Fatal("expected invalid for missing description")
	}
	t.Logf("✅ Missing field detected: %s", r.ErrorString())

	// 空字符串
	empty := `{"id": "", "title": "X", "description": "Y"}`
	r = schema.Validate(empty)
	if r.Valid {
		t.Fatal("expected invalid for empty id")
	}
	t.Logf("✅ Empty field detected: %s", r.ErrorString())
}

func TestValidator_EvaluatorOutput(t *testing.T) {
	schema := EvaluatorSchema()

	// 合法
	valid := `{"score": 85, "feedback": "Good", "passed": true}`
	r := schema.Validate(valid)
	if !r.Valid {
		t.Fatalf("expected valid, got: %s", r.ErrorString())
	}

	// score 超范围
	outOfRange := `{"score": 150, "feedback": "Good", "passed": true}`
	r = schema.Validate(outOfRange)
	if r.Valid {
		t.Fatal("expected invalid for score > 100")
	}
	t.Logf("✅ Out-of-range score detected: %s", r.ErrorString())

	// 类型错误
	wrongType := `{"score": "high", "feedback": "Good", "passed": true}`
	r = schema.Validate(wrongType)
	if r.Valid {
		t.Fatal("expected invalid for wrong type")
	}
	t.Logf("✅ Wrong type detected: %s", r.ErrorString())
}

func TestValidator_ArrayValidation(t *testing.T) {
	schema := PlannerSchema()

	// 合法数组
	valid := `[{"id": "t1", "title": "A", "description": "B"}, {"id": "t2", "title": "C", "description": "D"}]`
	r := schema.ValidateArray(valid)
	if !r.Valid {
		t.Fatalf("expected valid array, got: %s", r.ErrorString())
	}

	// 空数组
	empty := `[]`
	r = schema.ValidateArray(empty)
	if r.Valid {
		t.Fatal("expected invalid for empty array")
	}
	t.Logf("✅ Empty array rejected: %s", r.ErrorString())

	// 数组中有一个元素缺字段
	partial := `[{"id": "t1", "title": "A", "description": "B"}, {"id": "t2", "title": "C"}]`
	r = schema.ValidateArray(partial)
	if r.Valid {
		t.Fatal("expected invalid for partial element")
	}
	t.Logf("✅ Partial element detected: %s", r.ErrorString())
}

// ==================== Budget Tests ====================

func TestBudget_TokenExhaustion(t *testing.T) {
	bt := NewBudgetTracker(BudgetConfig{
		MaxTotalTokens: 1000,
		MaxLLMCalls:    0, // 不限制
		MaxSessionTime: 0, // 不限制
	})

	// 消耗 600 tokens
	err := bt.RecordUsage(400, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 再消耗 500 → 超预算
	err = bt.RecordUsage(300, 200)
	if err == nil {
		t.Fatal("expected budget exhaustion error")
	}
	t.Logf("✅ Token exhaustion detected: %v", err)

	// 后续 Check 也应该拒绝
	err = bt.Check()
	if err == nil {
		t.Fatal("expected Check to fail after exhaustion")
	}

	// 状态验证
	status := bt.Status()
	if !status.Exhausted {
		t.Error("expected exhausted = true")
	}
	if status.TotalTokensUsed != 1100 {
		t.Errorf("expected 1100 tokens used, got %d", status.TotalTokensUsed)
	}
}

func TestBudget_LLMCallLimit(t *testing.T) {
	bt := NewBudgetTracker(BudgetConfig{
		MaxTotalTokens: 0, // 不限制
		MaxLLMCalls:    3,
		MaxSessionTime: 0, // 不限制
	})

	for i := 0; i < 2; i++ {
		err := bt.RecordUsage(100, 50)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i, err)
		}
	}

	// 第 3 次应该触发限制
	err := bt.RecordUsage(100, 50)
	if err == nil {
		t.Fatal("expected LLM call limit error")
	}
	t.Logf("✅ LLM call limit detected: %v", err)
}

func TestBudget_TimeLimit(t *testing.T) {
	bt := NewBudgetTracker(BudgetConfig{
		MaxTotalTokens: 0,
		MaxLLMCalls:    0,
		MaxSessionTime: 100 * time.Millisecond,
	})

	// 立即检查应该通过
	err := bt.Check()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 等待超时
	time.Sleep(150 * time.Millisecond)

	err = bt.Check()
	if err == nil {
		t.Fatal("expected time limit error")
	}
	t.Logf("✅ Time limit detected: %v", err)
}

func TestBudget_Utilization(t *testing.T) {
	bt := NewBudgetTracker(BudgetConfig{
		MaxTotalTokens: 10000,
		MaxLLMCalls:    100,
		MaxSessionTime: 1 * time.Minute,
	})

	_ = bt.RecordUsage(2500, 2500)

	status := bt.Status()
	if status.TokenUtilization < 0.49 || status.TokenUtilization > 0.51 {
		t.Errorf("expected ~50%% token utilization, got %.2f", status.TokenUtilization)
	}
	t.Logf("✅ Utilization tracking: tokens=%.0f%%, time=%.2f%%",
		status.TokenUtilization*100, status.TimeUtilization*100)
}

// ==================== Context Engineering Tests ====================

func TestProgressTracker_RecordAndSummary(t *testing.T) {
	pt := NewProgressTracker()

	// 初始状态
	summary := pt.GetProgressSummary()
	if summary != "无历史进度记录。这是第一个 session。" {
		t.Fatalf("expected empty progress, got: %s", summary)
	}

	// 记录几条进度
	pt.Record(ProgressEntry{SessionID: "s1", TaskID: "t1", Action: "started", Summary: "开始任务1"})
	pt.Record(ProgressEntry{SessionID: "s1", TaskID: "t1", Action: "completed", Summary: "完成任务1"})
	pt.Record(ProgressEntry{SessionID: "s1", Action: "completed", Summary: "Session完成"})

	summary = pt.GetProgressSummary()
	if len(summary) == 0 {
		t.Fatal("expected non-empty summary")
	}

	entries := pt.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	t.Logf("✅ ProgressTracker: %d entries, summary length: %d", len(entries), len(summary))
}

// ==================== Handoff Artifact Tests (Phase 4 新增) ====================

func TestHandoffBuilder_FullLifecycle(t *testing.T) {
	builder := NewHandoffBuilder("sess_001", "Build a REST API")

	// 模拟 session 执行过程中的记录
	builder.RecordChange("t1", "Created user model", "commit abc123")
	builder.RecordChange("t2", "Added authentication endpoint", "test passed")

	builder.RecordVerification("t1", "evaluator", 85, "Good structure")
	builder.RecordVerification("t2", "evaluator", 92, "Excellent")

	builder.RecordFailure("t3", "timeout connecting to DB", "missing env var", 3)

	builder.RecordDecision("Use JWT for auth", "Industry standard, stateless", "t2")
	builder.RecordDecision("Skip rate limiting for now", "Not in MVP scope", "")

	builder.AddNextAction("Implement rate limiting", "normal", "Deferred from this session")
	builder.AddNextAction("Fix DB connection issue", "critical", "t3 failed due to missing DB_URL")

	builder.AddDoNotRepeat("Do not try to connect to DB without checking DB_URL env var first")

	builder.SetEnvironment(EnvironmentState{
		Healthy:    false,
		LastError:  "DB connection failed",
		OpenIssues: 1,
		BudgetLeft: "tokens: 30000/50000",
	})

	artifact := builder.Build("failed")

	// 验证所有字段
	if artifact.SessionID != "sess_001" {
		t.Errorf("expected session ID 'sess_001', got '%s'", artifact.SessionID)
	}
	if artifact.FinalStatus != "failed" {
		t.Errorf("expected status 'failed', got '%s'", artifact.FinalStatus)
	}
	if len(artifact.WhatChanged) != 2 {
		t.Errorf("expected 2 changes, got %d", len(artifact.WhatChanged))
	}
	if len(artifact.WhatVerified) != 2 {
		t.Errorf("expected 2 verifications, got %d", len(artifact.WhatVerified))
	}
	if len(artifact.WhatFailed) != 1 {
		t.Errorf("expected 1 failure, got %d", len(artifact.WhatFailed))
	}
	if artifact.WhatFailed[0].Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", artifact.WhatFailed[0].Attempts)
	}
	if len(artifact.Decisions) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(artifact.Decisions))
	}
	if len(artifact.NextActions) != 2 {
		t.Errorf("expected 2 next actions, got %d", len(artifact.NextActions))
	}
	if len(artifact.DoNotRepeat) != 1 {
		t.Errorf("expected 1 do-not-repeat lesson, got %d", len(artifact.DoNotRepeat))
	}
	if artifact.Environment.Healthy {
		t.Error("expected environment unhealthy")
	}
	if artifact.Environment.OpenIssues != 1 {
		t.Errorf("expected 1 open issue, got %d", artifact.Environment.OpenIssues)
	}

	t.Logf("✅ HandoffBuilder full lifecycle: %d changes, %d failures, %d decisions",
		len(artifact.WhatChanged), len(artifact.WhatFailed), len(artifact.Decisions))
}

func TestHandoffStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewHandoffStore(dir)

	// 保存两个 handoff（模拟两个 session）
	artifact1 := HandoffArtifact{
		SessionID:   "sess_001",
		Goal:        "First goal",
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		FinalStatus: "completed",
		WhatChanged: []ChangeRecord{{TaskID: "t1", Description: "did something"}},
		WhatVerified: []VerifyRecord{},
		WhatFailed:  []FailureRecord{},
		Decisions:   []DecisionRecord{},
		NextActions: []NextAction{},
		DoNotRepeat: []string{},
	}
	if err := store.Save(artifact1); err != nil {
		t.Fatalf("save artifact1 failed: %v", err)
	}

	// 稍等一秒确保文件名排序不同
	time.Sleep(10 * time.Millisecond)

	artifact2 := HandoffArtifact{
		SessionID:   "sess_002",
		Goal:        "Second goal",
		CreatedAt:   time.Now(),
		FinalStatus: "failed",
		WhatChanged: []ChangeRecord{},
		WhatVerified: []VerifyRecord{},
		WhatFailed:  []FailureRecord{{TaskID: "t5", Error: "boom", Attempts: 2}},
		Decisions:   []DecisionRecord{},
		NextActions: []NextAction{{Action: "Retry t5", Priority: "critical"}},
		DoNotRepeat: []string{"Don't forget to check prerequisites"},
	}
	if err := store.Save(artifact2); err != nil {
		t.Fatalf("save artifact2 failed: %v", err)
	}

	// 验证文件数量
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatalf("expected 2 handoff files, got %d", len(entries))
	}

	// 加载最新的（应该是 artifact2）
	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("load latest failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil artifact")
	}
	if loaded.SessionID != "sess_002" {
		t.Errorf("expected latest session 'sess_002', got '%s'", loaded.SessionID)
	}
	if loaded.FinalStatus != "failed" {
		t.Errorf("expected status 'failed', got '%s'", loaded.FinalStatus)
	}
	if len(loaded.DoNotRepeat) != 1 {
		t.Errorf("expected 1 do-not-repeat, got %d", len(loaded.DoNotRepeat))
	}

	t.Logf("✅ HandoffStore: saved 2, loaded latest = %s (status: %s)",
		loaded.SessionID, loaded.FinalStatus)
}

func TestHandoffStore_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewHandoffStore(dir)

	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for empty store")
	}

	t.Log("✅ HandoffStore: empty dir returns nil")
}

func TestHandoffStore_NonExistentDir(t *testing.T) {
	store := NewHandoffStore(filepath.Join(t.TempDir(), "nonexistent"))

	loaded, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Fatal("expected nil for non-existent dir")
	}

	t.Log("✅ HandoffStore: non-existent dir returns nil gracefully")
}

// ==================== Priming Protocol Tests (Phase 4 新增) ====================

func TestPrimingProtocol_FirstSession(t *testing.T) {
	priming := NewPrimingProtocol()

	result := priming.BuildPrimingContext(nil, nil, nil)
	if result != "No prior session context. This is the first session." {
		t.Fatalf("expected first-session message, got: %s", result)
	}

	t.Log("✅ PrimingProtocol: first session returns default message")
}

func TestPrimingProtocol_WithHandoff(t *testing.T) {
	priming := NewPrimingProtocol()

	// 构造上一个 session 的 handoff
	lastHandoff := &HandoffArtifact{
		SessionID:   "sess_prev",
		FinalStatus: "completed",
		CreatedAt:   time.Now().Add(-30 * time.Minute),
		WhatChanged: []ChangeRecord{
			{TaskID: "t1", Description: "Built user model", Evidence: "commit abc"},
			{TaskID: "t2", Description: "Added auth endpoint", Evidence: "tests pass"},
		},
		WhatFailed: []FailureRecord{
			{TaskID: "t3", Error: "DB timeout", RootCause: "missing env var", Attempts: 3},
		},
		Decisions: []DecisionRecord{
			{Decision: "Use JWT", Rationale: "Industry standard"},
		},
		NextActions: []NextAction{
			{Action: "Fix DB connection", Priority: "critical", Context: "Need DB_URL"},
			{Action: "Add rate limiting", Priority: "normal"},
		},
		DoNotRepeat: []string{
			"Do not connect to DB without checking DB_URL first",
		},
		Environment: EnvironmentState{
			Healthy:    false,
			LastError:  "DB connection failed",
			OpenIssues: 1,
		},
	}

	// 构造 feature tracker
	features := NewFeatureTracker()
	features.AddFeatures([]FeatureStatus{
		{ID: "f1", Description: "User model", Passes: true},
		{ID: "f2", Description: "Auth endpoint", Passes: true},
		{ID: "f3", Description: "DB integration", Passes: false},
	})
	features.MarkPassed("f1", "sess_prev")
	features.MarkPassed("f2", "sess_prev")

	// 构造 progress tracker
	progress := NewProgressTracker()
	progress.Record(ProgressEntry{SessionID: "sess_prev", TaskID: "t1", Action: "completed", Summary: "Built user model"})
	progress.Record(ProgressEntry{SessionID: "sess_prev", TaskID: "t2", Action: "completed", Summary: "Added auth"})
	progress.Record(ProgressEntry{SessionID: "sess_prev", TaskID: "t3", Action: "failed", Summary: "DB timeout"})

	result := priming.BuildPrimingContext(features, lastHandoff, progress)

	// 验证各层内容存在
	if !strings.Contains(result, "FEATURE STATUS") {
		t.Error("missing FEATURE STATUS section")
	}
	if !strings.Contains(result, "Remaining: 1") {
		t.Error("missing remaining feature count")
	}
	if !strings.Contains(result, "LAST SESSION HANDOFF") {
		t.Error("missing LAST SESSION HANDOFF section")
	}
	if !strings.Contains(result, "Built user model") {
		t.Error("missing change record")
	}
	if !strings.Contains(result, "DB timeout") {
		t.Error("missing failure record")
	}
	if !strings.Contains(result, "DO NOT REPEAT") {
		t.Error("missing DO NOT REPEAT section")
	}
	if !strings.Contains(result, "DB_URL") {
		t.Error("missing do-not-repeat lesson content")
	}
	if !strings.Contains(result, "ENVIRONMENT WARNING") {
		t.Error("missing environment warning")
	}
	if !strings.Contains(result, "PROGRESS JOURNAL") {
		t.Error("missing PROGRESS JOURNAL section")
	}
	if !strings.Contains(result, "Fix DB connection") {
		t.Error("missing next action")
	}
	if !strings.Contains(result, "Use JWT") {
		t.Error("missing decision record")
	}

	t.Logf("✅ PrimingProtocol with full handoff: %d chars", len(result))
}

func TestPrimingProtocol_OnlyFeatures(t *testing.T) {
	priming := NewPrimingProtocol()

	features := NewFeatureTracker()
	features.AddFeatures([]FeatureStatus{
		{ID: "f1", Description: "Login page", Passes: false},
		{ID: "f2", Description: "Dashboard", Passes: false},
	})

	result := priming.BuildPrimingContext(features, nil, nil)

	if !strings.Contains(result, "FEATURE STATUS") {
		t.Error("missing FEATURE STATUS")
	}
	if !strings.Contains(result, "Remaining: 2") {
		t.Error("expected 2 remaining features")
	}
	if strings.Contains(result, "LAST SESSION HANDOFF") {
		t.Error("should not contain handoff section when nil")
	}

	t.Logf("✅ PrimingProtocol with only features: %d chars", len(result))
}

// ==================== ContextWindow Tests (简化版) ====================

func TestContextWindow_NoCompaction(t *testing.T) {
	cw := NewContextWindow()
	cw.SystemInstruction = "你是一个测试助手"

	// 添加 20 条历史 — 不再触发任何压缩
	for i := 0; i < 20; i++ {
		cw.AddHistory("user", fmt.Sprintf("消息 %d", i))
	}

	// 验证全部保留（不压缩）
	if len(cw.History) != 20 {
		t.Fatalf("expected 20 history items (no compaction), got %d", len(cw.History))
	}

	t.Logf("✅ ContextWindow: 20 items retained without compaction")
}

func TestContextWindow_WithPrimingContext(t *testing.T) {
	cw := NewContextWindow()
	cw.SystemInstruction = "You are a helpful assistant"
	cw.PrimingContext = "=== LAST SESSION HANDOFF ===\nCompleted 3/5 tasks"
	cw.TaskContext = "Current task: fix the bug"
	cw.Memory["preference"] = "use Go"
	cw.AddHistory("user", "Hello")
	cw.AddHistory("assistant", "Hi! How can I help?")

	prompt := cw.BuildPromptContext()

	// 验证所有组件
	for _, expected := range []string{
		"You are a helpful assistant",
		"SESSION CONTEXT",
		"LAST SESSION HANDOFF",
		"MEMORY",
		"use Go",
		"CURRENT TASK",
		"fix the bug",
		"HISTORY",
		"Hello",
	} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt missing '%s'", expected)
		}
	}

	tokenEst := cw.GetTokenEstimate()
	if tokenEst <= 0 {
		t.Error("expected positive token estimate")
	}

	stats := cw.Stats()
	if !stats["has_priming"].(bool) {
		t.Error("expected has_priming = true")
	}

	t.Logf("✅ ContextWindow with PrimingContext: %d chars, ~%d tokens", len(prompt), tokenEst)
}

// ==================== Feature Tracker Tests ====================

func TestFeatureTracker_Lifecycle(t *testing.T) {
	ft := NewFeatureTracker()

	// 添加功能
	ft.AddFeatures([]FeatureStatus{
		{ID: "f1", Description: "用户登录", Category: "functional"},
		{ID: "f2", Description: "数据展示", Category: "functional"},
		{ID: "f3", Description: "导出报告", Category: "functional"},
	})

	// 初始全部未通过
	pending := ft.GetPending()
	if len(pending) != 3 {
		t.Fatalf("expected 3 pending, got %d", len(pending))
	}

	progress := ft.GetProgress()
	if progress["total"].(int) != 3 {
		t.Errorf("expected total=3, got %v", progress["total"])
	}
	if progress["passed"].(int) != 0 {
		t.Errorf("expected passed=0, got %v", progress["passed"])
	}

	// 标记通过
	ft.MarkPassed("f1", "session_123")
	ft.MarkPassed("f2", "session_123")

	pending = ft.GetPending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].ID != "f3" {
		t.Errorf("expected f3 pending, got %s", pending[0].ID)
	}

	progress = ft.GetProgress()
	completionRate := progress["completion_rate"].(float64)
	if completionRate < 66.0 || completionRate > 67.0 {
		t.Errorf("expected ~66.67%% completion, got %.2f%%", completionRate)
	}

	// JSON 序列化
	jsonStr, err := ft.ToJSON()
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if len(jsonStr) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	t.Logf("✅ FeatureTracker: 2/3 passed (%.1f%%)", completionRate)
}

// ==================== Session Logger Tests ====================

func TestSessionLogger_Persist(t *testing.T) {
	logDir := t.TempDir()
	logger := NewSessionLogger("test_sess", "测试目标", logDir)

	logger.LogEvent("init", "初始化", nil)
	logger.LogEvent("task_start", "开始任务", "t1")
	logger.LogEvent("task_completed", "完成任务", "t1")

	budgetStatus := BudgetStatus{TotalTokensUsed: 500, LLMCalls: 3}
	log, err := logger.Finalize(
		"completed",
		[]LLMCallRecord{{Agent: "planner", Success: true}},
		[]ProgressEntry{{Action: "completed", Summary: "done"}},
		&budgetStatus,
		map[string]interface{}{"total_calls": 3},
	)
	if err != nil {
		t.Fatalf("finalize failed: %v", err)
	}

	if log.SessionID != "test_sess" {
		t.Errorf("expected session ID 'test_sess', got '%s'", log.SessionID)
	}
	if len(log.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(log.Events))
	}

	// 检查文件是否创建
	entries, _ := os.ReadDir(logDir)
	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}
	t.Logf("✅ SessionLogger persisted to: %s/%s", logDir, entries[0].Name())
}

// ==================== Phase 5: Tracing Tests ====================

func TestTraceCollector_FullLifecycle(t *testing.T) {
	tc := NewTraceCollector("sess_trace_1", "build a REST API")

	// Verify trace ID format (32 hex chars = 16 bytes)
	if len(tc.GetTraceID()) != 32 {
		t.Errorf("expected 32-char trace ID, got %d: %s", len(tc.GetTraceID()), tc.GetTraceID())
	}
	if len(tc.GetRootSpanID()) != 16 {
		t.Errorf("expected 16-char root span ID, got %d", len(tc.GetRootSpanID()))
	}

	// Iteration 0: Plan → Execute → Evaluate
	planSpan := tc.StartSpan("planner.plan", "planner", "")
	planSpan.SetAttribute("tasks_count", "3")
	planSpan.InputTokens = 500
	planSpan.OutputTokens = 200
	planSpan.End(SpanStatusOK, "")
	tc.FinishSpan(planSpan)

	execSpan := tc.StartSpan("generator.execute", "generator", "task_1")
	execSpan.InputTokens = 1000
	execSpan.OutputTokens = 800
	execSpan.End(SpanStatusOK, "")
	tc.FinishSpan(execSpan)

	evalSpan := tc.StartSpan("evaluator.evaluate", "evaluator", "task_1")
	evalSpan.SetAttribute("score", "85")
	evalSpan.InputTokens = 300
	evalSpan.OutputTokens = 100
	evalSpan.End(SpanStatusOK, "")
	tc.FinishSpan(evalSpan)

	// Verify iteration
	if tc.GetIteration() != 0 {
		t.Errorf("expected iteration 0, got %d", tc.GetIteration())
	}

	// Iteration 1: Re-plan
	iter := tc.NextIteration()
	if iter != 1 {
		t.Errorf("expected iteration 1, got %d", iter)
	}

	rePlanSpan := tc.StartSpan("planner.plan", "planner", "")
	rePlanSpan.End(SpanStatusOK, "")
	tc.FinishSpan(rePlanSpan)

	// Error span
	failSpan := tc.StartSpan("generator.execute", "generator", "task_2")
	failSpan.InputTokens = 500
	failSpan.OutputTokens = 0
	failSpan.End(SpanStatusError, "timeout exceeded")
	tc.FinishSpan(failSpan)

	// Finalize
	trace := tc.Finalize("completed")

	if trace.TraceID != tc.GetTraceID() {
		t.Errorf("trace ID mismatch")
	}
	if trace.SessionID != "sess_trace_1" {
		t.Errorf("expected session ID 'sess_trace_1', got '%s'", trace.SessionID)
	}
	if trace.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", trace.Status)
	}

	// 5 child spans + 1 root = 6
	if len(trace.Spans) != 6 {
		t.Errorf("expected 6 spans (1 root + 5 children), got %d", len(trace.Spans))
	}

	// Root span should have aggregated tokens
	root := trace.Spans[0]
	if root.Name != "session.execute" {
		t.Errorf("expected root span name 'session.execute', got '%s'", root.Name)
	}
	expectedInput := 500 + 1000 + 300 + 0 + 500
	if root.InputTokens != expectedInput {
		t.Errorf("expected root input tokens %d, got %d", expectedInput, root.InputTokens)
	}

	// Parent chain
	for _, s := range trace.Spans[1:] {
		if s.ParentSpanID != root.SpanID {
			t.Errorf("span %s should have parent %s, got %s", s.Name, root.SpanID, s.ParentSpanID)
		}
	}

	t.Logf("✅ TraceCollector full lifecycle: %d spans, trace_id=%s", len(trace.Spans), trace.TraceID[:8])
}

func TestTraceCollector_GetSpans(t *testing.T) {
	tc := NewTraceCollector("sess_get_spans", "test")

	s1 := tc.StartSpan("a", "planner", "")
	s1.End(SpanStatusOK, "")
	tc.FinishSpan(s1)

	s2 := tc.StartSpan("b", "generator", "t1")
	s2.End(SpanStatusOK, "")
	tc.FinishSpan(s2)

	spans := tc.GetSpans()
	if len(spans) != 2 {
		t.Errorf("expected 2 spans, got %d", len(spans))
	}

	// Verify it's a copy
	spans[0].Name = "modified"
	origSpans := tc.GetSpans()
	if origSpans[0].Name == "modified" {
		t.Error("GetSpans should return a copy, not a reference")
	}
}

func TestTraceCollector_NilSpanFinish(t *testing.T) {
	tc := NewTraceCollector("sess_nil", "test")
	// Should not panic
	tc.FinishSpan(nil)
	if len(tc.GetSpans()) != 0 {
		t.Error("expected 0 spans after nil finish")
	}
}

func TestComputeTraceStats(t *testing.T) {
	tc := NewTraceCollector("sess_stats", "test stats")

	// Planner
	p := tc.StartSpan("planner.plan", "planner", "")
	p.InputTokens = 500
	p.OutputTokens = 200
	time.Sleep(10 * time.Millisecond)
	p.End(SpanStatusOK, "")
	tc.FinishSpan(p)

	// Generator (2 calls)
	g1 := tc.StartSpan("generator.execute", "generator", "t1")
	g1.InputTokens = 1000
	g1.OutputTokens = 800
	time.Sleep(20 * time.Millisecond)
	g1.End(SpanStatusOK, "")
	tc.FinishSpan(g1)

	g2 := tc.StartSpan("generator.execute", "generator", "t2")
	g2.InputTokens = 900
	g2.OutputTokens = 700
	time.Sleep(5 * time.Millisecond)
	g2.End(SpanStatusError, "timeout")
	tc.FinishSpan(g2)

	// Evaluator
	e := tc.StartSpan("evaluator.evaluate", "evaluator", "t1")
	e.InputTokens = 300
	e.OutputTokens = 100
	e.End(SpanStatusOK, "")
	tc.FinishSpan(e)

	trace := tc.Finalize("completed")
	stats := ComputeTraceStats(trace)

	// Total spans = 5 (root + 4 children)
	if stats.TotalSpans != 5 {
		t.Errorf("expected 5 total spans, got %d", stats.TotalSpans)
	}

	// Iterations = 1 (only iteration 0)
	if stats.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", stats.Iterations)
	}

	// Token totals (from child spans only, root is skipped in stats)
	expectedInputTokens := 500 + 1000 + 900 + 300
	if stats.TotalInput != expectedInputTokens {
		t.Errorf("expected total input %d, got %d", expectedInputTokens, stats.TotalInput)
	}

	expectedOutputTokens := 200 + 800 + 700 + 100
	if stats.TotalOutput != expectedOutputTokens {
		t.Errorf("expected total output %d, got %d", expectedOutputTokens, stats.TotalOutput)
	}

	// Error spans
	if stats.ErrorSpans != 1 {
		t.Errorf("expected 1 error span, got %d", stats.ErrorSpans)
	}

	// By agent
	if stats.ByAgent["planner"].Calls != 1 {
		t.Errorf("expected 1 planner call, got %d", stats.ByAgent["planner"].Calls)
	}
	if stats.ByAgent["generator"].Calls != 2 {
		t.Errorf("expected 2 generator calls, got %d", stats.ByAgent["generator"].Calls)
	}
	if stats.ByAgent["generator"].Errors != 1 {
		t.Errorf("expected 1 generator error, got %d", stats.ByAgent["generator"].Errors)
	}
	if stats.ByAgent["evaluator"].Calls != 1 {
		t.Errorf("expected 1 evaluator call, got %d", stats.ByAgent["evaluator"].Calls)
	}

	// Bottleneck should be the slowest span
	if stats.Bottleneck == "" {
		t.Error("expected bottleneck to be identified")
	}

	t.Logf("✅ TraceStats: spans=%d, iterations=%d, input=%d, output=%d, errors=%d, bottleneck=%s",
		stats.TotalSpans, stats.Iterations, stats.TotalInput, stats.TotalOutput, stats.ErrorSpans, stats.Bottleneck)
}

func TestComputeTraceStats_EmptyTrace(t *testing.T) {
	trace := Trace{Spans: nil}
	stats := ComputeTraceStats(trace)
	if stats.TotalSpans != 0 {
		t.Errorf("expected 0 total spans, got %d", stats.TotalSpans)
	}
	if stats.Bottleneck != "" {
		t.Errorf("expected empty bottleneck, got '%s'", stats.Bottleneck)
	}
}

// ==================== Phase 5: Grading Tests ====================

func TestSessionGrader_PerfectSession(t *testing.T) {
	grader := NewDefaultSessionGrader()

	input := SessionGradeInput{
		SessionID:      "sess_perfect",
		TotalTasks:     5,
		CompletedTasks: 5,
		FailedTasks:    0,
		TotalAttempts:  5, // no retries
		RePlanCount:    0,
		MaxRePlans:     2,
		EvalScores:     []int{90, 85, 92, 88, 95},
		TotalTokens:    5000,
		TokenBudget:    100000,
		TotalDuration:  30 * time.Second,
		TimeBudget:     10 * time.Minute,
		ErrorCount:     0,
		CircuitTripped: false,
	}

	grade := grader.Grade(input)

	if grade.Verdict != VerdictPass {
		t.Errorf("expected pass verdict, got %s", grade.Verdict)
	}
	if grade.OverallScore < 85 {
		t.Errorf("expected overall score >= 85, got %.1f", grade.OverallScore)
	}
	if len(grade.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d: %v", len(grade.Violations), grade.Violations)
	}
	if grade.Dimensions.TaskCompletion != 100 {
		t.Errorf("expected 100%% task completion, got %.1f", grade.Dimensions.TaskCompletion)
	}
	if grade.Dimensions.PlanAdherence != 100 {
		t.Errorf("expected 100%% plan adherence, got %.1f", grade.Dimensions.PlanAdherence)
	}

	t.Logf("✅ Perfect session: score=%.1f verdict=%s dims=%+v",
		grade.OverallScore, grade.Verdict, grade.Dimensions)
}

func TestSessionGrader_FailedSession(t *testing.T) {
	grader := NewDefaultSessionGrader()

	input := SessionGradeInput{
		SessionID:      "sess_failed",
		TotalTasks:     5,
		CompletedTasks: 1,
		FailedTasks:    4,
		TotalAttempts:  13, // many retries
		RePlanCount:    2,
		MaxRePlans:     2,
		EvalScores:     []int{75, 30, 25, 20, 35},
		TotalTokens:    80000,
		TokenBudget:    100000,
		TotalDuration:  8 * time.Minute,
		TimeBudget:     10 * time.Minute,
		ErrorCount:     4,
		CircuitTripped: true,
	}

	grade := grader.Grade(input)

	if grade.Verdict != VerdictFail {
		t.Errorf("expected fail verdict, got %s", grade.Verdict)
	}
	if grade.OverallScore >= 60 {
		t.Errorf("expected overall score < 60, got %.1f", grade.OverallScore)
	}
	if len(grade.Violations) == 0 {
		t.Error("expected violations for failed session")
	}
	if len(grade.Recommendations) == 0 {
		t.Error("expected recommendations for failed session")
	}

	// TaskCompletion should be 20%
	if grade.Dimensions.TaskCompletion != 20 {
		t.Errorf("expected 20%% task completion, got %.1f", grade.Dimensions.TaskCompletion)
	}

	t.Logf("✅ Failed session: score=%.1f verdict=%s violations=%v recs=%v",
		grade.OverallScore, grade.Verdict, grade.Violations, grade.Recommendations)
}

func TestSessionGrader_WarnSession(t *testing.T) {
	grader := NewDefaultSessionGrader()

	input := SessionGradeInput{
		SessionID:      "sess_warn",
		TotalTasks:     4,
		CompletedTasks: 3,
		FailedTasks:    1,
		TotalAttempts:  6, // some retries
		RePlanCount:    1,
		MaxRePlans:     2,
		EvalScores:     []int{80, 70, 65, 40},
		TotalTokens:    50000,
		TokenBudget:    100000,
		TotalDuration:  5 * time.Minute,
		TimeBudget:     10 * time.Minute,
		ErrorCount:     1,
		CircuitTripped: false,
	}

	grade := grader.Grade(input)

	// Should be warn — some violations but overall >= 60
	if grade.OverallScore < 50 || grade.OverallScore > 90 {
		t.Errorf("expected moderate score, got %.1f", grade.OverallScore)
	}

	t.Logf("✅ Warn session: score=%.1f verdict=%s violations=%d recs=%d",
		grade.OverallScore, grade.Verdict, len(grade.Violations), len(grade.Recommendations))
}

func TestSessionGrader_QualityGateViolations(t *testing.T) {
	gate := QualityGate{
		MinOverallScore:   80,   // strict
		MinTaskCompletion: 70,
		MaxRePlanRatio:    0.3,
		MaxErrorRate:      0.1,
		MinAvgEvalScore:   70,
		MaxTokenWaste:     0.5,
	}
	grader := NewSessionGrader(gate)

	input := SessionGradeInput{
		SessionID:      "sess_strict",
		TotalTasks:     10,
		CompletedTasks: 6,
		FailedTasks:    4,
		TotalAttempts:  15,
		RePlanCount:    1,
		MaxRePlans:     2,
		EvalScores:     []int{65, 70, 55, 80, 60, 50},
		TotalTokens:    60000,
		TokenBudget:    100000,
		TotalDuration:  6 * time.Minute,
		TimeBudget:     10 * time.Minute,
		ErrorCount:     4,
		CircuitTripped: false,
	}

	grade := grader.Grade(input)

	// Should have multiple violations with strict gate
	if len(grade.Violations) == 0 {
		t.Error("expected violations with strict quality gate")
	}

	// error rate = 4/10 = 0.4 > 0.1
	found := false
	for _, v := range grade.Violations {
		if len(v) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one violation string")
	}

	t.Logf("✅ Strict quality gate: %d violations: %v", len(grade.Violations), grade.Violations)
}

func TestSessionGrader_EmptySession(t *testing.T) {
	grader := NewDefaultSessionGrader()

	input := SessionGradeInput{
		SessionID:      "sess_empty",
		TotalTasks:     0,
		CompletedTasks: 0,
		FailedTasks:    0,
		TotalAttempts:  0,
		RePlanCount:    0,
		MaxRePlans:     2,
		EvalScores:     nil,
		TotalTokens:    0,
		TokenBudget:    100000,
		TotalDuration:  0,
		TimeBudget:     10 * time.Minute,
	}

	grade := grader.Grade(input)

	// Should not panic, dimensions should be reasonable defaults
	if grade.Dimensions.TaskCompletion != 0 {
		t.Errorf("expected 0%% completion for empty session, got %.1f", grade.Dimensions.TaskCompletion)
	}
	if grade.Dimensions.EvalQuality != 0 {
		t.Errorf("expected 0 eval quality for empty session, got %.1f", grade.Dimensions.EvalQuality)
	}

	t.Logf("✅ Empty session: score=%.1f verdict=%s", grade.OverallScore, grade.Verdict)
}

func TestSpan_SetAttribute(t *testing.T) {
	s := &Span{Name: "test"}
	s.SetAttribute("key1", "val1")
	s.SetAttribute("key2", "val2")

	if s.Attributes["key1"] != "val1" {
		t.Errorf("expected val1, got %s", s.Attributes["key1"])
	}
	if len(s.Attributes) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(s.Attributes))
	}
}

func TestSpan_End(t *testing.T) {
	s := &Span{
		StartTime: time.Now().Add(-100 * time.Millisecond),
	}
	s.End(SpanStatusError, "something broke")

	if s.Status != SpanStatusError {
		t.Errorf("expected error status, got %s", s.Status)
	}
	if s.Error != "something broke" {
		t.Errorf("expected error message, got '%s'", s.Error)
	}
	if s.Duration < 100*time.Millisecond {
		t.Errorf("expected duration >= 100ms, got %v", s.Duration)
	}
	if s.EndTime.IsZero() {
		t.Error("expected EndTime to be set")
	}
}
