# Agentic Learning

一站式 AI 智能体学习与实践平台 — Monorepo 架构。

## 项目结构

| 目录 | 语言 | 说明 |
|------|------|------|
| `gateway/` | Go | AI 调用网关服务（OpenAI-Compatible API） |
| `services/` | Java / TS / … | 预留：后续业务服务 |

## 快速开始

```bash
# 启动 AI Gateway
cd gateway
cp config.yaml config.local.yaml   # 修改你的 API Key 等配置
make run
```

## License

MIT
