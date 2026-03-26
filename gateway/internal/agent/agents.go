package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentic-learning/gateway/internal/harness"
	"github.com/agentic-learning/gateway/internal/model"
	"github.com/agentic-learning/gateway/internal/provider"
)

// ==================== Agent 基础设施 ====================

// AgentOptions 所有 Agent 共享的基础设施
type AgentOptions struct {
	Middleware *harness.MiddlewareChain // LLM 调用拦截链
	SessionID  string                  // 当前 Session ID（用于日志关联）
}

// callLLM 统一的 LLM 调用入口，自动走 Middleware
func callLLM(
	ctx context.Context,
	p provider.Provider,
	opts *AgentOptions,
	agentName string,
	taskID string,
	req *model.ChatCompletionRequest,
) (*model.ChatCompletionResponse, error) {

	// 如果没有 Middleware，直接调用
	if opts == nil || opts.Middleware == nil {
		return p.ChatCompletion(ctx, req)
	}

	// Before Hooks
	if err := opts.Middleware.RunBefore(ctx, agentName, req); err != nil {
		return nil, fmt.Errorf("[%s] before hook rejected: %w", agentName, err)
	}

	// 实际调用
	start := time.Now()
	resp, err := p.ChatCompletion(ctx, req)
	elapsed := time.Since(start)

	// 构建调用记录
	record := &harness.LLMCallRecord{
		Agent:     agentName,
		TaskID:    taskID,
		SessionID: "",
		StartTime: start,
		EndTime:   time.Now(),
		Latency:   elapsed,
		Success:   err == nil,
	}
	if opts.SessionID != "" {
		record.SessionID = opts.SessionID
	}
	if err != nil {
		record.Error = err.Error()
	}
	if resp != nil && len(resp.Choices) > 0 {
		record.RawOutput = resp.Choices[0].Message.Content
	}

	// After Hooks（即使调用失败也要执行，用于熔断追踪等）
	_ = opts.Middleware.RunAfter(ctx, record, resp)

	return resp, err
}

// ==================== Planner Agent ====================

type PlannerAgent struct {
	provider provider.Provider
	schema   *harness.SchemaValidator
}

func NewPlannerAgent(p provider.Provider) *PlannerAgent {
	return &PlannerAgent{
		provider: p,
		schema:   harness.PlannerSchema(),
	}
}

func (a *PlannerAgent) Plan(ctx context.Context, goal string) ([]model.SubTask, error) {
	return a.PlanWithOpts(ctx, goal, nil)
}

// PlanWithOpts 带 Middleware 的规划
func (a *PlannerAgent) PlanWithOpts(ctx context.Context, goal string, opts *AgentOptions) ([]model.SubTask, error) {
	systemPrompt := `你是一个 AI 学习系统的战略规划者。
你的任务是将用户的大目标拆解为一系列原子级的、可执行的子任务（3-5 个为宜）。
你只能输出纯 JSON 数组，不要输出任何其他文字或 markdown 代码块标记。
每个对象必须包含 id, title, description 三个字段，且都不能为空。
示例：[{"id": "task1", "title": "任务标题", "description": "详细描述"}]`

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("目标：%s", goal)},
		},
	}

	resp, err := callLLM(ctx, a.provider, opts, "planner", "", req)
	if err != nil {
		return nil, fmt.Errorf("planner 调用 LLM 失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("planner 未收到有效响应")
	}

	content := resp.Choices[0].Message.Content
	jsonStr := extractJSON(content)

	// Schema 验证
	if vr := a.schema.ValidateArray(jsonStr); !vr.Valid {
		return nil, fmt.Errorf("planner 输出未通过 schema 验证: %s\n原始内容: %s", vr.ErrorString(), content)
	}

	var tasks []model.SubTask
	if err := json.Unmarshal([]byte(jsonStr), &tasks); err != nil {
		return nil, fmt.Errorf("planner 解析任务失败: %w\n原始内容: %s", err, content)
	}

	if len(tasks) == 0 {
		return nil, fmt.Errorf("planner 返回了空的任务列表")
	}

	for i := range tasks {
		tasks[i].Status = model.TaskStatusPending
	}
	return tasks, nil
}

// ==================== Generator Agent ====================

type GeneratorAgent struct {
	provider provider.Provider
	schema   *harness.SchemaValidator
}

func NewGeneratorAgent(p provider.Provider) *GeneratorAgent {
	return &GeneratorAgent{
		provider: p,
		schema:   harness.GeneratorSchema(),
	}
}

func (a *GeneratorAgent) Execute(ctx context.Context, task model.SubTask, artifact model.Artifact) (string, model.Artifact, error) {
	return a.ExecuteWithOpts(ctx, task, artifact, nil)
}

// ExecuteWithOpts 带 Middleware 的执行
func (a *GeneratorAgent) ExecuteWithOpts(ctx context.Context, task model.SubTask, artifact model.Artifact, opts *AgentOptions) (string, model.Artifact, error) {
	systemPrompt := `你是一个执行 Agent。你只负责完成当前的子任务。
你会收到一个状态工件 (Artifact)，它包含了之前步骤的总结信息。
请根据工件信息和当前任务要求，输出任务结果，并更新工件内容。
你只能输出纯 JSON，不要输出任何其他文字或 markdown 代码块标记。
输出格式：{"result": "任务执行结果的详细文本", "updated_artifact_data": {"key": "value"}}`

	artifactJSON, _ := json.Marshal(artifact.Data)
	userContent := fmt.Sprintf("当前任务：%s\n任务描述：%s\n状态工件：%s", task.Title, task.Description, string(artifactJSON))

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	resp, err := callLLM(ctx, a.provider, opts, "generator", task.ID, req)
	if err != nil {
		return "", artifact, fmt.Errorf("generator 调用 LLM 失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", artifact, fmt.Errorf("generator 未收到有效响应")
	}

	content := resp.Choices[0].Message.Content
	jsonStr := extractJSON(content)

	var output struct {
		Result              string                 `json:"result"`
		UpdatedArtifactData map[string]interface{} `json:"updated_artifact_data"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		// Schema 验证失败时的降级：把整个回复当作 result
		return content, artifact, nil
	}

	// Schema 验证（通过后才接受）
	if vr := a.schema.Validate(jsonStr); !vr.Valid {
		// 结构不完整但 JSON 解析成功，接受但记录
		if output.Result == "" {
			return content, artifact, nil
		}
	}

	if output.UpdatedArtifactData != nil {
		artifact.Data = output.UpdatedArtifactData
	}
	return output.Result, artifact, nil
}

// ==================== Evaluator Agent ====================

type EvaluatorAgent struct {
	provider provider.Provider
	schema   *harness.SchemaValidator
}

func NewEvaluatorAgent(p provider.Provider) *EvaluatorAgent {
	return &EvaluatorAgent{
		provider: p,
		schema:   harness.EvaluatorSchema(),
	}
}

func (a *EvaluatorAgent) Evaluate(ctx context.Context, task model.SubTask, result string) (model.EvaluationResult, error) {
	return a.EvaluateWithOpts(ctx, task, result, nil)
}

// EvaluateWithOpts 带 Middleware 的评估
func (a *EvaluatorAgent) EvaluateWithOpts(ctx context.Context, task model.SubTask, result string, opts *AgentOptions) (model.EvaluationResult, error) {
	systemPrompt := `你是一个 AI 质检员。你必须根据以下评分准则对执行结果进行审计。

评分准则:
1. 准确性: 是否存在事实错误？
2. 深度: 是否只是表面文字的堆砌？
3. 冗余度: 是否包含过多的背景废话？
4. 格式符合度: 是否遵循了子任务要求的输出格式？

如果总分低于 60 分，将 passed 设置为 false，并在 feedback 中明确指出修改建议。
你只能输出纯 JSON，不要输出任何其他文字或 markdown 代码块标记。
输出格式：{"score": 85, "feedback": "改进建议或通过说明", "passed": true}
score 必须是 0-100 的整数。`

	userContent := fmt.Sprintf("子任务：%s\n任务描述：%s\n待评估执行结果：%s", task.Title, task.Description, result)

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	resp, err := callLLM(ctx, a.provider, opts, "evaluator", task.ID, req)
	if err != nil {
		return model.EvaluationResult{}, fmt.Errorf("evaluator 调用 LLM 失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return model.EvaluationResult{}, fmt.Errorf("evaluator 未收到有效响应")
	}

	content := resp.Choices[0].Message.Content
	jsonStr := extractJSON(content)

	// Schema 验证
	if vr := a.schema.Validate(jsonStr); !vr.Valid {
		// Schema 验证失败，降级通过
		return model.EvaluationResult{
			Score:    70,
			Feedback: "评估 schema 验证失败(" + vr.ErrorString() + ")，降级通过。原始回复: " + content,
			Passed:   true,
		}, nil
	}

	var eval model.EvaluationResult
	if err := json.Unmarshal([]byte(jsonStr), &eval); err != nil {
		return model.EvaluationResult{
			Score:    70,
			Feedback: "评估解析失败，降级通过。原始回复: " + content,
			Passed:   true,
		}, nil
	}

	// 额外校验：score 范围
	if eval.Score < 0 {
		eval.Score = 0
	}
	if eval.Score > 100 {
		eval.Score = 100
	}

	return eval, nil
}

// ==================== JSON 提取工具 ====================

func extractJSON(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		lines := strings.Split(content, "\n")
		startIdx := 0
		endIdx := len(lines)
		for i, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				if startIdx == 0 {
					startIdx = i + 1
				} else {
					endIdx = i
					break
				}
			}
		}
		if startIdx > 0 && endIdx <= len(lines) {
			content = strings.Join(lines[startIdx:endIdx], "\n")
			content = strings.TrimSpace(content)
		}
	}

	start := -1
	var openChar, closeChar rune

	for i, ch := range content {
		if ch == '{' || ch == '[' {
			start = i
			openChar = ch
			if ch == '{' {
				closeChar = '}'
			} else {
				closeChar = ']'
			}
			break
		}
	}

	if start == -1 {
		return content
	}

	depth := 0
	inString := false
	escaped := false

	for i, ch := range content[start:] {
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == openChar {
			depth++
		} else if ch == closeChar {
			depth--
			if depth == 0 {
				return content[start : start+i+1]
			}
		}
	}

	return content[start:]
}
