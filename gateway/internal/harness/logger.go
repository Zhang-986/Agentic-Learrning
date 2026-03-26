package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ==================== 持久化日志层 ====================
//
// 对标 Philschmid / nxcode.io 的结构化日志要求：
// - 每次 session 的完整执行轨迹结构化存储
// - 支持事后回放（replay）、评分（grading）、对比分析
// - 每次 LLM 调用记录 input tokens / output tokens / latency / model version

// SessionLog 完整的 session 执行日志
type SessionLog struct {
	SessionID   string                 `json:"session_id"`
	Goal        string                 `json:"goal"`
	StartTime   time.Time              `json:"start_time"`
	EndTime     time.Time              `json:"end_time,omitempty"`
	FinalStatus string                 `json:"final_status"`
	LLMCalls    []LLMCallRecord        `json:"llm_calls"`
	Progress    []ProgressEntry        `json:"progress"`
	Budget      *BudgetStatus          `json:"budget,omitempty"`
	CallStats   map[string]interface{} `json:"call_stats,omitempty"`
	Events      []LoggedEvent          `json:"events"`
}

// LoggedEvent 结构化事件日志
type LoggedEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

// SessionLogger 会话级日志记录器
type SessionLogger struct {
	mu        sync.Mutex
	sessionID string
	goal      string
	startTime time.Time
	events    []LoggedEvent
	logDir    string
}

// NewSessionLogger 创建日志记录器
// logDir: 日志存储目录（如 "./logs/sessions"），空字符串则不持久化
func NewSessionLogger(sessionID, goal, logDir string) *SessionLogger {
	return &SessionLogger{
		sessionID: sessionID,
		goal:      goal,
		startTime: time.Now(),
		logDir:    logDir,
	}
}

// LogEvent 记录事件
func (l *SessionLogger) LogEvent(eventType, message string, data interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, LoggedEvent{
		Timestamp: time.Now(),
		Type:      eventType,
		Message:   message,
		Data:      data,
	})
}

// Finalize 完成日志并持久化
func (l *SessionLogger) Finalize(
	finalStatus string,
	llmCalls []LLMCallRecord,
	progress []ProgressEntry,
	budget *BudgetStatus,
	callStats map[string]interface{},
) (*SessionLog, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	log := &SessionLog{
		SessionID:   l.sessionID,
		Goal:        l.goal,
		StartTime:   l.startTime,
		EndTime:     time.Now(),
		FinalStatus: finalStatus,
		LLMCalls:    llmCalls,
		Progress:    progress,
		Budget:      budget,
		CallStats:   callStats,
		Events:      l.events,
	}

	// 持久化到文件
	if l.logDir != "" {
		if err := l.persist(log); err != nil {
			return log, fmt.Errorf("日志持久化失败: %w", err)
		}
	}

	return log, nil
}

func (l *SessionLogger) persist(log *SessionLog) error {
	if err := os.MkdirAll(l.logDir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s_%s.json",
		l.sessionID,
		l.startTime.Format("20060102_150405"),
	)
	path := filepath.Join(l.logDir, filename)

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetEvents 获取当前事件列表
func (l *SessionLogger) GetEvents() []LoggedEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	cpy := make([]LoggedEvent, len(l.events))
	copy(cpy, l.events)
	return cpy
}
