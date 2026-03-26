package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/model"
	"github.com/agentic-learning/gateway/internal/orchestrator"
	"github.com/agentic-learning/gateway/internal/provider"
)

// HarnessHandler 处理 Agent Harness 请求
type HarnessHandler struct {
	registry   *provider.Registry
	store      orchestrator.SessionStore
	logDir     string
	handoffDir string

	// 缓存活跃的 orchestrator 实例，使得 Resume 能复用
	// 同一 session 的 middleware/budget/circuit-breaker 状态
	mu            sync.Mutex
	orchestrators map[string]*orchestrator.HarnessOrchestrator
}

func NewHarnessHandler(registry *provider.Registry, store orchestrator.SessionStore) *HarnessHandler {
	return &HarnessHandler{
		registry:      registry,
		store:         store,
		orchestrators: make(map[string]*orchestrator.HarnessOrchestrator),
	}
}

func NewHarnessHandlerWithDirs(registry *provider.Registry, store orchestrator.SessionStore, logDir, handoffDir string) *HarnessHandler {
	return &HarnessHandler{
		registry:      registry,
		store:         store,
		logDir:        logDir,
		handoffDir:    handoffDir,
		orchestrators: make(map[string]*orchestrator.HarnessOrchestrator),
	}
}

func NewHarnessHandlerWithLogDir(registry *provider.Registry, store orchestrator.SessionStore, logDir string) *HarnessHandler {
	return &HarnessHandler{
		registry:      registry,
		store:         store,
		logDir:        logDir,
		handoffDir:    logDir + "/handoffs",
		orchestrators: make(map[string]*orchestrator.HarnessOrchestrator),
	}
}

// ==================== 请求结构 ====================

type HarnessRequest struct {
	Goal     string `json:"goal" binding:"required"`
	Provider string `json:"provider"`
}

type ResumeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Provider  string `json:"provider"`
}

// ==================== 内部方法 ====================

func (h *HarnessHandler) buildConfig() orchestrator.HarnessConfig {
	config := orchestrator.DefaultConfig()
	if h.logDir != "" {
		config.LogDir = h.logDir
	}
	if h.handoffDir != "" {
		config.HandoffDir = h.handoffDir
	}
	return config
}

func (h *HarnessHandler) newOrchestrator(p provider.Provider, config orchestrator.HarnessConfig) *orchestrator.HarnessOrchestrator {
	planner := agent.NewPlannerAgent(p)
	generator := agent.NewGeneratorAgent(p)
	evaluator := agent.NewEvaluatorAgent(p)
	return orchestrator.NewHarnessOrchestratorWithConfig(planner, generator, evaluator, h.store, config)
}

func (h *HarnessHandler) cacheOrchestrator(sessionID string, orch *orchestrator.HarnessOrchestrator) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.orchestrators[sessionID] = orch
}

func (h *HarnessHandler) getOrchestrator(sessionID string) (*orchestrator.HarnessOrchestrator, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	orch, ok := h.orchestrators[sessionID]
	return orch, ok
}

func (h *HarnessHandler) removeOrchestrator(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.orchestrators, sessionID)
}

// ==================== POST /v1/harness/run — 执行新会话 ====================

func (h *HarnessHandler) Handle(c *gin.Context) {
	var req HarnessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.NewErrorResponse(err.Error(), "invalid_request", "bad_request"))
		return
	}

	p, ok := h.registry.Get(req.Provider)
	if !ok {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("Provider not found", "not_found", "provider_not_found"))
		return
	}

	config := h.buildConfig()
	orch := h.newOrchestrator(p, config)

	h.streamSSE(c, func(onEvent orchestrator.EventCallback) error {
		session, err := orch.ExecuteSession(c.Request.Context(), req.Goal, onEvent)
		if session != nil {
			h.cacheOrchestrator(session.ID, orch)
		}
		return err
	})
}

// ==================== POST /v1/harness/resume — 恢复中断会话 ====================

func (h *HarnessHandler) HandleResume(c *gin.Context) {
	var req ResumeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.NewErrorResponse(err.Error(), "invalid_request", "bad_request"))
		return
	}

	p, ok := h.registry.Get(req.Provider)
	if !ok {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("Provider not found", "not_found", "provider_not_found"))
		return
	}

	orch, found := h.getOrchestrator(req.SessionID)
	if !found {
		config := h.buildConfig()
		orch = h.newOrchestrator(p, config)
		h.cacheOrchestrator(req.SessionID, orch)
	}

	h.streamSSE(c, func(onEvent orchestrator.EventCallback) error {
		_, err := orch.ResumeSession(c.Request.Context(), req.SessionID, onEvent)
		return err
	})
}

// ==================== GET /v1/harness/session/:id — 查询单个会话 ====================

func (h *HarnessHandler) HandleGetSession(c *gin.Context) {
	sessionID := c.Param("id")
	session, ok := h.store.Get(sessionID)
	if !ok {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("Session not found", "not_found", "session_not_found"))
		return
	}
	c.JSON(http.StatusOK, session)
}

// ==================== GET /v1/harness/sessions — 查询会话列表 ====================

// SessionSummary 会话摘要（用于列表展示，不返回完整任务详情）
type SessionSummary struct {
	ID        string               `json:"id"`
	Goal      string               `json:"goal"`
	Status    model.SessionStatus  `json:"status"`
	Metrics   model.SessionMetrics `json:"metrics"`
	CreatedAt string               `json:"created_at"`
	UpdatedAt string               `json:"updated_at"`
}

func (h *HarnessHandler) HandleListSessions(c *gin.Context) {
	sessions := h.store.List()

	summaries := make([]SessionSummary, 0, len(sessions))
	for _, s := range sessions {
		summaries = append(summaries, SessionSummary{
			ID:        s.ID,
			Goal:      s.Goal,
			Status:    s.Status,
			Metrics:   s.Metrics,
			CreatedAt: s.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt: s.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": summaries,
		"total":    len(summaries),
	})
}

// ==================== SSE 工具方法 ====================

func (h *HarnessHandler) streamSSE(c *gin.Context, execute func(onEvent orchestrator.EventCallback) error) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeaderNow()

	flusher, _ := c.Writer.(http.Flusher)

	onEvent := func(event model.HarnessEvent) {
		data, err := json.Marshal(event)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	if err := execute(onEvent); err != nil {
		errEvent := model.HarnessEvent{
			Type:    model.EventError,
			Message: err.Error(),
		}
		data, _ := json.Marshal(errEvent)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}

	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}
