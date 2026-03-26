<template>
  <div class="harness-page">
    <!-- Nav -->
    <nav class="top-nav">
      <div class="nav-logo" @click="router.push('/')">★ STELLAR</div>
      <div class="nav-right">
        <span class="nav-user">{{ authStore.username }}</span>
        <button class="btn-outline" @click="handleLogout">登出</button>
      </div>
    </nav>

    <main class="content">
      <header class="page-header">
        <div class="badge">AGENTIC HARNESS</div>
        <h1>AI 思考实验室<span class="dot">.</span></h1>
        <p>基于 Long-Running Agent Harness 架构的主动式思考引擎</p>
      </header>

      <!-- Input Section -->
      <section class="input-card card">
        <h3>设定你的学习目标</h3>
        <div class="input-group">
          <textarea 
            v-model="goalInput" 
            placeholder="例如：深入研究量子计算在金融加密中的应用，并生成 3 个核心洞察卡片"
            :disabled="agentStore.isRunning"
          ></textarea>
          <button 
            class="btn-primary" 
            @click="handleStart" 
            :disabled="!goalInput || agentStore.isRunning"
          >
            {{ agentStore.isRunning ? 'AI 正在思考中...' : '启动 AI 引擎' }}
          </button>
        </div>
      </section>

      <div class="workspace-grid" v-if="agentStore.isRunning || agentStore.tasks.length > 0">
        <!-- Tasks / Planner -->
        <section class="tasks-column">
          <h2 class="section-title">PLANNER 规划清单</h2>
          <div class="task-list">
            <div 
              v-for="task in agentStore.tasks" 
              :key="task.id"
              class="task-item card"
              :class="{ 
                'status-running': task.status === 'running',
                'status-completed': task.status === 'completed',
                'status-failed': task.status === 'failed'
              }"
            >
              <div class="task-header">
                <span class="task-id">#{{ task.id }}</span>
                <span class="task-badge">{{ task.status.toUpperCase() }}</span>
              </div>
              <h4>{{ task.title }}</h4>
              <p>{{ task.description }}</p>
              <div v-if="task.result" class="task-result card-inner">
                <strong>执行结果：</strong>
                <p>{{ task.result }}</p>
              </div>
            </div>
          </div>
        </section>

        <!-- Live Events Log -->
        <section class="events-column">
          <h2 class="section-title">LIVE THINKING LOG</h2>
          <div class="events-log card shadow-inner">
            <div v-if="agentStore.events.length === 0" class="empty-log">
              等待引擎启动...
            </div>
            <div 
              v-for="(event, index) in agentStore.events" 
              :key="index"
              class="event-entry"
              :class="'event-' + event.type"
            >
              <span class="event-time">[{{ formatTime() }}]</span>
              <span class="event-type">[{{ event.type.toUpperCase() }}]</span>
              <span class="event-msg">{{ event.message }}</span>
              
              <!-- Eval Detail -->
              <div v-if="event.type === 'task_eval'" class="eval-detail">
                <div class="eval-score" :class="{ 'score-low': event.data.score < 60 }">
                  得分: {{ event.data.score }}
                </div>
                <div class="eval-feedback">建议: {{ event.data.feedback }}</div>
              </div>
            </div>
          </div>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useAgentStore } from '../stores/agent'

const router = useRouter()
const authStore = useAuthStore()
const agentStore = useAgentStore()

const goalInput = ref('')

async function handleStart() {
  if (!goalInput.value) return
  await agentStore.startHarness(goalInput.value)
}

function handleLogout() {
  authStore.logout()
  router.push('/login')
}

function formatTime() {
  return new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
</script>

<style scoped>
.harness-page {
  min-height: 100vh;
  padding-bottom: 4rem;
}

.top-nav {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.5rem 2rem;
  background: var(--neo-white);
  border-bottom: var(--border);
  position: sticky;
  top: 0;
  z-index: 100;
}

.nav-logo {
  font-size: 1.5rem;
  font-weight: 900;
  letter-spacing: -1px;
  cursor: pointer;
}

.nav-user {
  font-weight: 700;
  margin-right: 1.5rem;
}

.content {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.page-header {
  margin-bottom: 3rem;
}

.badge {
  display: inline-block;
  background: var(--neo-secondary);
  padding: 0.25rem 0.75rem;
  border: var(--border);
  font-weight: 700;
  font-size: 0.75rem;
  margin-bottom: 1rem;
}

h1 { font-size: 3.5rem; margin-bottom: 0.5rem; }
.dot { color: var(--neo-accent); }

/* Card Styles */
.card {
  background: var(--neo-white);
  border: var(--border);
  box-shadow: var(--shadow-md);
  padding: 1.5rem;
  transition: transform 0.2s;
}

.card:hover {
  transform: translate(-2px, -2px);
  box-shadow: var(--shadow-lg);
}

.input-card {
  margin-bottom: 3rem;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  margin-top: 1rem;
}

textarea {
  width: 100%;
  height: 120px;
  padding: 1rem;
  border: var(--border);
  font-family: inherit;
  font-size: 1.1rem;
  background: var(--neo-bg);
  resize: none;
}

textarea:focus {
  outline: none;
  background: white;
}

.btn-primary {
  align-self: flex-end;
  background: var(--neo-accent);
  color: white;
  padding: 1rem 2rem;
  border: var(--border);
  box-shadow: var(--shadow-sm);
  font-weight: 700;
  font-size: 1.1rem;
  cursor: pointer;
}

.btn-primary:active {
  transform: translate(2px, 2px);
  box-shadow: none;
}

.btn-primary:disabled {
  background: #ccc;
  cursor: not-allowed;
  transform: none;
  box-shadow: var(--shadow-sm);
}

.btn-outline {
  background: transparent;
  padding: 0.5rem 1rem;
  border: var(--border);
  font-weight: 700;
  cursor: pointer;
}

/* Grid Layout */
.workspace-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 2rem;
}

.section-title {
  font-size: 1.25rem;
  margin-bottom: 1.5rem;
  text-transform: uppercase;
  letter-spacing: 1px;
}

/* Task List */
.task-list {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.task-item {
  position: relative;
}

.task-header {
  display: flex;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.task-id { font-weight: 700; color: #666; }
.task-badge {
  font-size: 0.7rem;
  font-weight: 900;
  padding: 0.1rem 0.4rem;
  border: 2px solid currentColor;
}

.status-running { border-color: var(--neo-secondary); background: #fffdeb; }
.status-completed { border-color: #4ade80; background: #f0fdf4; }
.status-failed { border-color: var(--neo-accent); background: #fef2f2; }

.task-result {
  margin-top: 1rem;
  padding: 1rem;
  background: rgba(0,0,0,0.05);
  border: 2px dashed #000;
  font-size: 0.9rem;
}

/* Events Log */
.events-log {
  height: 600px;
  overflow-y: auto;
  background: #1a1a1a;
  color: #00ff00;
  padding: 1.5rem;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.85rem;
  border: var(--border);
  box-shadow: var(--shadow-md);
}

.event-entry {
  margin-bottom: 0.75rem;
  line-height: 1.4;
}

.event-time { color: #888; margin-right: 0.5rem; }
.event-type { font-weight: 700; margin-right: 0.5rem; }

.event-task_start { color: var(--neo-secondary); }
.event-task_eval { color: var(--neo-muted); }
.event-task_complete { color: #4ade80; }
.event-error { color: var(--neo-accent); }

.eval-detail {
  margin: 0.5rem 0 0.5rem 2rem;
  padding: 0.5rem;
  border-left: 2px solid #555;
  background: rgba(255,255,255,0.05);
}

.eval-score { font-weight: 900; color: #fff; }
.score-low { color: var(--neo-accent); }

.empty-log {
  color: #666;
  text-align: center;
  margin-top: 100px;
}

/* Shadow Inner */
.shadow-inner {
  box-shadow: inset 4px 4px 0px 0px rgba(0,0,0,0.5);
}
</style>
