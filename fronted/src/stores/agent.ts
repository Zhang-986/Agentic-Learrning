import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface SubTask {
  id: string
  title: string
  description: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  result?: string
}

export interface EvaluationResult {
  score: number
  feedback: string
  passed: boolean
}

export interface HarnessEvent {
  type: 'info' | 'task_start' | 'task_eval' | 'task_complete' | 'session_complete' | 'error'
  message: string
  data?: any
}

export const useAgentStore = defineStore('agent', () => {
  const isRunning = ref(false)
  const goal = ref('')
  const tasks = ref<SubTask[]>([])
  const events = ref<HarnessEvent[]>([])
  const currentSessionId = ref<string | null>(null)
  const currentTaskId = ref<string | null>(null)

  function reset() {
    isRunning.value = false
    tasks.value = []
    events.value = []
    currentSessionId.value = null
    currentTaskId.value = null
  }

  async function startHarness(userGoal: string, provider: string = 'zhipu') {
    reset()
    goal.value = userGoal
    isRunning.value = true

    // SSE Request using fetch (EventSource doesn't support POST and custom headers easily)
    try {
      const response = await fetch('/api/v1/harness/run', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer sk-gateway-change-me` // Using gateway's API key
        },
        body: JSON.stringify({ goal: userGoal, provider })
      })

      if (!response.ok) {
        throw new Error('Failed to start agent harness')
      }

      const reader = response.body?.getReader()
      if (!reader) throw new Error('No reader available')

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || ''

        for (const line of lines) {
          if (line.startsWith('data:')) {
            const dataStr = line.slice(5).trim()
            if (!dataStr) continue
            try {
              const event: HarnessEvent = JSON.parse(dataStr)
              handleEvent(event)
            } catch (e) {
              console.error('Failed to parse SSE event', e)
            }
          }
        }
      }
    } catch (error: any) {
      events.value.push({
        type: 'error',
        message: error.message || 'Unknown error occurred'
      })
    } finally {
      isRunning.value = false
    }
  }

  function handleEvent(event: HarnessEvent) {
    events.value.push(event)

    switch (event.type) {
      case 'info':
        if (event.message.includes('Planner 拆解完成')) {
          tasks.value = event.data || []
        }
        break
      case 'task_start':
        currentTaskId.value = event.data?.id
        updateTaskStatus(event.data?.id, 'running')
        break
      case 'task_eval':
        // Handle evaluation feedback if needed
        break
      case 'task_complete':
        updateTaskStatus(event.data?.id, event.data?.status, event.data?.result)
        break
      case 'session_complete':
        currentSessionId.value = event.data?.id
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
    isRunning, 
    goal, 
    tasks, 
    events, 
    currentSessionId, 
    currentTaskId, 
    startHarness, 
    reset 
  }
})
