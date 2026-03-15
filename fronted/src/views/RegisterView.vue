<template>
  <div class="auth-page">
    <div class="deco deco-star">★</div>
    <div class="deco deco-rect"></div>

    <div class="auth-card">
      <div class="badge badge-top">NEW ACCOUNT</div>

      <div class="auth-header">
        <h1>SIGN<br/>UP<span class="stroke-text">.</span></h1>
        <p class="subtitle">加入 Stellar，开启知识蒸馏之旅</p>
      </div>

      <form @submit.prevent="handleRegister" class="auth-form">
        <div class="form-group">
          <label for="username">用户名</label>
          <input
            id="username"
            v-model="form.username"
            type="text"
            placeholder="YOUR NAME"
            required
            minlength="2"
            maxlength="32"
            autocomplete="username"
          />
        </div>

        <div class="form-group">
          <label for="email">邮箱</label>
          <input
            id="email"
            v-model="form.email"
            type="email"
            placeholder="YOUR@EMAIL.COM"
            required
            autocomplete="email"
          />
        </div>

        <!-- 验证码 -->
        <div class="form-group">
          <label for="code">验证码</label>
          <div class="code-row">
            <input
              id="code"
              v-model="form.code"
              type="text"
              placeholder="6 位验证码"
              required
              maxlength="6"
              class="code-input"
            />
            <button
              type="button"
              class="btn-send-code"
              :disabled="codeCooldown > 0 || sendingCode"
              @click="handleSendCode"
            >
              {{ sendingCode ? '发送中...' : codeCooldown > 0 ? `${codeCooldown}s` : '发送验证码' }}
            </button>
          </div>
        </div>

        <div class="form-group">
          <label for="password">密码</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            placeholder="MIN 6 CHARS"
            required
            minlength="6"
            autocomplete="new-password"
          />
          <div v-if="form.password" class="strength-row">
            <div class="strength-bar">
              <div class="strength-fill" :style="{ width: strengthPercent + '%' }" :class="strengthClass"></div>
            </div>
            <span class="strength-label" :class="strengthClass">{{ strengthLabel }}</span>
          </div>
        </div>

        <div class="form-group">
          <label for="confirmPassword">确认密码</label>
          <input
            id="confirmPassword"
            v-model="form.confirmPassword"
            type="password"
            placeholder="REPEAT"
            required
            autocomplete="new-password"
          />
        </div>

        <div v-if="error" class="error-msg">{{ error }}</div>
        <div v-if="successMsg" class="success-msg">{{ successMsg }}</div>

        <button type="submit" class="btn-primary" :disabled="loading">
          {{ loading ? '注册中...' : '注 册 →' }}
        </button>
      </form>

      <div class="auth-footer">
        <span>已有账号？</span>
        <router-link to="/login" class="link-accent">立即登录 →</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api/api'

const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const sendingCode = ref(false)
const codeCooldown = ref(0)
const error = ref('')
const successMsg = ref('')

let cooldownTimer: ReturnType<typeof setInterval> | null = null

const form = reactive({
  username: '',
  email: '',
  code: '',
  password: '',
  confirmPassword: '',
})

async function handleSendCode() {
  error.value = ''
  successMsg.value = ''

  if (!form.email || !form.email.includes('@')) {
    error.value = '请先输入有效的邮箱地址'
    return
  }

  sendingCode.value = true
  try {
    await authApi.sendCode(form.email)
    successMsg.value = '验证码已发送到你的邮箱'
    codeCooldown.value = 60
    cooldownTimer = setInterval(() => {
      codeCooldown.value--
      if (codeCooldown.value <= 0 && cooldownTimer) {
        clearInterval(cooldownTimer)
        cooldownTimer = null
      }
    }, 1000)
  } catch (e: any) {
    error.value = e.response?.data?.error || '验证码发送失败'
  } finally {
    sendingCode.value = false
  }
}

const passwordStrength = computed(() => {
  const p = form.password
  if (!p) return 0
  let s = 0
  if (p.length >= 6) s++
  if (p.length >= 10) s++
  if (/[A-Z]/.test(p)) s++
  if (/[0-9]/.test(p)) s++
  if (/[^A-Za-z0-9]/.test(p)) s++
  return s
})

const strengthPercent = computed(() => (passwordStrength.value / 5) * 100)
const strengthClass = computed(() => {
  if (passwordStrength.value <= 1) return 'weak'
  if (passwordStrength.value <= 3) return 'medium'
  return 'strong'
})
const strengthLabel = computed(() => {
  if (passwordStrength.value <= 1) return '弱'
  if (passwordStrength.value <= 3) return '中'
  return '强'
})

async function handleRegister() {
  error.value = ''
  successMsg.value = ''

  if (form.password !== form.confirmPassword) {
    error.value = '两次输入的密码不一致'
    return
  }

  loading.value = true
  try {
    await authStore.register(form.username, form.email, form.password, form.code)
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data?.error || '注册失败，请检查网络连接'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  position: relative;
  overflow: hidden;
}

.deco { position: absolute; pointer-events: none; z-index: 0; }
.deco-star {
  bottom: 10%;
  left: 6%;
  font-size: 72px;
  color: var(--neo-accent);
  animation: spin-slow 12s linear infinite;
}
.deco-rect {
  top: 14%;
  right: 8%;
  width: 100px;
  height: 40px;
  border: var(--border);
  background: var(--neo-muted);
  box-shadow: var(--shadow-sm);
  transform: rotate(-6deg);
}

@keyframes spin-slow {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.auth-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 440px;
  border: var(--border);
  background: var(--neo-white);
  box-shadow: var(--shadow-lg);
  padding: 48px 40px 40px;
}

.badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  padding: 6px 14px;
  border: 3px solid var(--neo-fg);
  background: var(--neo-muted);
  box-shadow: var(--shadow-sm);
}
.badge-top {
  position: absolute;
  top: -16px;
  right: 24px;
  transform: rotate(3deg);
}

.auth-header { margin-bottom: 28px; }
.auth-header h1 {
  font-size: 52px;
  letter-spacing: -2px;
  text-transform: uppercase;
  line-height: 0.9;
}
.stroke-text { color: var(--neo-secondary); }
.subtitle {
  font-size: 15px;
  font-weight: 700;
  margin-top: 12px;
  opacity: 0.6;
}

.auth-form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 5px;
}
.form-group label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
}
.form-group input {
  width: 100%;
  padding: 13px 16px;
  border: var(--border);
  background: var(--neo-white);
  font-family: var(--font-sans);
  font-size: 15px;
  font-weight: 700;
  color: var(--neo-fg);
  outline: none;
  transition: background 0.1s, box-shadow 0.1s;
}
.form-group input:focus {
  background: var(--neo-secondary);
  box-shadow: var(--shadow-sm);
}
.form-group input::placeholder {
  color: rgba(0, 0, 0, 0.25);
  font-weight: 500;
}

/* 验证码行 */
.code-row {
  display: flex;
  gap: 8px;
}
.code-input {
  flex: 1;
}
.btn-send-code {
  flex-shrink: 0;
  padding: 13px 18px;
  border: var(--border);
  background: var(--neo-secondary);
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  cursor: pointer;
  box-shadow: var(--shadow-sm);
  transition: transform 0.1s, box-shadow 0.1s;
  white-space: nowrap;
}
.btn-send-code:hover:not(:disabled) {
  background: #e8c635;
}
.btn-send-code:active:not(:disabled) {
  transform: translate(2px, 2px);
  box-shadow: none;
}
.btn-send-code:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Strength bar */
.strength-row {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
}
.strength-bar {
  flex: 1;
  height: 6px;
  border: 2px solid var(--neo-fg);
  background: var(--neo-white);
}
.strength-fill {
  height: 100%;
  transition: width 0.2s;
}
.strength-fill.weak { background: var(--neo-accent); }
.strength-fill.medium { background: var(--neo-secondary); }
.strength-fill.strong { background: #22c55e; }

.strength-label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  min-width: 24px;
}
.strength-label.weak { color: var(--neo-accent); }
.strength-label.medium { color: #b8860b; }
.strength-label.strong { color: #16a34a; }

.error-msg {
  font-size: 14px;
  font-weight: 700;
  padding: 12px 16px;
  border: 3px solid var(--neo-fg);
  background: var(--neo-accent);
  color: var(--neo-white);
}

.success-msg {
  font-size: 14px;
  font-weight: 700;
  padding: 12px 16px;
  border: 3px solid var(--neo-fg);
  background: #22c55e;
  color: var(--neo-white);
}

.btn-primary {
  width: 100%;
  padding: 16px 24px;
  border: var(--border);
  background: var(--neo-accent);
  color: var(--neo-white);
  font-family: var(--font-sans);
  font-size: 15px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  cursor: pointer;
  box-shadow: var(--shadow-md);
  transition: transform 0.1s, box-shadow 0.1s;
}
.btn-primary:hover:not(:disabled) { background: #e85d5d; }
.btn-primary:active:not(:disabled) {
  transform: translate(4px, 4px);
  box-shadow: none;
}
.btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

.auth-footer {
  text-align: center;
  margin-top: 24px;
  font-size: 14px;
  font-weight: 700;
}
.link-accent {
  color: var(--neo-accent);
  border-bottom: 2px solid var(--neo-accent);
  margin-left: 6px;
  padding: 0 2px;
  transition: background 0.1s;
}
.link-accent:hover {
  background: var(--neo-accent);
  color: var(--neo-white);
}

@media (max-width: 480px) {
  .auth-card { padding: 40px 24px 32px; }
  .auth-header h1 { font-size: 40px; }
  .deco { display: none; }
  .code-row { flex-direction: column; }
}
</style>
