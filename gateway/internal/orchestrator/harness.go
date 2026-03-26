package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/model"
)

// EventCallback 用于向外发射状态事件的回调函数
type EventCallback func(event model.HarnessEvent)

// HarnessOrchestrator Agent 任务编排器
type HarnessOrchestrator struct {
	planner   *agent.PlannerAgent
	generator *agent.GeneratorAgent
	evaluator *agent.EvaluatorAgent
	store     SessionStore
}

func NewHarnessOrchestrator(planner *agent.PlannerAgent, generator *agent.GeneratorAgent, evaluator *agent.EvaluatorAgent, store SessionStore) *HarnessOrchestrator {
	return &HarnessOrchestrator{
		planner:   planner,
		generator: generator,
		evaluator: evaluator,
		store:     store,
	}
}

// ExecuteSession 执行一个完整的 Agent 会话，支持通过 onEvent 实时推送执行状态
func (o *HarnessOrchestrator) ExecuteSession(ctx context.Context, goal string, onEvent EventCallback) (*model.HarnessSession, error) {
	if onEvent == nil {
		onEvent = func(event model.HarnessEvent) {}
	}

	onEvent(model.HarnessEvent{Type: model.EventInfo, Message: "会话初始化中..."})

	session := &model.HarnessSession{
		ID:        fmt.Sprintf("sess_%d", time.Now().Unix()),
		Goal:      goal,
		Status:    "initializing",
		CreatedAt: time.Now(),
		Artifact: model.Artifact{
			SessionID:   "",
			Data:        make(map[string]interface{}),
			LastUpdated: time.Now(),
		},
	}
	session.Artifact.SessionID = session.ID
	o.store.Save(session)

	onEvent(model.HarnessEvent{Type: model.EventInfo, Message: "Planner 正在拆解任务..."})

	tasks, err := o.planner.Plan(ctx, goal)
	if err != nil {
		onEvent(model.HarnessEvent{Type: model.EventError, Message: fmt.Sprintf("任务规划失败: %v", err)})
		return nil, fmt.Errorf("任务规划失败: %w", err)
	}
	session.Tasks = tasks
	session.Status = "running"
	o.store.Save(session)

	onEvent(model.HarnessEvent{
		Type:    model.EventInfo,
		Message: fmt.Sprintf("Planner 拆解完成，共生成 %d 个子任务", len(tasks)),
		Data:    tasks,
	})

	return o.runRemainingTasks(ctx, session, onEvent)
}

// ResumeSession 恢复一个现有的会话 (Long-Running Resilience)
func (o *HarnessOrchestrator) ResumeSession(ctx context.Context, sessionID string, onEvent EventCallback) (*model.HarnessSession, error) {
	session, ok := o.store.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status == "completed" || session.Status == "failed" {
		return session, nil
	}

	onEvent(model.HarnessEvent{Type: model.EventInfo, Message: fmt.Sprintf("正在恢复会话 [%s]...", sessionID)})
	return o.runRemainingTasks(ctx, session, onEvent)
}

// runRemainingTasks 核心循环：执行剩余任务并持久化状态
func (o *HarnessOrchestrator) runRemainingTasks(ctx context.Context, session *model.HarnessSession, onEvent EventCallback) (*model.HarnessSession, error) {
	for i := range session.Tasks {
		task := &session.Tasks[i]
		if task.Status == model.TaskStatusCompleted {
			continue // 跳过已完成任务
		}

		task.Status = model.TaskStatusRunning
		o.store.Save(session)

		onEvent(model.HarnessEvent{
			Type:    model.EventTaskStart,
			Message: fmt.Sprintf("开始执行子任务 [%s]: %s", task.ID, task.Title),
			Data:    task,
		})

		maxRetries := 2
		for retry := 0; retry <= maxRetries; retry++ {
			if retry > 0 {
				onEvent(model.HarnessEvent{Type: model.EventInfo, Message: fmt.Sprintf("子任务 [%s] 第 %d 次重试...", task.ID, retry)})
			}

			result, updatedArtifact, err := o.generator.Execute(ctx, *task, session.Artifact)
			if err != nil {
				task.Status = model.TaskStatusFailed
				o.store.Save(session)
				onEvent(model.HarnessEvent{Type: model.EventError, Message: fmt.Sprintf("执行子任务 [%s] 失败: %v", task.ID, err)})
				return session, fmt.Errorf("执行子任务 [%s] 失败: %w", task.ID, err)
			}

			onEvent(model.HarnessEvent{Type: model.EventInfo, Message: fmt.Sprintf("子任务 [%s] 执行完成，开始 Evaluator 评估...", task.ID)})

			eval, err := o.evaluator.Evaluate(ctx, *task, result)
			if err != nil {
				onEvent(model.HarnessEvent{Type: model.EventInfo, Message: fmt.Sprintf("评估请求失败，跳过质检: %v", err)})
				task.Result = result
				session.Artifact = updatedArtifact
				task.Status = model.TaskStatusCompleted
				break
			}

			onEvent(model.HarnessEvent{
				Type:    model.EventTaskEval,
				Message: fmt.Sprintf("子任务 [%s] 评估结果: 分数=%d, 通过=%v", task.ID, eval.Score, eval.Passed),
				Data:    eval,
			})

			if eval.Passed {
				task.Result = result
				session.Artifact = updatedArtifact
				task.Status = model.TaskStatusCompleted
				break
			} else {
				if retry < maxRetries {
					task.Description += fmt.Sprintf("\n[质检反馈]: %s", eval.Feedback)
					continue
				} else {
					task.Result = result + "\n(未通过质检: " + eval.Feedback + ")"
					session.Artifact = updatedArtifact
					task.Status = model.TaskStatusFailed
					break
				}
			}
		}

		// 每次子任务完成，强制持久化一次
		o.store.Save(session)
		onEvent(model.HarnessEvent{
			Type:    model.EventTaskComplete,
			Message: fmt.Sprintf("子任务 [%s] 结束，最终状态: %s", task.ID, task.Status),
			Data:    task,
		})
	}

	session.Status = "completed"
	o.store.Save(session)
	onEvent(model.HarnessEvent{
		Type:    model.EventSessionComplete,
		Message: "整个会话执行完毕！",
		Data:    session,
	})

	return session, nil
}
