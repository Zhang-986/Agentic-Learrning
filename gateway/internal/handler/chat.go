package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/agentic-learning/gateway/internal/model"
	"github.com/agentic-learning/gateway/internal/provider"
)

// ChatHandler 聊天补全 Handler
type ChatHandler struct {
	registry *provider.Registry
}

// NewChatHandler 创建 ChatHandler
func NewChatHandler(registry *provider.Registry) *ChatHandler {
	return &ChatHandler{registry: registry}
}

// Handle 处理 /v1/chat/completions 请求
func (h *ChatHandler) Handle(c *gin.Context) {
	var req model.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.NewErrorResponse(
			"请求参数格式错误: "+err.Error(),
			"invalid_request_error",
			"invalid_request",
		))
		return
	}

	// 获取默认 Provider（智谱 AI）
	p, ok := h.registry.Get("")
	if !ok {
		c.JSON(http.StatusInternalServerError, model.NewErrorResponse(
			"Provider 未配置",
			"server_error",
			"provider_not_found",
		))
		return
	}

	// 强制使用流式输出
	req.Stream = true
	h.handleStream(c, p, &req)
}

// handleStream SSE 流式处理
//
// 修复: 原实现在 chunkCh 关闭后直接 return，可能丢失 errCh 中的错误。
// 现在改为先消费完 chunkCh，再检查 errCh。
func (h *ChatHandler) handleStream(c *gin.Context, p provider.Provider, req *model.ChatCompletionRequest) {
	chunkCh, errCh := p.StreamChatCompletion(c.Request.Context(), req)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	c.Writer.WriteHeaderNow()
	flusher, _ := c.Writer.(http.Flusher)

	flush := func() {
		if flusher != nil {
			flusher.Flush()
		}
	}

	// 消费所有 chunk
	for chunk := range chunkCh {
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flush()
	}

	// chunkCh 已关闭，检查是否有错误
	// errCh 由 provider goroutine defer close()，所以这里安全读取
	if err, ok := <-errCh; ok && err != nil {
		errResp, _ := json.Marshal(model.NewErrorResponse(
			err.Error(), "api_error", "stream_error",
		))
		fmt.Fprintf(c.Writer, "data: %s\n\n", errResp)
		flush()
		return
	}

	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flush()
}
