import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    proxy: {
      '/api/v1/harness': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, ''),
        // SSE 代理：关闭缓冲，确保事件实时推送到前端
        configure: (proxy) => {
          proxy.on('proxyRes', (proxyRes) => {
            // 如果后端返回 SSE，确保 http-proxy 不缓冲
            if (proxyRes.headers['content-type']?.includes('text/event-stream')) {
              proxyRes.headers['cache-control'] = 'no-cache'
              proxyRes.headers['x-accel-buffering'] = 'no'
            }
          })
        },
      },
      '/api': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
    },
  },
})
