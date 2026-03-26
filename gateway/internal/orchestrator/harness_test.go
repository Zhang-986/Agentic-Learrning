package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/model"
)

// MockProvider 模拟 AI 服务提供商
type MockProvider struct {
	responses []string
	callCount int
}

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	if p.callCount >= len(p.responses) {
		return nil, fmt.Errorf("no more mock responses (called %d times)", p.callCount)
	}
	resp := p.responses[p.callCount]
	p.callCount++
	return &model.ChatCompletionResponse{
		Choices: []model.ChatCompletionChoice{
			{
				Message: &model.ChatMessage{
					Role:    "assistant",
					Content: resp,
				},
			},
		},
	}, nil
}

func (p *MockProvider) StreamChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamChunk, <-chan error) {
	return nil, nil
}

// collectEvents 收集事件的 helper
func collectEvents(t *testing.T) (EventCallback, *[]model.HarnessEvent) {
	events := make([]model.HarnessEvent, 0)
	cb := func(e model.HarnessEvent) {
		events = append(events, e)
		t.Logf("[%s] %s", e.Type, e.Message)
	}
	return cb, &events
}

// ==================== Test 1: 基本的 Execute → Eval Pass ====================

func TestHarness_BasicSuccess(t *testing.T) {
	mock := &MockProvider{
		responses: []string{
			// Planner
			`[{"id": "t1", "title": "Research", "description": "Research the topic"}]`,
			// Generator
			`{"result": "Detailed research findings...", "updated_artifact_data": {"research": "done"}}`,
			// Evaluator
			`{"score": 90, "feedback": "Well done", "passed": true}`,
		},
	}

	orch := buildOrchestrator(mock)
	onEvent, events := collectEvents(t)

	session, err := orch.ExecuteSession(context.Background(), "Learn about Go", onEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证会话状态
	assertEqual(t, string(model.SessionCompleted), string(session.Status), "session status")
	assertEqual(t, 1, len(session.Tasks), "task count")
	assertEqual(t, string(model.TaskStatusCompleted), string(session.Tasks[0].Status), "task status")
	assertEqual(t, "Detailed research findings...", session.Tasks[0].Result, "task result")
	assertEqual(t, 1, session.Tasks[0].Attempts, "task attempts")

	// 验证度量
	assertEqual(t, 1, session.Metrics.CompletedTasks, "completed tasks")
	assertEqual(t, 0, session.Metrics.FailedTasks, "failed tasks")
	assertEqual(t, 0, session.Metrics.TotalRetries, "retries")

	// 验证 Artifact 有快照
	assertEqual(t, 1, len(session.ArtifactHistory), "artifact snapshots")

	// 验证有 metrics 事件
	hasMetrics := false
	for _, e := range *events {
		if e.Type == model.EventMetrics {
			hasMetrics = true
			break
		}
	}
	if !hasMetrics {
		t.Error("expected metrics event")
	}

	t.Log("✅ BasicSuccess passed")
}

// ==================== Test 2: 重试后通过（Resilience） ====================

func TestHarness_RetryThenPass(t *testing.T) {
	mock := &MockProvider{
		responses: []string{
			// Planner
			`[{"id": "t1", "title": "Task", "description": "Do something"}]`,
			// Generator attempt 1
			`{"result": "Shallow result", "updated_artifact_data": {"v": 1}}`,
			// Evaluator attempt 1 → fail
			`{"score": 40, "feedback": "Too shallow, need more depth", "passed": false}`,
			// Generator attempt 2 (retry)
			`{"result": "Deep and thorough result", "updated_artifact_data": {"v": 2}}`,
			// Evaluator attempt 2 → pass
			`{"score": 92, "feedback": "Excellent improvement", "passed": true}`,
		},
	}

	orch := buildOrchestrator(mock)
	onEvent, _ := collectEvents(t)

	session, err := orch.ExecuteSession(context.Background(), "Test", onEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, string(model.SessionCompleted), string(session.Status), "session status")
	assertEqual(t, string(model.TaskStatusCompleted), string(session.Tasks[0].Status), "task status")
	assertEqual(t, 2, session.Tasks[0].Attempts, "attempts")
	assertEqual(t, "Deep and thorough result", session.Tasks[0].Result, "result")
	assertEqual(t, 2, len(session.Tasks[0].EvalHistory), "eval history count")
	assertEqual(t, 1, session.Metrics.TotalRetries, "total retries")

	// 验证持久化
	saved, ok := orch.store.Get(session.ID)
	if !ok {
		t.Fatal("session not persisted")
	}
	assertEqual(t, string(model.SessionCompleted), string(saved.Status), "persisted status")

	t.Log("✅ RetryThenPass passed")
}

// ==================== Test 3: Re-plan 触发 ====================

func TestHarness_RePlan(t *testing.T) {
	mock := &MockProvider{
		responses: []string{
			// Planner (初始规划): 2 个任务
			`[{"id": "t1", "title": "Task 1", "description": "First task"}, {"id": "t2", "title": "Task 2", "description": "Second task"}]`,
			// Generator t1 attempt 1 → 失败
			`{"result": "bad", "updated_artifact_data": {}}`,
			// Evaluator t1 attempt 1 → fail
			`{"score": 20, "feedback": "Completely wrong", "passed": false}`,
			// Generator t1 attempt 2 → 还是失败
			`{"result": "still bad", "updated_artifact_data": {}}`,
			// Evaluator t1 attempt 2 → fail
			`{"score": 25, "feedback": "Still wrong", "passed": false}`,
			// Generator t1 attempt 3 → 第三次还是失败
			`{"result": "terrible", "updated_artifact_data": {}}`,
			// Evaluator t1 attempt 3 → fail（彻底失败，触发 Re-plan）
			`{"score": 15, "feedback": "Give up", "passed": false}`,

			// Re-plan: Planner 重新规划
			`[{"id": "t3", "title": "New Task", "description": "Better approach"}]`,
			// Generator t3
			`{"result": "Good result with new plan", "updated_artifact_data": {"v": 1}}`,
			// Evaluator t3 → pass
			`{"score": 88, "feedback": "Good", "passed": true}`,
		},
	}

	orch := buildOrchestrator(mock)
	onEvent, events := collectEvents(t)

	session, err := orch.ExecuteSession(context.Background(), "Difficult task", onEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, string(model.SessionCompleted), string(session.Status), "session status")
	assertEqual(t, 1, session.RePlanCount, "re-plan count")

	// Re-plan 后任务列表应该是新的
	assertEqual(t, "New Task", session.Tasks[0].Title, "re-planned task title")
	assertEqual(t, string(model.TaskStatusCompleted), string(session.Tasks[0].Status), "re-planned task status")

	// 验证有 re_plan 事件
	hasRePlan := false
	for _, e := range *events {
		if e.Type == model.EventRePlan {
			hasRePlan = true
			break
		}
	}
	if !hasRePlan {
		t.Error("expected re_plan event")
	}

	t.Log("✅ RePlan passed")
}

// ==================== Test 4: 断点恢复 ====================

func TestHarness_ResumeSession(t *testing.T) {
	mock := &MockProvider{
		responses: []string{
			// Planner
			`[{"id": "t1", "title": "Task 1", "description": "First"}, {"id": "t2", "title": "Task 2", "description": "Second"}]`,
			// Generator t1
			`{"result": "Result 1", "updated_artifact_data": {"step": 1}}`,
			// Evaluator t1 → pass
			`{"score": 85, "feedback": "OK", "passed": true}`,
			// Generator t2 (this will be used on resume)
			`{"result": "Result 2", "updated_artifact_data": {"step": 2}}`,
			// Evaluator t2 → pass
			`{"score": 80, "feedback": "OK", "passed": true}`,
		},
	}

	orch := buildOrchestrator(mock)
	onEvent, _ := collectEvents(t)

	// 先执行，让它完成 t1
	session, err := orch.ExecuteSession(context.Background(), "Two step task", onEvent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, string(model.SessionCompleted), string(session.Status), "initial status")
	assertEqual(t, 2, session.Metrics.CompletedTasks, "completed")

	// Resume 一个已完成的会话，应该直接返回
	resumed, err := orch.ResumeSession(context.Background(), session.ID, onEvent)
	if err != nil {
		t.Fatalf("resume error: %v", err)
	}
	assertEqual(t, string(model.SessionCompleted), string(resumed.Status), "resumed status")

	t.Log("✅ ResumeSession passed")
}

// ==================== Test 5: Session ID 唯一性 ====================

func TestHarness_UniqueSessionID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		s := model.NewSession("test")
		if ids[s.ID] {
			t.Fatalf("duplicate session ID: %s", s.ID)
		}
		ids[s.ID] = true
	}
	t.Log("✅ UniqueSessionID passed (100 unique IDs)")
}

// ==================== Test 6: 超时保护 ====================

func TestHarness_TaskTimeout(t *testing.T) {
	// 模拟一个永远不返回的 Provider
	slowMock := &MockProvider{
		responses: []string{
			// Planner 正常返回
			`[{"id": "t1", "title": "Slow Task", "description": "This will timeout"}]`,
			// Generator 永远不会被调用到（因为 context 会超时）
		},
	}

	config := DefaultConfig()
	config.TaskTimeout = 100 * time.Millisecond // 100ms 超时

	orch := NewHarnessOrchestratorWithConfig(
		agent.NewPlannerAgent(slowMock),
		agent.NewGeneratorAgent(&TimeoutProvider{}), // 用一个会阻塞的 provider
		agent.NewEvaluatorAgent(slowMock),
		NewInMemSessionStore(),
		config,
	)

	onEvent, _ := collectEvents(t)
	session, _ := orch.ExecuteSession(context.Background(), "Timeout test", onEvent)

	// 任务应该失败
	if len(session.Tasks) > 0 {
		assertEqual(t, string(model.TaskStatusFailed), string(session.Tasks[0].Status), "timeout task status")
	}

	t.Log("✅ TaskTimeout passed")
}

// TimeoutProvider 模拟超时的 Provider
type TimeoutProvider struct{}

func (p *TimeoutProvider) Name() string { return "timeout" }
func (p *TimeoutProvider) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("should not reach here")
	}
}
func (p *TimeoutProvider) StreamChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamChunk, <-chan error) {
	return nil, nil
}

// ==================== Helpers ====================

func buildOrchestrator(mock *MockProvider) *HarnessOrchestrator {
	return NewHarnessOrchestrator(
		agent.NewPlannerAgent(mock),
		agent.NewGeneratorAgent(mock),
		agent.NewEvaluatorAgent(mock),
		NewInMemSessionStore(),
	)
}

func assertEqual(t *testing.T, expected, actual interface{}, label string) {
	t.Helper()
	if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
		t.Errorf("%s: expected [%v], got [%v]", label, expected, actual)
	}
}
