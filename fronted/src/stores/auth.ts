import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/api'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(localStorage.getItem('token'))
  const username = ref<string | null>(localStorage.getItem('username'))
  const email = ref<string | null>(localStorage.getItem('email'))

  const isLoggedIn = computed(() => !!token.value)

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
