<template>
  <div class="auth-page">
    <!-- Decorative floating shapes -->
    <div class="deco deco-star">★</div>
    <div class="deco deco-circle"></div>
    <div class="deco deco-square"></div>

    <div class="auth-card">
      <!-- Header badge -->
      <div class="badge badge-top">STELLAR</div>

      <div class="auth-header">
        <h1>LOGIN<span class="stroke-text">.</span></h1>
        <p class="subtitle">登录你的知识宇宙</p>
      </div>

      <form @submit.prevent="handleLogin" class="auth-form">
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

        <div class="form-group">
          <label for="password">密码</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            placeholder="••••••••"
            required
            autocomplete="current-password"
          />
        </div>

        <div v-if="error" class="error-msg">{{ error }}</div>

        <button type="submit" class="btn-primary" :disabled="loading">
          {{ loading ? '登录中...' : '登 录 →' }}
        </button>
      </form>

      <div class="auth-footer">
        <span>还没有账号？</span>
        <router-link to="/register" class="link-accent">立即注册 →</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const loading = ref(false)
const error = ref('')

const form = reactive({ email: '', password: '' })

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await authStore.login(form.email, form.password)
    router.push('/')
  } catch (e: any) {
    error.value = e.response?.data?.error || '登录失败，请检查网络连接'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ---- Page ---- */
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  position: relative;
  overflow: hidden;
}

/* ---- Decorative elements ---- */
.deco {
  position: absolute;
  pointer-events: none;
  z-index: 0;
}
.deco-star {
  top: 12%;
  left: 8%;
  font-size: 64px;
  color: var(--neo-secondary);
  animation: spin-slow 10s linear infinite;
}
.deco-circle {
  bottom: 15%;
  right: 10%;
  width: 80px;
  height: 80px;
  border: var(--border);
  border-radius: 50%;
  background: var(--neo-muted);
  box-shadow: var(--shadow-sm);
}
.deco-square {
  top: 20%;
  right: 15%;
  width: 48px;
  height: 48px;
  border: var(--border);
  background: var(--neo-secondary);
  box-shadow: var(--shadow-sm);
  transform: rotate(12deg);
}

@keyframes spin-slow {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* ---- Card ---- */
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

/* ---- Badge ---- */
.badge {
  display: inline-block;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.2em;
  text-transform: uppercase;
  padding: 6px 14px;
  border: 3px solid var(--neo-fg);
  background: var(--neo-secondary);
  box-shadow: var(--shadow-sm);
}
.badge-top {
  position: absolute;
  top: -16px;
  left: 24px;
  transform: rotate(-2deg);
}

/* ---- Header ---- */
.auth-header {
  margin-bottom: 32px;
}
.auth-header h1 {
  font-size: 56px;
  letter-spacing: -2px;
  text-transform: uppercase;
  line-height: 0.9;
}
.stroke-text {
  color: var(--neo-accent);
}
.subtitle {
  font-size: 16px;
  font-weight: 700;
  color: var(--neo-fg);
  margin-top: 10px;
  opacity: 0.6;
}

/* ---- Form ---- */
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.15em;
}

.form-group input {
  width: 100%;
  padding: 14px 16px;
  border: var(--border);
  background: var(--neo-white);
  font-family: var(--font-sans);
  font-size: 16px;
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

/* ---- Error ---- */
.error-msg {
  font-size: 14px;
  font-weight: 700;
  padding: 12px 16px;
  border: 3px solid var(--neo-fg);
  background: var(--neo-accent);
  color: var(--neo-white);
}

/* ---- Button ---- */
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

.btn-primary:hover:not(:disabled) {
  background: #e85d5d;
}

.btn-primary:active:not(:disabled) {
  transform: translate(4px, 4px);
  box-shadow: none;
}

.btn-primary:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* ---- Footer ---- */
.auth-footer {
  text-align: center;
  margin-top: 28px;
  font-size: 14px;
  font-weight: 700;
}

.link-accent {
  color: var(--neo-accent);
  border-bottom: 2px solid var(--neo-accent);
  margin-left: 6px;
  transition: background 0.1s;
  padding: 0 2px;
}
.link-accent:hover {
  background: var(--neo-accent);
  color: var(--neo-white);
}

/* ---- Mobile ---- */
@media (max-width: 480px) {
  .auth-card {
    padding: 40px 24px 32px;
  }
  .auth-header h1 {
    font-size: 42px;
  }
  .deco { display: none; }
}
</style>
