package harness

import (
	"fmt"
	"time"
)

// ==================== Session-Level Grading 层 (Phase 5) ====================

// ==================== Session Grade ====================

// SessionGrade 一次 session 的聚合质量评估结果
type SessionGrade struct {
	SessionID       string          `json:"session_id"`
	GradedAt        time.Time       `json:"graded_at"`
	OverallScore    float64         `json:"overall_score"`
	Verdict         GradeVerdict    `json:"verdict"`
	Dimensions      GradeDimensions `json:"dimensions"`
	Violations      []string        `json:"violations"`
	Recommendations []string        `json:"recommendations"`
}

// GradeVerdict 评级结论
type GradeVerdict string

const (
	VerdictPass GradeVerdict = "pass"
	VerdictWarn GradeVerdict = "warn"
	VerdictFail GradeVerdict = "fail"
)

// GradeDimensions 多维度评估
type GradeDimensions struct {
	TaskCompletion float64 `json:"task_completion"`
	EvalQuality    float64 `json:"eval_quality"`
	Efficiency     float64 `json:"efficiency"`
	Reliability    float64 `json:"reliability"`
	PlanAdherence  float64 `json:"plan_adherence"`
}

// ==================== Quality Gate ====================

// QualityGate 质量门禁配置
type QualityGate struct {
	MinOverallScore   float64
	MinTaskCompletion float64
	MaxRePlanRatio    float64
	MaxErrorRate      float64
	MinAvgEvalScore   float64
	MaxTokenWaste     float64
}

func DefaultQualityGate() QualityGate {
	return QualityGate{
		MinOverallScore:   60,
		MinTaskCompletion: 50,
		MaxRePlanRatio:    0.5,
		MaxErrorRate:      0.3,
		MinAvgEvalScore:   50,
		MaxTokenWaste:     0.7,
	}
}

// ==================== Session Grader ====================

// SessionGradeInput 计算评分所需的输入数据
type SessionGradeInput struct {
	SessionID      string
	TotalTasks     int
	CompletedTasks int
	FailedTasks    int
	TotalAttempts  int
	RePlanCount    int
	MaxRePlans     int
	EvalScores     []int
	TotalTokens    int
	TokenBudget    int
	TotalDuration  time.Duration
	TimeBudget     time.Duration
	ErrorCount     int
	CircuitTripped bool
}

// SessionGrader 计算 session 级评分
type SessionGrader struct {
	gate QualityGate
}

func NewSessionGrader(gate QualityGate) *SessionGrader {
	return &SessionGrader{gate: gate}
}

func NewDefaultSessionGrader() *SessionGrader {
	return &SessionGrader{gate: DefaultQualityGate()}
}

// Grade 计算 session 的综合评分
func (g *SessionGrader) Grade(input SessionGradeInput) SessionGrade {
	dims := g.computeDimensions(input)
	overall := g.computeOverall(dims)
	violations := g.checkViolations(input, dims, overall)
	recommendations := g.generateRecommendations(input, dims, violations)
	verdict := g.computeVerdict(overall, violations)

	return SessionGrade{
		SessionID:       input.SessionID,
		GradedAt:        time.Now(),
		OverallScore:    overall,
		Verdict:         verdict,
		Dimensions:      dims,
		Violations:      violations,
		Recommendations: recommendations,
	}
}

func (g *SessionGrader) computeDimensions(input SessionGradeInput) GradeDimensions {
	var dims GradeDimensions

	// 1. 任务完成率
	if input.TotalTasks > 0 {
		dims.TaskCompletion = float64(input.CompletedTasks) / float64(input.TotalTasks) * 100
	}

	// 2. 平均评估分
	if len(input.EvalScores) > 0 {
		total := 0
		for _, s := range input.EvalScores {
			total += s
		}
		dims.EvalQuality = float64(total) / float64(len(input.EvalScores))
	}

	// 3. 效率分
	tokenEfficiency := 100.0
	if input.TokenBudget > 0 && input.TotalTasks > 0 {
		if input.CompletedTasks > 0 {
			avgTokenPerTask := float64(input.TotalTokens) / float64(input.CompletedTasks)
			idealTokenPerTask := float64(input.TokenBudget) / float64(input.TotalTasks)
			if idealTokenPerTask > 0 {
				ratio := avgTokenPerTask / idealTokenPerTask
				if ratio > 1 {
					tokenEfficiency = maxFloat64(0, 100-((ratio-1)*50))
				}
			}
		} else {
			tokenEfficiency = 0
		}
	}

	timeEfficiency := 100.0
	if input.TimeBudget > 0 {
		timeRatio := float64(input.TotalDuration) / float64(input.TimeBudget)
		if timeRatio > 1 {
			timeEfficiency = maxFloat64(0, 100-((timeRatio-1)*50))
		}
	}
	dims.Efficiency = (tokenEfficiency + timeEfficiency) / 2

	// 4. 可靠性分
	dims.Reliability = 100.0
	if input.TotalTasks > 0 {
		errorRate := float64(input.FailedTasks) / float64(input.TotalTasks)
		dims.Reliability -= errorRate * 40

		if input.TotalAttempts > input.TotalTasks {
			retryRate := float64(input.TotalAttempts-input.TotalTasks) / float64(input.TotalTasks)
			dims.Reliability -= retryRate * 15
		}

		if input.CircuitTripped {
			dims.Reliability -= 20
		}
	}
	dims.Reliability = maxFloat64(0, dims.Reliability)

	// 5. 计划执行率
	dims.PlanAdherence = 100.0
	if input.MaxRePlans > 0 {
		rePlanPenalty := float64(input.RePlanCount) / float64(input.MaxRePlans) * 40
		dims.PlanAdherence -= rePlanPenalty
	} else if input.RePlanCount > 0 {
		dims.PlanAdherence -= float64(input.RePlanCount) * 20
	}
	dims.PlanAdherence = maxFloat64(0, dims.PlanAdherence)

	return dims
}

func (g *SessionGrader) computeOverall(dims GradeDimensions) float64 {
	overall := dims.TaskCompletion*0.35 +
		dims.EvalQuality*0.25 +
		dims.Efficiency*0.15 +
		dims.Reliability*0.15 +
		dims.PlanAdherence*0.10

	return clamp(overall, 0, 100)
}

func (g *SessionGrader) computeVerdict(overall float64, violations []string) GradeVerdict {
	if overall < g.gate.MinOverallScore || len(violations) >= 3 {
		return VerdictFail
	}
	if len(violations) > 0 {
		return VerdictWarn
	}
	return VerdictPass
}

func (g *SessionGrader) checkViolations(input SessionGradeInput, dims GradeDimensions, overall float64) []string {
	var violations []string

	if overall < g.gate.MinOverallScore {
		violations = append(violations,
			fmt.Sprintf("overall score %.1f < minimum %.1f", overall, g.gate.MinOverallScore))
	}

	if dims.TaskCompletion < g.gate.MinTaskCompletion {
		violations = append(violations,
			fmt.Sprintf("task completion %.1f%% < minimum %.1f%%", dims.TaskCompletion, g.gate.MinTaskCompletion))
	}

	if input.MaxRePlans > 0 {
		rePlanRatio := float64(input.RePlanCount) / float64(input.MaxRePlans)
		if rePlanRatio > g.gate.MaxRePlanRatio {
			violations = append(violations,
				fmt.Sprintf("re-plan ratio %.2f > maximum %.2f", rePlanRatio, g.gate.MaxRePlanRatio))
		}
	}

	if input.TotalTasks > 0 {
		errorRate := float64(input.FailedTasks) / float64(input.TotalTasks)
		if errorRate > g.gate.MaxErrorRate {
			violations = append(violations,
				fmt.Sprintf("error rate %.2f > maximum %.2f", errorRate, g.gate.MaxErrorRate))
		}
	}

	if dims.EvalQuality > 0 && dims.EvalQuality < g.gate.MinAvgEvalScore {
		violations = append(violations,
			fmt.Sprintf("avg eval score %.1f < minimum %.1f", dims.EvalQuality, g.gate.MinAvgEvalScore))
	}

	// Token 浪费检测
	if input.TokenBudget > 0 && input.TotalTasks > 0 {
		tokenUtilization := float64(input.TotalTokens) / float64(input.TokenBudget)
		completionRate := float64(input.CompletedTasks) / float64(input.TotalTasks)
		if tokenUtilization > g.gate.MaxTokenWaste && completionRate < 0.5 {
			violations = append(violations,
				fmt.Sprintf("token waste: used %.0f%% budget but only completed %.0f%% tasks",
					tokenUtilization*100, completionRate*100))
		}
	}

	return violations
}

func (g *SessionGrader) generateRecommendations(input SessionGradeInput, dims GradeDimensions, violations []string) []string {
	var recs []string

	if dims.TaskCompletion < 50 {
		recs = append(recs, "Consider breaking down complex tasks into smaller subtasks")
	}

	if dims.Efficiency < 60 && input.TotalTokens > 0 {
		recs = append(recs, "Token usage is high relative to output — review prompt engineering for conciseness")
	}

	if dims.Reliability < 70 {
		if input.CircuitTripped {
			recs = append(recs, "Circuit breaker tripped — check provider stability or increase threshold")
		}
		if input.FailedTasks > 0 {
			recs = append(recs,
				fmt.Sprintf("%d tasks failed — review error patterns in handoff artifact", input.FailedTasks))
		}
	}

	if dims.PlanAdherence < 70 && input.RePlanCount > 0 {
		recs = append(recs,
			fmt.Sprintf("Plan was revised %d times — consider improving goal decomposition", input.RePlanCount))
	}

	if dims.EvalQuality > 0 && dims.EvalQuality < 70 {
		recs = append(recs, "Average eval scores are low — review evaluator criteria or task descriptions")
	}

	return recs
}

// ==================== 辅助函数 ====================

func clamp(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

// maxFloat64 返回两个 float64 中的较大值
// 注意：不使用 "max" 作为函数名，避免遮蔽 Go 1.21+ 内置的 max 函数
func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
