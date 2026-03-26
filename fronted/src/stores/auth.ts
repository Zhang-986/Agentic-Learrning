import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/api'

// JWT payload 结构（仅取 exp 字段）
interface JwtPayload {
  exp?: number
}

/**
 * 解析 JWT token 的 payload 部分（不做签名验证，仅用于读取过期时间）
 */
function parseJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return null
    const payload = JSON.parse(atob(parts[1]))
    return payload as JwtPayload
  } catch {
    return null
  }
}

/**
 * 检查 JWT 是否已过期
 * @param token JWT token string
 * @param bufferSeconds 提前多少秒视为过期（默认 60s），避免边界请求
 */
function isTokenExpired(token: string, bufferSeconds: number = 60): boolean {
  const payload = parseJwtPayload(token)
  if (!payload?.exp) return true // 无法解析或没有 exp 字段 → 视为过期
  const nowSeconds = Math.floor(Date.now() / 1000)
  return nowSeconds >= (payload.exp - bufferSeconds)
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const username = ref<string | null>(localStorage.getItem('username'))
  const email = ref<string | null>(localStorage.getItem('email'))

  // 启动时校验：如果 token 已过期，直接清除
  if (token.value && isTokenExpired(token.value)) {
    token.value = null
    username.value = null
    email.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    localStorage.removeItem('email')
  }

  const isLoggedIn = computed(() => {
    if (!token.value) return false
    // 每次访问 isLoggedIn 时都检查过期
    if (isTokenExpired(token.value)) {
      // 自动清理过期 token
      logout()
      return false
    }
    return true
  })

  async function login(loginEmail: string, password: string) {
    const { data } = await authApi.login({ email: loginEmail, password })
    setAuth(data)
    return data
  }

  async function register(regUsername: string, regEmail: string, password: string, code: string) {
    const { data } = await authApi.register({
      username: regUsername,
      email: regEmail,
      password,
      code,
    })
    setAuth(data)
    return data
  }

  function setAuth(data: { token: string; username: string; email: string }) {
    token.value = data.token
    username.value = data.username
    email.value = data.email
    localStorage.setItem('token', data.token)
    localStorage.setItem('username', data.username)
    localStorage.setItem('email', data.email)
  }

  function logout() {
    token.value = null
    username.value = null
    email.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    localStorage.removeItem('email')
  }

  return { token, username, email, isLoggedIn, login, register, logout }
})
