package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentic-learning/gateway/internal/config"
	"github.com/agentic-learning/gateway/internal/model"
)

// ZhipuProvider 智谱 AI 适配器（OpenAI-compatible API）
type ZhipuProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

// NewZhipuProvider 创建智谱 AI Provider
func NewZhipuProvider(cfg config.ProviderConfig) *ZhipuProvider {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://open.bigmodel.cn/api/paas/v4"
	}
	return &ZhipuProvider{
		apiKey:  cfg.APIKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   cfg.DefaultModel,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
	}
}

func (p *ZhipuProvider) Name() string {
	return "zhipu"
}

// ChatCompletion 非流式请求
func (p *ZhipuProvider) ChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (*model.ChatCompletionResponse, error) {
	if req.Model == "" {
		req.Model = p.model
	}
	req.Stream = false

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	p.setHeaders(httpReq)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("请求智谱 AI 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("智谱 AI 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var result model.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	return &result, nil
}

// StreamChatCompletion 流式请求（SSE）
func (p *ZhipuProvider) StreamChatCompletion(ctx context.Context, req *model.ChatCompletionRequest) (<-chan *model.ChatCompletionStreamChunk, <-chan error) {
	chunkCh := make(chan *model.ChatCompletionStreamChunk, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(chunkCh)
		defer close(errCh)

		if req.Model == "" {
			req.Model = p.model
		}
		req.Stream = true

		body, err := json.Marshal(req)
		if err != nil {
			errCh <- fmt.Errorf("序列化请求失败: %w", err)
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			errCh <- fmt.Errorf("创建请求失败: %w", err)
			return
		}
		p.setHeaders(httpReq)

		// 流式请求不设超时，由 context 控制
		streamClient := &http.Client{}
		resp, err := streamClient.Do(httpReq)
		if err != nil {
			errCh <- fmt.Errorf("请求智谱 AI 失败: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("智谱 AI 返回错误 (HTTP %d): %s", resp.StatusCode, string(respBody))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return
			}

			var chunk model.ChatCompletionStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}

			select {
			case chunkCh <- &chunk:
			case <-ctx.Done():
				return
			}
		}
	}()

	return chunkCh, errCh
}

func (p *ZhipuProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}
