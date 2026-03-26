package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentic-learning/gateway/internal/model"
	"github.com/agentic-learning/gateway/internal/provider"
)

// PlannerAgent 规划者 Agent
type PlannerAgent struct {
	provider provider.Provider
}

func NewPlannerAgent(p provider.Provider) *PlannerAgent {
	return &PlannerAgent{provider: p}
}

// Plan 将大目标拆解为子任务
func (a *PlannerAgent) Plan(ctx context.Context, goal string) ([]model.SubTask, error) {
	systemPrompt := `你是一个 AI 学习系统的战略规划者。
你的任务是将用户的大目标拆解为一系列原子级的、可执行的子任务。
输出格式必须是 JSON 数组，每个对象包含 id, title, description。
例如：[{"id": "task1", "title": "任务标题", "description": "详细描述"}]`

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: fmt.Sprintf("目标：%s", goal)},
		},
	}

	resp, err := a.provider.ChatCompletion(ctx, req)
	if err != nil {
		return nil, err
	}

	var tasks []model.SubTask
	content := resp.Choices[0].Message.Content
	// 这里简单处理，实际生产中需要更健壮的 JSON 提取逻辑
	if err := json.Unmarshal([]byte(extractJSON(content)), &tasks); err != nil {
		return nil, fmt.Errorf("解析任务失败: %w, 内容: %s", err, content)
	}

	for i := range tasks {
		tasks[i].Status = model.TaskStatusPending
	}
	return tasks, nil
}

// GeneratorAgent 执行者 Agent
type GeneratorAgent struct {
	provider provider.Provider
}

func NewGeneratorAgent(p provider.Provider) *GeneratorAgent {
	return &GeneratorAgent{provider: p}
}

// Execute 执行单个子任务，并利用 Artifact 实现上下文重置
func (a *GeneratorAgent) Execute(ctx context.Context, task model.SubTask, artifact model.Artifact) (string, model.Artifact, error) {
	systemPrompt := `你是一个执行 Agent。你只负责完成当前的子任务。
你会收到一个状态工件 (Artifact)，它包含了之前步骤的总结信息。
请根据工件信息和当前任务要求，输出任务结果，并更新工件内容。
输出格式：{"result": "任务执行结果", "updated_artifact_data": {"key": "value"}}`

	artifactJSON, _ := json.Marshal(artifact.Data)
	userContent := fmt.Sprintf("当前任务：%s\n任务描述：%s\n状态工件：%s", task.Title, task.Description, string(artifactJSON))

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	resp, err := a.provider.ChatCompletion(ctx, req)
	if err != nil {
		return "", artifact, err
	}

	var output struct {
		Result              string                 `json:"result"`
		UpdatedArtifactData map[string]interface{} `json:"updated_artifact_data"`
	}

	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(extractJSON(content)), &output); err != nil {
		return "", artifact, fmt.Errorf("解析执行结果失败: %w", err)
	}

	artifact.Data = output.UpdatedArtifactData
	return output.Result, artifact, nil
}

// EvaluatorAgent 评估者 Agent
type EvaluatorAgent struct {
	provider provider.Provider
}

func NewEvaluatorAgent(p provider.Provider) *EvaluatorAgent {
	return &EvaluatorAgent{provider: p}
}

// Evaluate 评估执行结果
func (a *EvaluatorAgent) Evaluate(ctx context.Context, task model.SubTask, result string) (model.EvaluationResult, error) {
	systemPrompt := `你是一个严苛的 AI 质检员。你必须根据以下【评分准则 (Rubrics)】对执行结果进行审计。

【评分准则】:
1. 准确性 (Accuracy): 是否存在事实错误？
2. 深度 (Depth): 是否只是表面文字的堆砌？是否提取了非显性的洞察？
3. 冗余度 (Redundancy): 是否包含超过 20% 的背景废话？
4. 格式符合度 (Formatting): 是否严格遵循了子任务要求的输出格式？

如果总分低于 80 分，你必须将 Passed 设置为 false，并在 Feedback 中明确指出违反了哪条准则以及具体的修改建议。
输出格式：{"score": 85, "feedback": "改进建议", "passed": true}`

	userContent := fmt.Sprintf("子任务：%s\n任务描述：%s\n待评估执行结果：%s", task.Title, task.Description, result)

	req := &model.ChatCompletionRequest{
		Messages: []model.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
	}

	resp, err := a.provider.ChatCompletion(ctx, req)
	if err != nil {
		return model.EvaluationResult{}, err
	}

	var eval model.EvaluationResult
	content := resp.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(extractJSON(content)), &eval); err != nil {
		return model.EvaluationResult{}, fmt.Errorf("解析评估结果失败: %w", err)
	}

	return eval, nil
}

// extractJSON 辅助函数，从 AI 回复中提取 JSON 部分
func extractJSON(content string) string {
	start := -1
	end := -1
	for i, char := range content {
		if char == '{' || char == '[' {
			if start == -1 {
				start = i
			}
		}
		if char == '}' || char == ']' {
			end = i
		}
	}
	if start != -1 && end != -1 && end >= start {
		return content[start : end+1]
	}
	return content
}
