# AI Gateway

基于 Golang 的 AI 调用网关，集成智谱 AI（GLM 系列模型），对外提供 OpenAI-Compatible API。

## 特性

- 🤖 **智谱 AI** — 集成 GLM-4-Flash / GLM-4-Plus 等模型
- 🌊 **流式支持** — SSE (Server-Sent Events) 实时输出
- 🔐 **API Key 认证** — Bearer Token 鉴权
- ⚡ **限流保护** — 令牌桶算法
- 📝 **结构化日志** — Zap 高性能日志
- ⚙️ **配置驱动** — YAML + 环境变量

## 快速开始

```bash
# 安装依赖
make deps

# 配置智谱 API Key（二选一）
# 方式一：修改 config.yaml
vim config.yaml

# 方式二：设置环境变量
export ZHIPU_API_KEY="your-api-key-here"

# 启动服务
make run
```

## API

### 健康检查

```bash
curl http://localhost:8080/health
```

### 聊天对话

```bash
# 非流式
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-gateway-change-me" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-4-flash",
    "messages": [{"role": "user", "content": "你好！"}]
  }'

# 流式
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer sk-gateway-change-me" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "glm-4-flash",
    "messages": [{"role": "user", "content": "你好！"}],
    "stream": true
  }'
```

## 项目结构

```
gateway/
├── cmd/server/main.go          # 服务入口
├── internal/
│   ├── config/config.go        # 配置加载
│   ├── handler/chat.go         # HTTP Handler
│   ├── middleware/              # 中间件（认证、限流、日志）
│   ├── model/types.go          # 数据结构
│   ├── provider/
│   │   ├── provider.go         # Provider 接口
│   │   └── zhipu.go            # 智谱 AI 适配器
│   └── router/router.go        # 路由注册
└── config.yaml                 # 配置文件
```
