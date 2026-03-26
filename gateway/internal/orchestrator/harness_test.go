package orchestrator

import (
	"context"
	"fmt"
	"testing"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/model"
)

// MockProvider 模拟 AI 服务提供商，用于控制输出场景
type MockProvider struct {
	responses []string
	callCount int
}

func (p *MockProvider) Name() string { return "mock" }

func (p *MockProvider) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	if p.callCount >= len(p.responses) {
		return nil, fmt.Errorf("no more mock responses")
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

func TestHarnessOrchestrator_Resilience(t *testing.T) {
	// 场景设计：
	// 1. Planner 返回 1 个子任务。
	// 2. Generator 第一次执行任务。
	// 3. Evaluator 觉得不行 (Passed: false)，触发重试。
	// 4. Generator 第二次执行任务。
	// 5. Evaluator 觉得行了 (Passed: true)。

	plannerResp := `[{"id": "t1", "title": "Test Task", "description": "Doing something"}]`
	gen1Resp := `{"result": "Initial poor result", "updated_artifact_data": {"count": 1}}`
	eval1Resp := `{"score": 40, "feedback": "Too shallow", "passed": false}`
	gen2Resp := `{"result": "Better result after feedback", "updated_artifact_data": {"count": 2}}`
	eval2Resp := `{"score": 90, "feedback": "Excellent", "passed": true}`

	mock := &MockProvider{
		responses: []string{plannerResp, gen1Resp, eval1Resp, gen2Resp, eval2Resp},
	}

	planner := agent.NewPlannerAgent(mock)
	generator := agent.NewGeneratorAgent(mock)
	evaluator := agent.NewEvaluatorAgent(mock)
	store := NewInMemSessionStore()
	orch := NewHarnessOrchestrator(planner, generator, evaluator, store)

	events := make([]model.HarnessEvent, 0)
	onEvent := func(e model.HarnessEvent) {
		events = append(events, e)
		t.Logf("[EVENT] Type: %s, Message: %s", e.Type, e.Message)
	}

	ctx := context.Background()
	session, err := orch.ExecuteSession(ctx, "Test Goal", onEvent)

	if err != nil {
		t.Fatalf("ExecuteSession failed: %v", err)
	}

	// 验证最终状态
	if session.Status != "completed" {
		t.Errorf("Expected status completed, got %s", session.Status)
	}

	// 验证任务是否成功完成
	if session.Tasks[0].Status != model.TaskStatusCompleted {
		t.Errorf("Task status should be completed, got %s", session.Tasks[0].Status)
	}

	// 验证结果是否为第二次生成的结果
	if session.Tasks[0].Result != "Better result after feedback" {
		t.Errorf("Unexpected result: %s", session.Tasks[0].Result)
	}

	// 验证持久化是否生效
	savedSess, ok := store.Get(session.ID)
	if !ok || savedSess.Status != "completed" {
		t.Error("Session should be correctly persisted in store")
	}

	// 验证重试逻辑：总共应该调用了 5 次 Mock Provider (1 Planner + 2 Gen + 2 Eval)
	if mock.callCount != 5 {
		t.Errorf("Expected 5 calls to provider, got %d", mock.callCount)
	}

	t.Log("Harness resilience test passed successfully!")
}
