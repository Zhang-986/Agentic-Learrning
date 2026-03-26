package model

import (
	"fmt"
	"sync/atomic"
	"time"
)

// ==================== 状态枚举 ====================

// TaskStatus 子任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped" // Re-plan 后被替换掉的旧任务
)

// SessionStatus 会话状态
type SessionStatus string

const (
	SessionInitializing SessionStatus = "initializing"
	SessionPlanning     SessionStatus = "planning"
	SessionRunning      SessionStatus = "running"
	SessionRePlanning   SessionStatus = "re_planning"
	SessionCompleted    SessionStatus = "completed"
	SessionFailed       SessionStatus = "failed"
)

// ==================== 核心数据结构 ====================

// SubTask 由 Planner 生成的子任务
type SubTask struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	Result      string     `json:"result,omitempty"`

	// 执行度量
	Attempts   int           `json:"attempts"`              // 已执行次数（含重试）
	LatencyMs  int64         `json:"latency_ms,omitempty"`  // 最后一次执行耗时（毫秒）
	StartedAt  *time.Time    `json:"started_at,omitempty"`
	FinishedAt *time.Time    `json:"finished_at,omitempty"`
	EvalHistory []EvaluationResult `json:"eval_history,omitempty"` // 所有评估记录
}

// EvaluationResult Evaluator 的评估结果
type EvaluationResult struct {
	Score    int    `json:"score"`
	Feedback string `json:"feedback"`
	Passed   bool   `json:"passed"`
	Attempt  int    `json:"attempt"` // 第几次尝试的评估
}

// Artifact 状态工件
type Artifact struct {
	SessionID   string                 `json:"session_id"`
	Data        map[string]interface{} `json:"data"`
	LastUpdated time.Time              `json:"last_updated"`
}

// ArtifactSnapshot Artifact 的历史快照
type ArtifactSnapshot struct {
	TaskID    string                 `json:"task_id"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// SessionMetrics 会话级度量
type SessionMetrics struct {
	TotalTasks     int   `json:"total_tasks"`
	CompletedTasks int   `json:"completed_tasks"`
	FailedTasks    int   `json:"failed_tasks"`
	TotalRetries   int   `json:"total_retries"`
	RePlanCount    int   `json:"re_plan_count"`
	TotalLatencyMs int64 `json:"total_latency_ms"`
}

// HarnessSession 长运行 Agent 会话
type HarnessSession struct {
	ID        string        `json:"id"`
	Goal      string        `json:"goal"`
	Tasks     []SubTask     `json:"tasks"`
	Artifact  Artifact      `json:"artifact"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`

	// 可观测性
	Metrics          SessionMetrics     `json:"metrics"`
	ArtifactHistory  []ArtifactSnapshot `json:"artifact_history,omitempty"`
	RePlanCount      int                `json:"re_plan_count"`
	MaxRePlans       int                `json:"max_re_plans"`
}

// sessionCounter 全局递增计数器，确保 ID 唯一
var sessionCounter atomic.Int64

// NewSession 创建新会话（唯一 ID）
func NewSession(goal string) *HarnessSession {
	now := time.Now()
	id := fmt.Sprintf("sess_%d_%d", now.UnixNano(), sessionCounter.Add(1))
	return &HarnessSession{
		ID:   id,
		Goal: goal,
		Status: SessionInitializing,
		CreatedAt: now,
		UpdatedAt: now,
		Artifact: Artifact{
			SessionID:   id,
			Data:        make(map[string]interface{}),
			LastUpdated: now,
		},
		MaxRePlans: 2, // 最多重新规划 2 次
	}
}

// SnapshotArtifact 保存当前 Artifact 快照
func (s *HarnessSession) SnapshotArtifact(taskID string) {
	snapshot := ArtifactSnapshot{
		TaskID:    taskID,
		Timestamp: time.Now(),
		Data:      make(map[string]interface{}),
	}
	for k, v := range s.Artifact.Data {
		snapshot.Data[k] = v
	}
	s.ArtifactHistory = append(s.ArtifactHistory, snapshot)
}

// UpdateMetrics 根据当前 Tasks 刷新度量
func (s *HarnessSession) UpdateMetrics() {
	m := SessionMetrics{
		TotalTasks: len(s.Tasks),
		RePlanCount: s.RePlanCount,
	}
	for _, t := range s.Tasks {
		switch t.Status {
		case TaskStatusCompleted:
			m.CompletedTasks++
		case TaskStatusFailed:
			m.FailedTasks++
		}
		if t.Attempts > 1 {
			m.TotalRetries += t.Attempts - 1
		}
		m.TotalLatencyMs += t.LatencyMs
	}
	s.Metrics = m
}

// ==================== 事件 ====================

type EventType string

const (
	EventInfo            EventType = "info"
	EventTaskStart       EventType = "task_start"
	EventTaskEval        EventType = "task_eval"
	EventTaskComplete    EventType = "task_complete"
	EventRePlan          EventType = "re_plan"
	EventSessionComplete EventType = "session_complete"
	EventError           EventType = "error"
	EventMetrics         EventType = "metrics"
)

type HarnessEvent struct {
	Type    EventType   `json:"type"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
