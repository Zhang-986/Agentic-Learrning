import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// ==================== 类型定义 ====================

export interface SubTask {
  id: string
  title: string
  description: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'skipped'
  result?: string
  attempts?: number
  latency_ms?: number
  eval_history?: EvaluationResult[]
}

export interface EvaluationResult {
  score: number
  feedback: string
  passed: boolean
  attempt?: number
}

export interface SessionMetrics {
  total_tasks: number
  completed_tasks: number
  failed_tasks: number
  total_retries: number
  re_plan_count: number
  total_latency_ms: number
}

export interface BudgetStatus {
  total_tokens_used: number
  total_tokens_limit: number
  token_utilization: number
  llm_calls: number
  llm_calls_limit: number
  elapsed_time: number
  time_limit: number
  time_utilization: number
  exhausted: boolean
  reason?: string
}

export interface FeatureProgress {
  total: number
  passed: number
  remaining: number
  completion_rate: number
}

// Phase 4: Handoff Artifact 类型
export interface HandoffArtifact {
  session_id: string
  goal: string
  final_status: string
  what_changed: { task_id: string; description: string; evidence?: string }[]
  what_failed: { task_id: string; error: string; root_cause?: string; attempts: number }[]
  decisions: { decision: string; rationale: string; task_id?: string }[]
  next_actions: { action: string; priority: string; context?: string }[]
  do_not_repeat: string[]
  environment: { healthy: boolean; last_error?: string; open_issues: number }
}

// Phase 5: Trace + Grade 类型
export interface TraceInfo {
  trace_id: string
  total_spans: number
  iterations: number
  total_duration: string
  total_input: number
  total_output: number
  error_spans: number
  bottleneck: string
  by_agent: Record<string, AgentStats>
}

export interface AgentStats {
  calls: number
  total_latency: number
  avg_latency: number
  input_tokens: number
  output_tokens: number
  errors: number
}

export interface SessionGrade {
  session_id: string
  overall_score: number
  verdict: 'pass' | 'warn' | 'fail'
  dimensions: {
    task_completion: number
    eval_quality: number
    efficiency: number
    reliability: number
    plan_adherence: number
  }
  violations: string[]
  recommendations: string[]
}

export interface HarnessEvent {
  type: 'info' | 'task_start' | 'task_eval' | 'task_complete'
      | 'session_complete' | 'error' | 're_plan' | 'metrics'
  message: string
  data?: any
}

// ==================== Store ====================

export const useAgentStore = defineStore('agent', () => {
  // 核心状态
  const isRunning = ref(false)
  const goal = ref('')
  const tasks = ref<SubTask[]>([])
  const events = ref<HarnessEvent[]>([])
  const currentSessionId = ref<string | null>(null)
  const currentTaskId = ref<string | null>(null)
  const error = ref<string | null>(null)

  // Phase 1+2+3: Harness 可观测性
  const sessionMetrics = ref<SessionMetrics | null>(null)
  const budgetStatus = ref<BudgetStatus | null>(null)
  const featureProgress = ref<FeatureProgress | null>(null)
  const callStats = ref<Record<string, any> | null>(null)
  const rePlanCount = ref(0)

  // Phase 4: Handoff 状态
  const lastHandoff = ref<HandoffArtifact | null>(null)
  const primingLoaded = ref(false)

  // Phase 5: Trace + Grade 状态
  const traceInfo = ref<TraceInfo | null>(null)
  const sessionGrade = ref<SessionGrade | null>(null)

  // SSE 连接管理：AbortController 防止页面导航/组件卸载时连接泄漏
  let currentAbortController: AbortController | null = null

  // 计算属性
  const completionRate = computed(() => {
    if (!sessionMetrics.value || sessionMetrics.value.total_tasks === 0) return 0
    return Math.round(
      (sessionMetrics.value.completed_tasks / sessionMetrics.value.total_tasks) * 100
    )
  })

  const tokenUtilization = computed(() => {
    if (!budgetStatus.value) return 0
    return Math.round(budgetStatus.value.token_utilization * 100)
  })

  // Phase 4: 从 handoff 计算下一步建议
  const nextActions = computed(() => {
    if (!lastHandoff.value) return []
    return lastHandoff.value.next_actions || []
  })

  const lessonsLearned = computed(() => {
    if (!lastHandoff.value) return []
    return lastHandoff.value.do_not_repeat || []
  })

  // Phase 5: 从 grade 提取
  const gradeVerdict = computed(() => sessionGrade.value?.verdict ?? null)
  const gradeScore = computed(() => sessionGrade.value?.overall_score ?? null)
  const gradeViolations = computed(() => sessionGrade.value?.violations ?? [])
  const gradeRecommendations = computed(() => sessionGrade.value?.recommendations ?? [])
  const traceBottleneck = computed(() => traceInfo.value?.bottleneck ?? null)

  function reset() {
    // 先中止正在进行的 SSE 连接
    abort()

    isRunning.value = false
    tasks.value = []
    events.value = []
    currentSessionId.value = null
    currentTaskId.value = null
    error.value = null
    sessionMetrics.value = null
    budgetStatus.value = null
    featureProgress.value = null
    callStats.value = null
    rePlanCount.value = 0
    lastHandoff.value = null
    primingLoaded.value = false
    traceInfo.value = null
    sessionGrade.value = null
  }

  /**
   * 中止当前 SSE 连接。
   * 在页面导航、组件卸载、或 reset 时调用，防止连接泄漏。
   */
  function abort() {
    if (currentAbortController) {
      currentAbortController.abort()
      currentAbortController = null
    }
  }

  // ==================== 执行新会话 ====================

  async function startHarness(userGoal: string, provider: string = 'zhipu') {
    reset()
    goal.value = userGoal
    isRunning.value = true

    try {
      await streamEndpoint('/api/v1/harness/run', {
        goal: userGoal,
        provider,
      })
    } catch (err: any) {
      // AbortError 是用户主动取消，不视为错误
      if (err.name === 'AbortError') return
      error.value = err.message
      events.value.push({ type: 'error', message: err.message || '未知错误' })
    } finally {
      isRunning.value = false
      currentAbortController = null
    }
  }

  // ==================== 恢复中断会话 ====================

  async function resumeHarness(sessionId: string, provider: string = 'zhipu') {
    isRunning.value = true
    error.value = null

    try {
      await streamEndpoint('/api/v1/harness/resume', {
        session_id: sessionId,
        provider,
      })
    } catch (err: any) {
      if (err.name === 'AbortError') return
      error.value = err.message
      events.value.push({ type: 'error', message: err.message || '恢复失败' })
    } finally {
      isRunning.value = false
      currentAbortController = null
    }
  }

  // ==================== 查询会话状态 ====================

  async function fetchSession(sessionId: string) {
    try {
      const resp = await fetch(`/api/v1/harness/session/${sessionId}`, {
        headers: { 'Authorization': `Bearer test-key-001` },
      })
      if (!resp.ok) throw new Error(`查询失败 (${resp.status})`)
      return await resp.json()
    } catch (err: any) {
      error.value = err.message
      return null
    }
  }

  // ==================== SSE 流式通信 ====================

  async function streamEndpoint(url: string, body: Record<string, any>) {
    // 如果有上一次未结束的连接，先中止
    abort()

    const controller = new AbortController()
    currentAbortController = controller

    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer test-key-001`,
      },
      body: JSON.stringify(body),
      signal: controller.signal,
    })

    if (!response.ok) {
      const errText = await response.text()
      throw new Error(`请求失败 (${response.status}): ${errText}`)
    }

    const reader = response.body?.getReader()
    if (!reader) throw new Error('无法获取响应流')

    const decoder = new TextDecoder()
    let buffer = ''

    try {
      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed) continue

          if (trimmed.startsWith('data:')) {
            const dataStr = trimmed.slice(5).trim()
            if (!dataStr || dataStr === '[DONE]') continue

            try {
              const event: HarnessEvent = JSON.parse(dataStr)
              handleEvent(event)
            } catch (e) {
              console.warn('SSE 解析失败:', dataStr, e)
            }
          }
        }
      }
    } finally {
      reader.releaseLock()
    }
  }

  // ==================== 事件处理 ====================

  function handleEvent(event: HarnessEvent) {
    events.value.push(event)

    switch (event.type) {
      case 'info':
        // Planner 拆解完成时，data 是子任务数组
        if (event.data && Array.isArray(event.data)) {
          tasks.value = event.data.map((t: any) => ({
            ...t,
            status: t.status || 'pending',
          }))
        }
        // Phase 4: 检测 Priming 事件
        if (event.message.includes('Priming') || event.message.includes('交接信息')) {
          primingLoaded.value = true
        }
        break

      case 'task_start':
        if (event.data?.id) {
          currentTaskId.value = event.data.id
          updateTaskStatus(event.data.id, 'running')
        }
        break

      case 'task_eval':
        // 评估反馈展示用，不更新任务状态
        break

      case 'task_complete':
        if (event.data?.id) {
          const status = event.data.status === 'failed' ? 'failed' : 'completed'
          updateTaskStatus(event.data.id, status, event.data.result)
          // 同步执行度量
          if (event.data.attempts) {
            const task = tasks.value.find(t => t.id === event.data.id)
            if (task) {
              task.attempts = event.data.attempts
              task.latency_ms = event.data.latency_ms
              task.eval_history = event.data.eval_history
            }
          }
        }
        break

      case 're_plan':
        rePlanCount.value++
        break

      case 'metrics':
        // 根据 message 区分不同的 metrics 类型
        if (event.message.includes('预算消耗') && event.data) {
          budgetStatus.value = event.data as BudgetStatus
        } else if (event.message.includes('调用统计') && event.data) {
          callStats.value = event.data
        } else if (event.message.includes('功能完成度') && event.data) {
          featureProgress.value = event.data as FeatureProgress
        } else if (event.message.includes('会话度量') && event.data) {
          sessionMetrics.value = event.data as SessionMetrics
        } else if (event.message.includes('Structured Trace') && event.data) {
          // Phase 5: Trace 数据
          traceInfo.value = event.data as TraceInfo
        } else if (event.message.includes('Session Grade') && event.data) {
          // Phase 5: Session Grade 数据
          sessionGrade.value = event.data as SessionGrade
        }
        break

      case 'session_complete':
        if (event.data?.id) {
          currentSessionId.value = event.data.id
        }
        if (event.data?.metrics) {
          sessionMetrics.value = event.data.metrics
        }
        break

      case 'error':
        error.value = event.message
        break
    }
  }

  function updateTaskStatus(id: string, status: SubTask['status'], result?: string) {
    const task = tasks.value.find(t => t.id === id)
    if (task) {
      task.status = status
      if (result) task.result = result
    }
  }

  return {
    // 状态
    isRunning,
    goal,
    tasks,
    events,
    currentSessionId,
    currentTaskId,
    error,
    sessionMetrics,
    budgetStatus,
    featureProgress,
    callStats,
    rePlanCount,
    // Phase 4
    lastHandoff,
    primingLoaded,
    // Phase 5
    traceInfo,
    sessionGrade,
    // 计算
    completionRate,
    tokenUtilization,
    nextActions,
    lessonsLearned,
    gradeVerdict,
    gradeScore,
    gradeViolations,
    gradeRecommendations,
    traceBottleneck,
    // 操作
    startHarness,
    resumeHarness,
    fetchSession,
    reset,
    abort,
  }
})
