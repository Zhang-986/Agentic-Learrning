package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/agentic-learning/gateway/internal/agent"
	"github.com/agentic-learning/gateway/internal/harness"
	"github.com/agentic-learning/gateway/internal/model"
)

// ==================== 配置 ====================

type HarnessConfig struct {
	MaxRetriesPerTask int
	MaxRePlans        int
	TaskTimeout       time.Duration
	EvalPassScore     int
	LogDir            string // 日志持久化目录（空 = 不持久化）
	HandoffDir        string // Handoff Artifact 存储目录（空 = 不持久化）
}

func DefaultConfig() HarnessConfig {
	return HarnessConfig{
		MaxRetriesPerTask: 2,
		MaxRePlans:        2,
		TaskTimeout:       120 * time.Second,
		EvalPassScore:     60,
		LogDir:            "",
		HandoffDir:        "",
	}
}

type EventCallback func(event model.HarnessEvent)

// ==================== 编排器 ====================

type HarnessOrchestrator struct {
	planner   *agent.PlannerAgent
	generator *agent.GeneratorAgent
	evaluator *agent.EvaluatorAgent
	store     SessionStore
	config    HarnessConfig

	// Phase 1: Middleware + Budget
	middleware *harness.MiddlewareChain
	budget     *harness.BudgetTracker

	// Phase 2: Context Engineering
	progress *harness.ProgressTracker
	features *harness.FeatureTracker

	// Phase 3: Session Logger
	logger *harness.SessionLogger

	// Phase 4: Handoff + Priming
	handoffBuilder *harness.HandoffBuilder
	handoffStore   *harness.HandoffStore
	priming        *harness.PrimingProtocol

	// Phase 5: Structured Tracing + Session Grading
	trace  *harness.TraceCollector
	grader *harness.SessionGrader
}

func NewHarnessOrchestrator(
	planner *agent.PlannerAgent,
	generator *agent.GeneratorAgent,
	evaluator *agent.EvaluatorAgent,
	store SessionStore,
) *HarnessOrchestrator {
	return NewHarnessOrchestratorWithConfig(planner, generator, evaluator, store, DefaultConfig())
}

func NewHarnessOrchestratorWithConfig(
	planner *agent.PlannerAgent,
	generator *agent.GeneratorAgent,
	evaluator *agent.EvaluatorAgent,
	store SessionStore,
	config HarnessConfig,
) *HarnessOrchestrator {
	mw := harness.NewMiddlewareChain()
	mw.AddAfterHook(harness.TokenTrackingHook())

	cb := harness.NewCircuitBreaker(5, 30*time.Second)
	mw.AddBeforeHook(cb.BeforeHook())
	mw.AddAfterHook(cb.AfterHook())

	bt := harness.NewBudgetTracker(harness.DefaultBudgetConfig())
	mw.AddBeforeHook(harness.BudgetCheckBeforeHook(bt))
	mw.AddAfterHook(harness.BudgetRecordAfterHook(bt))

	return &HarnessOrchestrator{
		planner:      planner,
		generator:    generator,
		evaluator:    evaluator,
		store:        store,
		config:       config,
		middleware:   mw,
		budget:       bt,
		progress:     harness.NewProgressTracker(),
		features:     harness.NewFeatureTracker(),
		handoffStore: harness.NewHandoffStore(config.HandoffDir),
		priming:      harness.NewPrimingProtocol(),
		grader:       harness.NewDefaultSessionGrader(),
	}
}

func (o *HarnessOrchestrator) agentOpts(sessionID string) *agent.AgentOptions {
	return &agent.AgentOptions{
		Middleware: o.middleware,
		SessionID: sessionID,
	}
}

// ==================== 会话生命周期 ====================

func (o *HarnessOrchestrator) ExecuteSession(ctx context.Context, goal string, onEvent EventCallback) (*model.HarnessSession, error) {
	if onEvent == nil {
		onEvent = func(event model.HarnessEvent) {}
	}

	// 初始化
	session := model.NewSession(goal)
	session.MaxRePlans = o.config.MaxRePlans

	// 初始化 Session Logger
	o.logger = harness.NewSessionLogger(session.ID, goal, o.config.LogDir)

	// 初始化 Handoff Builder
	o.handoffBuilder = harness.NewHandoffBuilder(session.ID, goal)

	// Phase 5: 初始化 Trace Collector
	o.trace = harness.NewTraceCollector(session.ID, goal)

	o.emit(onEvent, model.EventInfo, "会话初始化完成", nil)
	o.logEvent("init", "会话初始化完成", nil)

	// ====== Priming Protocol ======
	// 不再做上下文压缩，而是从上一个 session 的 Handoff Artifact 中加载交接信息
	lastHandoff, err := o.handoffStore.LoadLatest()
	if err != nil {
		o.emit(onEvent, model.EventInfo,
			fmt.Sprintf("读取上次交接信息时出错 (非致命): %v", err), nil)
	}
	primingContext := o.priming.BuildPrimingContext(o.features, lastHandoff, o.progress)

	if lastHandoff != nil {
		o.emit(onEvent, model.EventInfo,
			fmt.Sprintf("Priming: 已读取上次会话 [%s] 的交接信息 (状态: %s)",
				lastHandoff.SessionID, lastHandoff.FinalStatus), nil)
		o.logEvent("priming", "从 Handoff Artifact 加载交接上下文", lastHandoff.SessionID)

		// 如果上一个 session 环境不健康，记录警告
		if !lastHandoff.Environment.Healthy {
			o.emit(onEvent, model.EventInfo,
				fmt.Sprintf("⚠ 上次环境警告: %s", lastHandoff.Environment.LastError), nil)
			o.handoffBuilder.RecordDecision(
				"acknowledged environment warning from previous session",
				lastHandoff.Environment.LastError,
				"",
			)
		}
	} else {
		o.emit(onEvent, model.EventInfo, "首次运行，无历史交接信息", nil)
	}

	o.persist(session)

	// 阶段 1: 规划（注入 Priming Context）
	session.Status = model.SessionPlanning
	o.persist(session)
	if err := o.plan(ctx, session, onEvent, primingContext); err != nil {
		session.Status = model.SessionFailed
		o.persist(session)
		o.buildAndSaveHandoff(session)
		o.gradeAndEmit(session, onEvent)
		o.finalizeLog(session)
		return session, err
	}

	// 阶段 2: 执行循环
	result, execErr := o.runLoop(ctx, session, onEvent)

	// 阶段 3: 推送最终统计
	budgetStatus := o.budget.Status()
	callStats := o.middleware.GetCallStats()

	o.emit(onEvent, model.EventMetrics, "Session 预算消耗", budgetStatus)
	o.emit(onEvent, model.EventMetrics, "LLM 调用统计", callStats)

	// 阶段 4: Feature Tracker 进度
	if fp := o.features.GetProgress(); fp["total"].(int) > 0 {
		o.emit(onEvent, model.EventMetrics, "功能完成度", fp)
	}

	// ====== Phase 5: Finalize Trace ======
	traceData := o.trace.Finalize(string(result.Status))
	traceStats := harness.ComputeTraceStats(traceData)
	o.emit(onEvent, model.EventMetrics, "Structured Trace", map[string]interface{}{
		"trace_id":       traceData.TraceID,
		"total_spans":    traceStats.TotalSpans,
		"iterations":     traceStats.Iterations,
		"total_duration": traceStats.TotalDuration.String(),
		"total_input":    traceStats.TotalInput,
		"total_output":   traceStats.TotalOutput,
		"error_spans":    traceStats.ErrorSpans,
		"bottleneck":     traceStats.Bottleneck,
		"by_agent":       traceStats.ByAgent,
	})

	// ====== 生成 Handoff Artifact ======
	o.buildAndSaveHandoff(result)

	// ====== Phase 5: Session Grading ======
	o.gradeAndEmit(result, onEvent)

	// 阶段 5: 持久化日志
	o.finalizeLog(result)

	return result, execErr
}

func (o *HarnessOrchestrator) ResumeSession(ctx context.Context, sessionID string, onEvent EventCallback) (*model.HarnessSession, error) {
	if onEvent == nil {
		onEvent = func(event model.HarnessEvent) {}
	}

	session, ok := o.store.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("会话不存在: %s", sessionID)
	}

	if session.Status == model.SessionCompleted || session.Status == model.SessionFailed {
		o.emit(onEvent, model.EventInfo,
			fmt.Sprintf("会话已结束 (状态: %s)，无需恢复", session.Status), nil)
		return session, nil
	}

	// Resume 时也初始化 handoff builder 和 trace
	if o.handoffBuilder == nil {
		o.handoffBuilder = harness.NewHandoffBuilder(session.ID, session.Goal)
	}
	if o.trace == nil {
		o.trace = harness.NewTraceCollector(session.ID, session.Goal)
	}

	o.emit(onEvent, model.EventInfo,
		fmt.Sprintf("正在恢复会话 [%s]，从上次中断处继续...", sessionID), nil)
	return o.runLoop(ctx, session, onEvent)
}

// ==================== 核心编排循环 ====================

func (o *HarnessOrchestrator) plan(ctx context.Context, session *model.HarnessSession, onEvent EventCallback, primingContext string) error {
	o.emit(onEvent, model.EventInfo, "Planner 正在拆解任务...", nil)

	// Phase 5: Start plan span
	var planSpan *harness.Span
	if o.trace != nil {
		planSpan = o.trace.StartSpan("planner.plan", "planner", "")
	}

	planGoal := session.Goal
	if session.RePlanCount > 0 {
		planGoal = o.buildRePlanContext(session)
	}

	// 将 Priming Context 注入规划上下文（取代旧的 progressSummary 注入）
	if session.RePlanCount == 0 && primingContext != "" &&
		primingContext != "No prior session context. This is the first session." {
		planGoal = planGoal + "\n\n" + primingContext
	}

	tasks, err := o.planner.PlanWithOpts(ctx, planGoal, o.agentOpts(session.ID))

	// Phase 5: Finish plan span
	if planSpan != nil {
		if err != nil {
			planSpan.End(harness.SpanStatusError, err.Error())
		} else {
			planSpan.SetAttribute("tasks_count", fmt.Sprintf("%d", len(tasks)))
			planSpan.End(harness.SpanStatusOK, "")
		}
		o.trace.FinishSpan(planSpan)
	}

	if err != nil {
		o.emit(onEvent, model.EventError, fmt.Sprintf("任务规划失败: %v", err), nil)
		o.logEvent("plan_failed", err.Error(), nil)
		return fmt.Errorf("任务规划失败: %w", err)
	}

	session.Tasks = tasks
	session.Status = model.SessionRunning
	session.UpdatedAt = time.Now()
	o.persist(session)

	// 记录规划进度
	o.progress.Record(harness.ProgressEntry{
		SessionID: session.ID,
		Action:    "planned",
		Summary:   fmt.Sprintf("规划了 %d 个子任务", len(tasks)),
	})
	o.logEvent("planned", fmt.Sprintf("生成 %d 个子任务", len(tasks)), tasks)

	// 记录决策
	if o.handoffBuilder != nil {
		o.handoffBuilder.RecordDecision(
			fmt.Sprintf("planned %d subtasks", len(tasks)),
			fmt.Sprintf("goal: %s", session.Goal),
			"",
		)
	}

	// 注册到 Feature Tracker
	features := make([]harness.FeatureStatus, len(tasks))
	for i, t := range tasks {
		features[i] = harness.FeatureStatus{
			ID:          t.ID,
			Description: t.Title + ": " + t.Description,
			Category:    "functional",
			Passes:      false,
		}
	}
	o.features.AddFeatures(features)

	o.emit(onEvent, model.EventInfo,
		fmt.Sprintf("Planner 拆解完成，共生成 %d 个子任务", len(tasks)), tasks)
	return nil
}

func (o *HarnessOrchestrator) runLoop(ctx context.Context, session *model.HarnessSession, onEvent EventCallback) (*model.HarnessSession, error) {
	for i := range session.Tasks {
		task := &session.Tasks[i]

		if task.Status == model.TaskStatusCompleted || task.Status == model.TaskStatusSkipped {
			continue
		}

		if o.budget.IsExhausted() {
			o.emit(onEvent, model.EventError,
				fmt.Sprintf("预算耗尽，中止执行。已完成 %d 个任务。", o.countCompleted(session)),
				o.budget.Status())
			o.logEvent("budget_exhausted", "预算耗尽", o.budget.Status())

			// Handoff: 记录预算耗尽为环境问题
			if o.handoffBuilder != nil {
				o.handoffBuilder.SetEnvironment(harness.EnvironmentState{
					Healthy:    false,
					LastError:  "budget exhausted before completing all tasks",
					OpenIssues: len(session.Tasks) - o.countCompleted(session),
					BudgetLeft: "0",
				})
				o.handoffBuilder.AddDoNotRepeat("Budget exhausted — consider increasing budget or reducing task granularity")
			}

			session.Status = model.SessionFailed
			o.persist(session)
			return session, fmt.Errorf("budget exhausted")
		}

		taskResult := o.executeTaskWithRetry(ctx, session, task, onEvent)

		session.UpdateMetrics()
		o.persist(session)
		o.emit(onEvent, model.EventTaskComplete,
			fmt.Sprintf("子任务 [%s] 结束，状态: %s (耗时 %dms, 尝试 %d 次)",
				task.ID, task.Status, task.LatencyMs, task.Attempts),
			task,
		)
		o.emit(onEvent, model.EventMetrics, "当前会话度量", session.Metrics)

		if taskResult == taskResultFailed {
			shouldRePlan := o.shouldRePlan(session)

			if shouldRePlan {
				o.emit(onEvent, model.EventRePlan,
					fmt.Sprintf("子任务 [%s] 失败，触发 Re-plan (第 %d/%d 次)",
						task.ID, session.RePlanCount+1, o.config.MaxRePlans),
					nil,
				)
				o.logEvent("re_plan",
					fmt.Sprintf("Re-plan #%d triggered by task %s", session.RePlanCount+1, task.ID), nil)

				// Handoff: 记录 re-plan 决策
				if o.handoffBuilder != nil {
					o.handoffBuilder.RecordDecision(
						fmt.Sprintf("re-plan triggered (attempt %d/%d)", session.RePlanCount+1, o.config.MaxRePlans),
						fmt.Sprintf("task %s failed", task.ID),
						task.ID,
					)
				}

				// Phase 5: 新 iteration
				if o.trace != nil {
					o.trace.NextIteration()
				}

				session.RePlanCount++
				session.Status = model.SessionRePlanning
				o.persist(session)

				o.progress.Record(harness.ProgressEntry{
					SessionID: session.ID,
					TaskID:    task.ID,
					Action:    "re_planned",
					Summary:   fmt.Sprintf("任务 %s 失败，触发第 %d 次重新规划", task.ID, session.RePlanCount),
				})

				// Re-plan 时不再注入 priming（已在 session 内）
				if err := o.plan(ctx, session, onEvent, ""); err != nil {
					session.Status = model.SessionFailed
					o.persist(session)
					return session, err
				}

				return o.runLoop(ctx, session, onEvent)
			}

			o.emit(onEvent, model.EventInfo,
				fmt.Sprintf("子任务 [%s] 失败，已达 Re-plan 上限，继续执行剩余任务", task.ID),
				nil,
			)
		}
	}

	// 最终状态
	session.UpdateMetrics()
	if session.Metrics.FailedTasks > 0 && session.Metrics.CompletedTasks == 0 {
		session.Status = model.SessionFailed
	} else {
		session.Status = model.SessionCompleted
	}
	session.UpdatedAt = time.Now()
	o.persist(session)

	// 记录完成进度
	o.progress.Record(harness.ProgressEntry{
		SessionID: session.ID,
		Action:    string(session.Status),
		Summary: fmt.Sprintf("完成 %d/%d 个任务",
			session.Metrics.CompletedTasks, session.Metrics.TotalTasks),
	})

	o.emit(onEvent, model.EventSessionComplete,
		fmt.Sprintf("会话执行完毕！完成 %d/%d 个任务",
			session.Metrics.CompletedTasks, session.Metrics.TotalTasks),
		session,
	)

	return session, nil
}

// ==================== 单任务执行 ====================

type taskExecResult int

const (
	taskResultSuccess taskExecResult = iota
	taskResultFailed
)

func (o *HarnessOrchestrator) executeTaskWithRetry(
	ctx context.Context,
	session *model.HarnessSession,
	task *model.SubTask,
	onEvent EventCallback,
) taskExecResult {

	now := time.Now()
	task.Status = model.TaskStatusRunning
	task.StartedAt = &now
	o.persist(session)

	o.emit(onEvent, model.EventTaskStart,
		fmt.Sprintf("开始执行子任务 [%s]: %s", task.ID, task.Title),
		task,
	)
	o.logEvent("task_start", fmt.Sprintf("开始 %s", task.Title), task.ID)

	// 记录进度
	o.progress.Record(harness.ProgressEntry{
		SessionID: session.ID,
		TaskID:    task.ID,
		Action:    "started",
		Summary:   task.Title,
	})

	originalDesc := task.Description
	opts := o.agentOpts(session.ID)

	for attempt := 0; attempt <= o.config.MaxRetriesPerTask; attempt++ {
		task.Attempts = attempt + 1

		if attempt > 0 {
			o.emit(onEvent, model.EventInfo,
				fmt.Sprintf("子任务 [%s] 第 %d 次重试...", task.ID, attempt), nil)
		}

		taskCtx, cancel := context.WithTimeout(ctx, o.config.TaskTimeout)

		// Phase 5: Start execute span
		var execSpan *harness.Span
		if o.trace != nil {
			execSpan = o.trace.StartSpan("generator.execute", "generator", task.ID)
			execSpan.SetAttribute("attempt", fmt.Sprintf("%d", attempt+1))
		}

		execStart := time.Now()
		result, updatedArtifact, err := o.generator.ExecuteWithOpts(taskCtx, *task, session.Artifact, opts)
		task.LatencyMs = time.Since(execStart).Milliseconds()
		cancel()

		// Phase 5: Finish execute span
		if execSpan != nil {
			if err != nil {
				execSpan.End(harness.SpanStatusError, err.Error())
			} else {
				execSpan.End(harness.SpanStatusOK, "")
			}
			o.trace.FinishSpan(execSpan)
		}

		if err != nil {
			o.emit(onEvent, model.EventError,
				fmt.Sprintf("子任务 [%s] 执行出错: %v", task.ID, err), nil)

			if attempt >= o.config.MaxRetriesPerTask {
				finishNow := time.Now()
				task.Status = model.TaskStatusFailed
				task.FinishedAt = &finishNow
				task.Result = fmt.Sprintf("执行失败: %v", err)

				o.progress.Record(harness.ProgressEntry{
					SessionID: session.ID,
					TaskID:    task.ID,
					Action:    "failed",
					Summary:   fmt.Sprintf("执行出错: %v", err),
				})
				o.logEvent("task_failed", err.Error(), task.ID)

				// Handoff: 记录失败
				if o.handoffBuilder != nil {
					o.handoffBuilder.RecordFailure(task.ID, err.Error(), "execution error", task.Attempts)
					o.handoffBuilder.AddDoNotRepeat(
						fmt.Sprintf("Task %s failed with: %v — verify prerequisites before retrying", task.ID, err))
				}

				return taskResultFailed
			}
			continue
		}

		// Phase 5: Start evaluate span
		var evalSpan *harness.Span
		if o.trace != nil {
			evalSpan = o.trace.StartSpan("evaluator.evaluate", "evaluator", task.ID)
			evalSpan.SetAttribute("attempt", fmt.Sprintf("%d", attempt+1))
		}

		// Evaluate
		o.emit(onEvent, model.EventInfo,
			fmt.Sprintf("子任务 [%s] 执行完成 (%dms)，开始 Evaluator 评估...",
				task.ID, task.LatencyMs), nil)

		eval, evalErr := o.evaluator.EvaluateWithOpts(ctx, *task, result, opts)

		// Phase 5: Finish evaluate span
		if evalSpan != nil {
			if evalErr != nil {
				evalSpan.End(harness.SpanStatusError, evalErr.Error())
			} else {
				evalSpan.SetAttribute("score", fmt.Sprintf("%d", eval.Score))
				evalSpan.SetAttribute("passed", fmt.Sprintf("%v", eval.Passed))
				evalSpan.End(harness.SpanStatusOK, "")
			}
			o.trace.FinishSpan(evalSpan)
		}

		if evalErr != nil {
			o.emit(onEvent, model.EventInfo,
				fmt.Sprintf("评估调用失败 (%v)，降级接受结果", evalErr), nil)
			eval = model.EvaluationResult{
				Score: 70, Feedback: "评估不可用，降级通过", Passed: true, Attempt: task.Attempts,
			}
		}

		eval.Attempt = task.Attempts
		task.EvalHistory = append(task.EvalHistory, eval)

		o.emit(onEvent, model.EventTaskEval,
			fmt.Sprintf("子任务 [%s] 评估结果: 分数=%d, 通过=%v",
				task.ID, eval.Score, eval.Passed),
			eval,
		)

		if eval.Passed {
			finishNow := time.Now()
			task.Result = result
			task.Status = model.TaskStatusCompleted
			task.FinishedAt = &finishNow
			session.Artifact = updatedArtifact
			session.Artifact.LastUpdated = time.Now()
			session.SnapshotArtifact(task.ID)

			// 更新 Feature Tracker
			o.features.MarkPassed(task.ID, session.ID)

			// 记录完成进度
			o.progress.Record(harness.ProgressEntry{
				SessionID:   session.ID,
				TaskID:      task.ID,
				Action:      "completed",
				Summary:     fmt.Sprintf("评分 %d，通过", eval.Score),
				ArtifactRef: fmt.Sprintf("snapshot_%s", task.ID),
			})
			o.logEvent("task_completed",
				fmt.Sprintf("评分=%d, 尝试=%d", eval.Score, task.Attempts), task.ID)

			// Handoff: 记录变更和验证
			if o.handoffBuilder != nil {
				o.handoffBuilder.RecordChange(task.ID, task.Title,
					fmt.Sprintf("score=%d, attempts=%d", eval.Score, task.Attempts))
				o.handoffBuilder.RecordVerification(task.ID, "evaluator", eval.Score, eval.Feedback)
			}

			return taskResultSuccess
		}

		if attempt < o.config.MaxRetriesPerTask {
			task.Description = originalDesc + fmt.Sprintf("\n\n[上一次质检反馈，请据此改进]: %s", eval.Feedback)
		} else {
			finishNow := time.Now()
			task.Result = result + "\n(未通过质检: " + eval.Feedback + ")"
			task.Status = model.TaskStatusFailed
			task.FinishedAt = &finishNow
			session.Artifact = updatedArtifact
			session.Artifact.LastUpdated = time.Now()
			session.SnapshotArtifact(task.ID)

			o.progress.Record(harness.ProgressEntry{
				SessionID: session.ID,
				TaskID:    task.ID,
				Action:    "failed",
				Summary:   fmt.Sprintf("评分 %d，未通过", eval.Score),
			})
			o.logEvent("task_failed",
				fmt.Sprintf("评分=%d, feedback=%s", eval.Score, eval.Feedback), task.ID)

			// Handoff: 记录失败
			if o.handoffBuilder != nil {
				o.handoffBuilder.RecordFailure(task.ID,
					fmt.Sprintf("eval score %d < threshold", eval.Score),
					eval.Feedback, task.Attempts)
			}

			return taskResultFailed
		}
	}

	return taskResultFailed
}

// ==================== Handoff 生成 ====================

// buildAndSaveHandoff 在 session 结束时构建并持久化 Handoff Artifact
func (o *HarnessOrchestrator) buildAndSaveHandoff(session *model.HarnessSession) {
	if o.handoffBuilder == nil {
		return
	}

	// 设置环境状态
	budgetStatus := o.budget.Status()
	healthy := session.Status == model.SessionCompleted
	env := harness.EnvironmentState{
		Healthy:    healthy,
		OpenIssues: 0,
		BudgetLeft: fmt.Sprintf("tokens: %d/%d, calls: %d",
			budgetStatus.TotalTokensUsed,
			budgetStatus.TotalTokensLimit,
			budgetStatus.LLMCalls),
	}
	if !healthy {
		env.LastError = fmt.Sprintf("session ended with status: %s", session.Status)
		env.OpenIssues = session.Metrics.FailedTasks
	}
	o.handoffBuilder.SetEnvironment(env)

	// 生成未完成任务的 next actions
	for _, task := range session.Tasks {
		if task.Status != model.TaskStatusCompleted && task.Status != model.TaskStatusSkipped {
			priority := "normal"
			if task.Attempts > 1 {
				priority = "critical"
			}
			o.handoffBuilder.AddNextAction(
				fmt.Sprintf("Complete task: %s — %s", task.ID, task.Title),
				priority,
				task.Description,
			)
		}
	}

	artifact := o.handoffBuilder.Build(string(session.Status))

	// 持久化
	if err := o.handoffStore.Save(artifact); err != nil {
		o.logEvent("handoff_save_error", err.Error(), nil)
	} else {
		o.logEvent("handoff_saved",
			fmt.Sprintf("Handoff artifact saved for session %s", session.ID), nil)
	}
}

// ==================== Phase 5: Session Grading ====================

// gradeAndEmit 在 session 结束后计算评分并推送事件
func (o *HarnessOrchestrator) gradeAndEmit(session *model.HarnessSession, onEvent EventCallback) {
	if o.grader == nil {
		return
	}

	// 收集评分输入数据
	var evalScores []int
	totalAttempts := 0
	for _, task := range session.Tasks {
		totalAttempts += task.Attempts
		for _, ev := range task.EvalHistory {
			evalScores = append(evalScores, ev.Score)
		}
	}

	budgetStatus := o.budget.Status()
	budgetCfg := harness.DefaultBudgetConfig()

	input := harness.SessionGradeInput{
		SessionID:      session.ID,
		TotalTasks:     len(session.Tasks),
		CompletedTasks: session.Metrics.CompletedTasks,
		FailedTasks:    session.Metrics.FailedTasks,
		TotalAttempts:  totalAttempts,
		RePlanCount:    session.RePlanCount,
		MaxRePlans:     o.config.MaxRePlans,
		EvalScores:     evalScores,
		TotalTokens:    budgetStatus.TotalTokensUsed,
		TokenBudget:    budgetCfg.MaxTotalTokens,
		TotalDuration:  time.Duration(budgetStatus.ElapsedTime),
		TimeBudget:     budgetCfg.MaxSessionTime,
		ErrorCount:     session.Metrics.FailedTasks,
		CircuitTripped: false, // 从 middleware 获取
	}

	grade := o.grader.Grade(input)

	o.emit(onEvent, model.EventMetrics, "Session Grade", map[string]interface{}{
		"session_id":      grade.SessionID,
		"overall_score":   grade.OverallScore,
		"verdict":         grade.Verdict,
		"dimensions":      grade.Dimensions,
		"violations":      grade.Violations,
		"recommendations": grade.Recommendations,
	})

	o.logEvent("session_grade",
		fmt.Sprintf("score=%.1f verdict=%s violations=%d",
			grade.OverallScore, grade.Verdict, len(grade.Violations)), grade)
}

// ==================== Re-plan 决策 ====================

func (o *HarnessOrchestrator) shouldRePlan(session *model.HarnessSession) bool {
	if session.RePlanCount >= o.config.MaxRePlans {
		return false
	}
	completed := o.countCompleted(session)
	if len(session.Tasks) > 0 && float64(completed)/float64(len(session.Tasks)) > 0.5 {
		return false
	}
	return true
}

func (o *HarnessOrchestrator) buildRePlanContext(session *model.HarnessSession) string {
	ctx := fmt.Sprintf("原始目标：%s\n\n", session.Goal)
	ctx += "之前的执行情况：\n"
	for _, t := range session.Tasks {
		ctx += fmt.Sprintf("- [%s] %s (状态: %s)", t.ID, t.Title, t.Status)
		if t.Result != "" {
			r := t.Result
			if len(r) > 200 {
				r = r[:200] + "..."
			}
			ctx += fmt.Sprintf(" → 结果摘要: %s", r)
		}
		if len(t.EvalHistory) > 0 {
			lastEval := t.EvalHistory[len(t.EvalHistory)-1]
			ctx += fmt.Sprintf(" → 最后评分: %d, 反馈: %s", lastEval.Score, lastEval.Feedback)
		}
		ctx += "\n"
	}
	ctx += "\n请根据以上信息，重新规划剩余需要完成的子任务。已完成的不需要重做。"
	return ctx
}

// ==================== 工具方法 ====================

func (o *HarnessOrchestrator) emit(onEvent EventCallback, t model.EventType, msg string, data interface{}) {
	onEvent(model.HarnessEvent{Type: t, Message: msg, Data: data})
}

func (o *HarnessOrchestrator) persist(session *model.HarnessSession) {
	session.UpdatedAt = time.Now()
	o.store.Save(session)
}

func (o *HarnessOrchestrator) countCompleted(session *model.HarnessSession) int {
	c := 0
	for _, t := range session.Tasks {
		if t.Status == model.TaskStatusCompleted {
			c++
		}
	}
	return c
}

func (o *HarnessOrchestrator) logEvent(eventType, message string, data interface{}) {
	if o.logger != nil {
		o.logger.LogEvent(eventType, message, data)
	}
}

func (o *HarnessOrchestrator) finalizeLog(session *model.HarnessSession) {
	if o.logger == nil {
		return
	}
	budgetStatus := o.budget.Status()
	_, _ = o.logger.Finalize(
		string(session.Status),
		o.middleware.GetCallLog(),
		o.progress.GetEntries(),
		&budgetStatus,
		o.middleware.GetCallStats(),
	)
}

// ==================== 暴露访问器 ====================

func (o *HarnessOrchestrator) GetMiddleware() *harness.MiddlewareChain { return o.middleware }
func (o *HarnessOrchestrator) GetBudget() *harness.BudgetTracker       { return o.budget }
func (o *HarnessOrchestrator) GetProgress() *harness.ProgressTracker   { return o.progress }
func (o *HarnessOrchestrator) GetFeatures() *harness.FeatureTracker    { return o.features }
func (o *HarnessOrchestrator) GetHandoffStore() *harness.HandoffStore  { return o.handoffStore }
func (o *HarnessOrchestrator) GetTrace() *harness.TraceCollector       { return o.trace }
func (o *HarnessOrchestrator) GetGrader() *harness.SessionGrader       { return o.grader }
