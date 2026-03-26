package handler

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/model"
	"github.com/agentic-learning/gateway/internal/orchestrator"
	"github.com/agentic-learning/gateway/internal/provider"
)

// HarnessHandler 处理长运行任务请求
type HarnessHandler struct {
	registry *provider.Registry
	store    orchestrator.SessionStore
}

func NewHarnessHandler(registry *provider.Registry, store orchestrator.SessionStore) *HarnessHandler {
	return &HarnessHandler{registry: registry, store: store}
}

// HarnessRequest 运行任务请求
type HarnessRequest struct {
	Goal     string `json:"goal" binding:"required"`
	Provider string `json:"provider"`
}

// Handle 处理请求
func (h *HarnessHandler) Handle(c *gin.Context) {
	var req HarnessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.NewErrorResponse(err.Error(), "invalid_request", "bad_request"))
		return
	}

	// 获取指定的 Provider
	p, ok := h.registry.Get(req.Provider)
	if !ok {
		c.JSON(http.StatusNotFound, model.NewErrorResponse("Provider not found", "not_found", "provider_not_found"))
		return
	}

	// 初始化角色和编排器
	planner := agent.NewPlannerAgent(p)
	generator := agent.NewGeneratorAgent(p)
	evaluator := agent.NewEvaluatorAgent(p)
	orch := orchestrator.NewHarnessOrchestrator(planner, generator, evaluator, h.store)

	// 设置 SSE Headers
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")

	// 创建一个 channel 用于接收事件
	eventCh := make(chan model.HarnessEvent, 10)
	doneCh := make(chan struct{})

	// 异步执行会话
	go func() {
		defer close(eventCh)
		defer close(doneCh)
		_, err := orch.ExecuteSession(c.Request.Context(), req.Goal, func(event model.HarnessEvent) {
			eventCh <- event
		})
		if err != nil {
			eventCh <- model.HarnessEvent{
				Type:    model.EventError,
				Message: err.Error(),
			}
		}
	}()

	// 推送 SSE 数据
	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return false // channel 关闭，结束流
			}
			c.SSEvent(string(event.Type), event)
			return true
		case <-c.Request.Context().Done():
			return false // 客户端断开连接
		}
	})
}
