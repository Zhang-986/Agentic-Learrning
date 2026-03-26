package model

import "time"

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// SubTask 由 Planner 生成的子任务
type SubTask struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Result      string     `json:"result,omitempty"`
}

// EvaluationResult Evaluator 的评估结果
type EvaluationResult struct {
	Score    int    `json:"score"`    // 0-100
	Feedback string `json:"feedback"` // 改进建议
	Passed   bool   `json:"passed"`   // 是否通过
}

// Artifact 状态工件，用于上下文重置 (Context Reset) 时的状态传递
type Artifact struct {
	SessionID   string                 `json:"session_id"`
	Data        map[string]interface{} `json:"data"`
	LastUpdated time.Time              `json:"last_updated"`
}

// HarnessSession 长运行 Agent 会话
type HarnessSession struct {
	ID        string    `json:"id"`
	Goal      string    `json:"goal"`     // 最终目标
	Tasks     []SubTask `json:"tasks"`    // Planner 拆解的任务列表
	Artifact  Artifact  `json:"artifact"` // 当前状态工件
	Status    string    `json:"status"`   // 会话总体状态
	CreatedAt time.Time `json:"created_at"`
}

// EventType 编排器事件类型
type EventType string

const (
	EventInfo            EventType = "info"
	EventTaskStart       EventType = "task_start"
	EventTaskEval        EventType = "task_eval"
	EventTaskComplete    EventType = "task_complete"
	EventSessionComplete EventType = "session_complete"
	EventError           EventType = "error"
)

// HarnessEvent 长运行任务的 SSE 推送事件
type HarnessEvent struct {
	Type    EventType   `json:"type"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
