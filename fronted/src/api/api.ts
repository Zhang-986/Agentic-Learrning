import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import router from '../router'

const api = axios.create({
  baseURL: '/api',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
})

// 请求拦截器：自动携带 JWT Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：401 自动跳转登录
api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      const authStore = useAuthStore()
      authStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

export default api

// Auth API
export const authApi = {
  sendCode(email: string) {
    return api.post('/auth/send-code', { email })
  },
  register(data: { username: string; email: string; password: string; code: string }) {
    return api.post('/auth/register', data)
  },
  login(data: { email: string; password: string }) {
    return api.post('/auth/login', data)
  },
  getMe() {
    return api.get('/auth/me')
  },
}
