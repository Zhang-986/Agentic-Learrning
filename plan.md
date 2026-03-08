# AI 个人知识蒸馏助手 - 完整技术方案

> **项目名称**：Stellar Ionosphere（星辰电离层）
> **定位**：主动式 AI 驱动的个人知识管理系统
> **核心理念**：AI 不等待用户提问，而是持续感知、自主思考、主动行动

---

## 一、项目概述

### 1.1 解决的痛点

每个知识工作者都面临**信息过载**问题：

| 现状 | 痛点 |
|------|------|
| 每天刷 B站、公众号、微信读书 | 看完就忘，知识留存率 < 10% |
| 收藏了 500+ 文章 | "收藏 = 学过"的自欺欺人 |
| 开了很多会、听了很多课 | 信息散落在各个平台，无法串联 |
| 想找"之前看过的某个观点" | 翻遍全网找不到，或忘了在哪看的 |

### 1.2 核心价值

对接用户的所有信息来源，**AI 自动完成 `信息 → 知识 → 洞察` 的蒸馏过程**：

1. **蒸馏核心信息**：自动生成结构化知识卡片
2. **跨源关联**：自动发现不同来源之间的知识关联
3. **私人问答**：基于个人知识库的 RAG 问答助理
4. **主动推送**：AI 主动发现洞察、提醒复习、预测需求

### 1.3 与市面产品的本质区别

```
Notion/Obsidian：被动式笔记工具 → 用户自己整理
本项目：         主动式 AI 助手 → AI 替用户整理、思考、行动
```

---

## 二、主动式 AI 核心设计

### 2.1 AI 思考循环（本项目灵魂）

区别于传统"用户提问 → AI 回答"的被动模式，本系统的 AI 拥有**自己的持续思考循环**：

```
┌─────────────────────┐
│   AI 思考循环引擎     │ ← 永不停止的后台大脑
└──────────┬──────────┘
           │
  ┌────────┼────────┐
  ▼        ▼        ▼
感知     思考      行动
Sense   Think     Act
  │        │        │
  └────────┼────────┘
           ▼
         反思
        Reflect
```

- **感知**：持续监测用户行为和信息流（浏览器插件、RSS、平台同步）
- **思考**：LLM 自主推理——这条信息重要吗？和已有知识有关联吗？用户可能需要什么？
- **行动**：生成卡片、建立关联、推送洞察、提出建议
- **反思**：采集用户反馈，调整行为策略（学习什么时候该说、什么时候该闭嘴）

### 2.2 多 Agent 协作体系

```
┌──────────────────────────────────────────────┐
│              AI Agent 引擎                    │
│                                              │
│  ┌────────────┐  ┌────────────┐              │
│  │ 价值评估     │  │ 关联发现    │              │
│  │ Agent      │  │ Agent     │              │
│  │ 评估新信息   │  │ 发现跨源    │              │
│  │ 是否值得存储  │  │ 知识关联    │              │
│  └────────────┘  └────────────┘              │
│  ┌────────────┐  ┌────────────┐              │
│  │ 学习状态     │  │ 认知冲突    │              │
│  │ Agent      │  │ Agent     │              │
│  │ 追踪掌握    │  │ 发现知识库   │              │
│  │ 程度和遗忘   │  │ 中矛盾观点   │              │
│  └────────────┘  └────────────┘              │
│  ┌────────────┐  ┌────────────┐              │
│  │ 推送决策     │  │ 自我反思    │              │
│  │ Agent      │  │ Agent     │              │
│  │ 判断何时     │  │ 分析用户    │              │
│  │ 以何种方式   │  │ 反馈调整    │              │
│  │ 推送给用户   │  │ 自身策略    │              │
│  └────────────┘  └────────────┘              │
│                                              │
│  Agent 调度器（事件触发 + 定时巡检 混合模式）    │
└──────────────────────────────────────────────┘
```

### 2.3 信息重要性评估模型（5 维评分）

```
总分 = w1·显式信号 + w2·行为信号 + w3·内容信号 + w4·关联信号 + w5·时效信号
```

| 维度 | 信号示例 | 权重 |
|------|---------|------|
| **显式信号** | 用户主动收藏/标记/导入 | 最高，直接反映意图 |
| **行为信号** | 阅读停留时长、反复查看、复制分享 | 高，隐式兴趣投票 |
| **内容信号** | LLM 评估信息类型、密度、独特性、可操作性 | 中，内容本身质量 |
| **关联信号** | 是否填补知识缺口、连接孤立知识簇 | 高，对用户知识体系的价值 |
| **时效信号** | 信息时效性、用户当前目标相关度 | 中，动态调整 |

不确定时的处理策略：
- 评分 > 80 → 自动存入正式库
- 20 ~ 80 → 存入「待确认区」，后续观察是否被引用
- < 20 → 丢弃
- 用户主动操作 → 无条件遵从

---

## 三、系统架构

### 3.1 整体架构（Java + Go 多语言微服务）

```
                        用户
                         │
              ┌──────────▼───────────┐
              │    前端 (Vue 3)       │
              │  知识看板 · 问答 · 图谱 │
              └──────────┬───────────┘
                         │
  ┌──────────────────────▼───────────────────────┐
  │           Spring Boot 主服务 (Java)            │
  │                                               │
  │  ┌──────────┐ ┌──────────┐ ┌──────────┐      │
  │  │数据源接入  │ │知识管理   │ │问答引擎   │      │
  │  │Service   │ │Service   │ │(RAG)     │      │
  │  └──────────┘ └──────────┘ └──────────┘      │
  │  ┌──────────┐ ┌──────────┐ ┌──────────┐      │
  │  │蒸馏引擎   │ │关联引擎   │ │推送引擎   │      │
  │  │(异步MQ)  │ │(图分析)   │ │(智能推送) │      │
  │  └──────────┘ └──────────┘ └──────────┘      │
  │  ┌──────────────────────────────────┐         │
  │  │      🧠 AI Agent 引擎            │         │
  │  │  多 Agent 调度 · 思考循环 · 反思   │         │
  │  └──────────────────────────────────┘         │
  │                                               │
  │         需要调 LLM 时 ──┐                      │
  └─────────────────────────┼─────────────────────┘
                            │ gRPC
                 ┌──────────▼───────────┐
                 │   Go AI Gateway      │
                 │                      │
                 │ 智能路由 · 限流熔断    │
                 │ 语义缓存 · Token计量  │
                 │ 多Key轮转 · 降级重试  │
                 └──┬──────┬──────┬─────┘
                    │      │      │
               ┌────▼─┐ ┌─▼────┐ ┌▼───────┐
               │OpenAI│ │Deep  │ │Claude  │
               │      │ │Seek  │ │        │
               └──────┘ └──────┘ └────────┘


                  基础设施层
  ┌──────────────────────────────────────────┐
  │  MySQL · Redis · Milvus · RabbitMQ       │
  │  XXL-JOB · Neo4j(可选) · MinIO(文件)     │
  └──────────────────────────────────────────┘
```

### 3.2 技术选型 × 面试考点映射

| 层级 | 技术选型 | 面试高频考点 |
|------|---------|------------|
| **框架** | Spring Boot 3 | IoC/AOP、Bean 生命周期、自动配置 |
| **ORM** | MyBatis-Plus | SQL 优化、分页、多表关联 |
| **数据库** | MySQL 8 | 索引优化、事务隔离、慢查询 |
| **缓存** | Redis | 穿透/击穿/雪崩、分布式锁 |
| **消息队列** | RabbitMQ | 消息可靠性、死信队列、消费确认 |
| **向量检索** | Milvus | ANN 索引原理、混合检索 |
| **定时任务** | XXL-JOB | 分布式调度、任务分片 |
| **AI 集成** | Spring AI / LangChain4j | LLM 工程化、Prompt 管理 |
| **AI 网关** | Go + gin/chi | Goroutine 并发、gRPC、流式代理 |
| **服务通信** | gRPC + Protobuf | 序列化、HTTP/2、IDL |
| **设计模式** | 策略/模板/观察者/责任链 | 各模块自然使用 |

### 3.3 为什么网关用 Go 而不是 Java

| 维度 | Go | Java |
|------|-----|------|
| 并发模型 | Goroutine 轻量协程，数万并发 | 线程池开销大 |
| 流式处理 | 原生 io.Reader + Channel | 需要 WebFlux 响应式 |
| 内存占用 | < 30MB | Spring Boot > 200MB |
| 部署 | 单二进制，无需运行时 | 需要 JRE |
| 延迟 | 无 GC 停顿，P99 稳定 | GC 可能抖动 |

> [!IMPORTANT]
> 面试时这是一个绝佳的「技术选型思考」展示点——不是"我就是想用"，而是有明确的技术理由。

---

## 四、各模块详细设计

### 4.1 数据源接入模块

#### 设计模式：策略模式 + 模板方法

```java
// 统一解析接口
public interface DataSourceParser {
    KnowledgeRaw parse(RawContent content);
    SourceType getType();
    boolean supports(String url);
}

// 各平台实现
@Component
public class WebArticleParser implements DataSourceParser { ... }
@Component
public class WeChatReadParser implements DataSourceParser { ... }
@Component
public class BilibiliParser implements DataSourceParser { ... }

// 路由分发
@Component
public class ParserRouter {
    private final List<DataSourceParser> parsers;
    
    public DataSourceParser route(String url) {
        return parsers.stream()
            .filter(p -> p.supports(url))
            .findFirst()
            .orElse(defaultParser);
    }
}
```

**MVP 阶段支持的数据源**：
1. URL 导入（网页文章智能提取正文）
2. 文本/Markdown 手动粘贴
3. 文件上传（PDF、TXT、MD）

**后续扩展**：微信读书、B站、飞书、公众号（通过浏览器插件采集）

---

### 4.2 蒸馏引擎

#### 异步处理管线（MQ 驱动）

```
用户导入内容
    │
    ▼
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│ 内容预处理    │ ──→ │ LLM 蒸馏      │ ──→ │ 后处理 & 存储  │
│              │     │              │     │              │
│ · 正文提取   │     │ · 核心论点    │     │ · 存 MySQL    │
│ · 分段分块   │     │ · 关键概念    │     │ · 向量化存    │
│ · 去噪清洗   │     │ · 结构化摘要  │     │   Milvus     │
│              │     │ · 标签生成    │     │ · 触发关联    │
└──────────────┘     └──────────────┘     │   分析事件    │
                         │                └──────────────┘
                    失败 ↓
               ┌──────────────┐
               │ 死信队列      │
               │ → 告警 + 重试 │
               └──────────────┘
```

#### 知识卡片数据结构

```json
{
  "id": "card_001",
  "title": "Transformer 的自注意力机制",
  "source": {
    "type": "WEB_ARTICLE",
    "url": "https://example.com/transformer",
    "title": "Attention Is All You Need 论文精读",
    "importedAt": "2026-03-08T10:00:00Z"
  },
  "distilled": {
    "corePoints": [
      "自注意力机制替代了传统 RNN 的序列依赖",
      "多头注意力允许模型关注不同位置的信息",
      "位置编码解决了并行化后的顺序信息丢失"
    ],
    "keyTerms": ["Self-Attention", "Multi-Head", "Positional Encoding"],
    "summary": "本文介绍了 Transformer 架构的核心创新...",
    "contentType": "METHODOLOGY",
    "qualityScore": 85
  },
  "tags": ["深度学习", "NLP", "注意力机制"],
  "maturity": {
    "level": "UNDERSTANDING",
    "reviewCount": 2,
    "nextReviewAt": "2026-03-15T08:00:00Z"
  },
  "relations": [
    { "targetCardId": "card_042", "type": "EXTENDS", "score": 0.92 },
    { "targetCardId": "card_078", "type": "CONTRADICTS", "score": 0.75 }
  ]
}
```

---

### 4.3 AI Agent 引擎（核心模块）

#### Agent 抽象接口

```java
public interface KnowledgeAgent {
    
    // Agent 名称
    String getName();
    
    // 触发条件：什么时候该运行这个 Agent
    boolean shouldTrigger(AgentContext context);
    
    // 执行逻辑：感知 → 思考 → 行动
    AgentResult execute(AgentContext context);
    
    // 优先级（多个 Agent 同时触发时的执行顺序）
    int priority();
}
```

#### 各 Agent 职责

| Agent | 触发条件 | 思考过程 | 行动 |
|-------|---------|---------|------|
| **ValueAssessmentAgent** | 新内容进入系统时 | LLM 评估 5 维重要性分数 | 决定存储/丢弃/待确认 |
| **RelationDiscoveryAgent** | 新卡片创建后 / 定时巡检 | 向量相似度 + LLM 推理关联 | 建立知识关联，生成桥接洞察 |
| **LearningStateAgent** | 定时（每日） | 遗忘曲线计算 + 行为分析 | 生成复习提醒，调整成熟度 |
| **ConflictDetectionAgent** | 新卡片创建后 | 对比已有知识，LLM 判断矛盾 | 推送矛盾提示，引导思考 |
| **PushDecisionAgent** | 有待推送内容时 | 分析用户状态、历史偏好、当前时间 | 决定推送时机和方式 |
| **SelfReflectionAgent** | 定时（每周） | 统计用户反馈数据 | 调整各 Agent 参数 |

#### Agent 调度器

```java
@Component
public class AgentScheduler {
    
    private final List<KnowledgeAgent> agents;
    private final ThreadPoolExecutor executor;
    
    // 事件触发：新内容进来时
    @EventListener
    public void onNewContent(NewContentEvent event) {
        AgentContext context = buildContext(event);
        agents.stream()
            .filter(a -> a.shouldTrigger(context))
            .sorted(Comparator.comparingInt(KnowledgeAgent::priority))
            .forEach(a -> executor.submit(() -> a.execute(context)));
    }
    
    // 定时巡检：Agent 的周期性思考
    @Scheduled(cron = "0 0 8 * * ?") // 每天早上 8 点
    public void dailyThinkingLoop() {
        // 遗忘曲线检查、知识缺口分析、每日简报生成...
    }
}
```

---

### 4.4 知识关联引擎

#### 关联发现流程

```
新卡片创建
    │
    ▼
┌──────────────────┐
│ 1. 向量相似度检索  │  从 Milvus 中找出 Top-20 相似卡片
└────────┬─────────┘
         ▼
┌──────────────────┐
│ 2. LLM 关联推理   │  让 LLM 判断具体关联类型和强度
│                  │
│  类型：           │
│  · EXTENDS 扩展   │  B 是 A 的深入/细化
│  · SUPPORTS 支持  │  B 为 A 提供了证据
│  · CONTRADICTS    │  B 与 A 观点矛盾
│  · APPLIES 应用   │  B 是 A 的实践案例
│  · BRIDGES 桥接   │  B 连接了 A 和 C
└────────┬─────────┘
         ▼
┌──────────────────┐
│ 3. 关联存储       │  存入关联表 + 判断是否值得推送
└──────────────────┘
```

---

### 4.5 RAG 问答引擎

#### 混合检索策略

```
用户提问："Transformer 和 LSTM 有什么区别？"
    │
    ├── 路径 1：关键词检索 (MySQL 全文索引)
    │   → 匹配 "Transformer", "LSTM" 关键词的卡片
    │
    ├── 路径 2：向量语义检索 (Milvus)
    │   → 问题向量化 → ANN 检索相似内容
    │
    └── 路径 3：知识图谱遍历（可选）
        → 从 "Transformer" 节点出发，遍历关联卡片
    
    合并 → 去重 → RRF 融合排序 → Top-K
    
    → 将 Top-K 卡片作为上下文，LLM 生成回答
    → 回答中标注引用来源（哪张卡片、哪个数据源）
```

---

### 4.6 Go AI 网关

#### 核心功能模块

```
┌──────────────────────────────────────────────┐
│               Go AI Gateway                   │
│                                              │
│  接收 Java 后端的 gRPC 请求                    │
│                  │                            │
│         ┌────────▼─────────┐                  │
│         │   语义缓存检查    │                  │
│         │ 相似 Prompt 命中  │                  │
│         │ → 直接返回       │                  │
│         └───┬──────────┬───┘                  │
│          命中│          │未命中                │
│             │    ┌─────▼──────┐               │
│             │    │ 智能路由    │               │
│             │    │            │               │
│             │    │ 蒸馏任务→DeepSeek            │
│             │    │ 推理任务→Claude              │
│             │    │ 通用任务→GPT-4o-mini         │
│             │    └─────┬──────┘               │
│             │          │                      │
│             │    ┌─────▼──────┐               │
│             │    │ 限流 & 熔断 │               │
│             │    │ 令牌桶      │               │
│             │    │ + 断路器    │               │
│             │    └─────┬──────┘               │
│             │          │                      │
│             │    ┌─────▼──────┐               │
│             │    │ 代理转发    │               │
│             │    │ SSE 流式   │               │
│             │    │ 透传       │               │
│             │    └─────┬──────┘               │
│             │          │                      │
│             │    ┌─────▼──────┐               │
│             │    │ Token 计量  │               │
│             │    │ 成本统计    │               │
│             │    └────────────┘               │
│             │          │                      │
│             └────┬─────┘                      │
│                  ▼                            │
│           返回结果给 Java 后端                  │
└──────────────────────────────────────────────┘
```

#### 语义缓存（面试亮点）

```go
type SemanticCache struct {
    vectorStore VectorStore  // 向量索引
    resultStore KVStore      // 结果缓存
    threshold   float64      // 相似度阈值 (0.95)
    ttl         time.Duration
}

// 相似问题命中缓存，节省 30%+ LLM 调用成本
func (c *SemanticCache) Get(prompt string) (string, bool) {
    embedding := c.embed(prompt)
    matches := c.vectorStore.Search(embedding, c.threshold)
    if len(matches) > 0 {
        return c.resultStore.Get(matches[0].ID)
    }
    return "", false
}
```

---

### 4.7 主动推送引擎

#### 推送类型

| 推送类型 | 触发方式 | 内容 |
|---------|---------|------|
| **实时洞察** | 新卡片创建时 | "这和你之前看的 XX 高度相关" |
| **知识简报** | 每日定时 | 昨日摘入统计 + AI 洞察 + 复习建议 |
| **复习提醒** | 遗忘曲线触发 | 即将遗忘的卡片 + 快速测验 |
| **矛盾提醒** | Agent 发现时 | 知识库中的矛盾观点 |
| **缺口分析** | 每周定时 | 知识体系中的薄弱环节 + 推荐阅读 |

#### 推送时机决策（不骚扰用户）

```java
// 不是无脑推，而是 AI 判断合适时机
public boolean shouldPushNow(User user, PushContent content) {
    // 1. 用户设置的免打扰时段
    if (user.isInQuietHours()) return false;
    
    // 2. 今日已推送数量（避免过度骚扰）
    if (getTodayPushCount(user) >= user.getDailyPushLimit()) return false;
    
    // 3. 上次推送间隔（至少 2 小时）
    if (minutesSinceLastPush(user) < 120) return false;
    
    // 4. 内容紧急度
    if (content.getUrgency() == URGENT) return true; // 紧急内容立即推
    
    // 5. 用户当前可能空闲（基于历史行为模式）
    return user.isLikelyFree(LocalTime.now());
}
```

---

## 五、数据库设计

### 5.1 MySQL 核心表

```sql
-- 用户表
CREATE TABLE user (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    username        VARCHAR(64) NOT NULL,
    email           VARCHAR(128),
    preferences     JSON COMMENT '用户偏好设置',
    interest_tags   JSON COMMENT '兴趣标签',
    push_config     JSON COMMENT '推送配置',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

-- 知识卡片表
CREATE TABLE knowledge_card (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT NOT NULL,
    title           VARCHAR(256) NOT NULL,
    summary         TEXT COMMENT '蒸馏后的结构化摘要',
    core_points     JSON COMMENT '核心论点列表',
    key_terms       JSON COMMENT '关键术语',
    content_type    VARCHAR(32) COMMENT 'METHODOLOGY/CASE/OPINION/NEWS',
    quality_score   INT COMMENT '内容质量评分 0-100',
    importance_score INT COMMENT '对用户的重要性评分 0-100',
    tags            JSON COMMENT '标签列表',
    maturity_level  VARCHAR(32) DEFAULT 'NEW' COMMENT 'NEW/UNDERSTANDING/FAMILIAR/MASTERED',
    review_count    INT DEFAULT 0,
    next_review_at  DATETIME COMMENT '下次复习时间（遗忘曲线）',
    status          VARCHAR(16) DEFAULT 'ACTIVE' COMMENT 'ACTIVE/PENDING/ARCHIVED',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_user_id (user_id),
    INDEX idx_user_status (user_id, status),
    INDEX idx_next_review (user_id, next_review_at)
);

-- 数据来源表
CREATE TABLE knowledge_source (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    card_id         BIGINT NOT NULL,
    source_type     VARCHAR(32) NOT NULL COMMENT 'WEB/WECHAT_READ/BILIBILI/MANUAL/FILE',
    source_url      VARCHAR(1024),
    source_title    VARCHAR(256),
    raw_content     MEDIUMTEXT COMMENT '原始内容',
    imported_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_card_id (card_id)
);

-- 知识关联表
CREATE TABLE knowledge_relation (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT NOT NULL,
    source_card_id  BIGINT NOT NULL,
    target_card_id  BIGINT NOT NULL,
    relation_type   VARCHAR(32) COMMENT 'EXTENDS/SUPPORTS/CONTRADICTS/APPLIES/BRIDGES',
    similarity_score DECIMAL(4,3) COMMENT '相似度分数',
    ai_explanation  VARCHAR(512) COMMENT 'AI 对这个关联的解释',
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_source_card (source_card_id),
    INDEX idx_target_card (target_card_id)
);

-- 用户行为日志（用于反馈闭环）
CREATE TABLE user_behavior_log (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT NOT NULL,
    behavior_type   VARCHAR(32) COMMENT 'VIEW/SEARCH/REVIEW/DISMISS/SHARE',
    target_type     VARCHAR(32) COMMENT 'CARD/PUSH/QUIZ',
    target_id       BIGINT,
    extra_data      JSON,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_time (user_id, created_at)
);

-- 推送记录表
CREATE TABLE push_record (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT NOT NULL,
    push_type       VARCHAR(32) COMMENT 'INSIGHT/DAILY_DIGEST/REVIEW/CONFLICT/GAP',
    content         JSON COMMENT '推送内容',
    is_read         BOOLEAN DEFAULT FALSE,
    is_acted        BOOLEAN DEFAULT FALSE COMMENT '用户是否采取了行动',
    pushed_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_time (user_id, pushed_at)
);

-- AI 网关调用计量表
CREATE TABLE llm_usage_log (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id         BIGINT,
    model           VARCHAR(64),
    task_type       VARCHAR(32) COMMENT 'DISTILL/QA/RELATE/ASSESS',
    input_tokens    INT,
    output_tokens   INT,
    cost_cents      INT COMMENT '费用（分）',
    latency_ms      INT,
    cache_hit       BOOLEAN DEFAULT FALSE,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_user_time (user_id, created_at)
);
```

### 5.2 Milvus 向量集合

```
集合名：knowledge_vectors
字段：
  - id (INT64, 主键)
  - card_id (INT64)
  - user_id (INT64)  
  - chunk_index (INT32, 卡片内的分块序号)
  - chunk_text (VARCHAR, 原始文本块)
  - embedding (FLOAT_VECTOR, dim=1536)
索引：IVF_FLAT, nlist=1024
```

---

## 六、API 设计（核心接口）

### 6.1 Java 后端 REST API

```
知识管理
  POST   /api/v1/knowledge/import         导入内容（触发蒸馏管线）
  GET    /api/v1/knowledge/cards           获取知识卡片列表
  GET    /api/v1/knowledge/cards/{id}      获取卡片详情（含关联）
  PUT    /api/v1/knowledge/cards/{id}      更新卡片
  DELETE /api/v1/knowledge/cards/{id}      删除卡片
  GET    /api/v1/knowledge/cards/{id}/relations  获取卡片关联
  POST   /api/v1/knowledge/search         搜索知识库

问答
  POST   /api/v1/qa/ask                   提问（RAG 问答，支持 SSE 流式）

主动推送
  GET    /api/v1/push/unread              获取未读推送
  POST   /api/v1/push/{id}/read           标记已读
  POST   /api/v1/push/{id}/act            标记已采取行动

用户
  GET    /api/v1/user/profile             获取用户画像
  PUT    /api/v1/user/preferences         更新偏好设置
  GET    /api/v1/user/stats               获取学习统计

仪表盘
  GET    /api/v1/dashboard/daily-digest   获取每日简报
  GET    /api/v1/dashboard/knowledge-map  获取知识图谱数据
  GET    /api/v1/dashboard/review-queue   获取待复习队列
```

### 6.2 Go AI Gateway gRPC 接口

```protobuf
service AIGateway {
  // 普通请求
  rpc Complete(CompletionRequest) returns (CompletionResponse);
  // 流式请求
  rpc StreamComplete(CompletionRequest) returns (stream CompletionChunk);
  // 向量化
  rpc Embed(EmbedRequest) returns (EmbedResponse);
}

message CompletionRequest {
  string task_type = 1;  // DISTILL, QA, RELATE, ASSESS
  string model_preference = 2;  // 可选，指定模型
  repeated Message messages = 3;
  int64 user_id = 4;
  CompletionConfig config = 5;
}
```

---

## 七、分阶段实施计划

### Phase 1：核心骨架（第 1-2 周）

> 目标：跑通 "导入内容 → 蒸馏 → 存储 → 问答" 的完整链路

- [ ] Spring Boot 项目初始化（分层架构）
- [ ] MySQL 数据库建表 & MyBatis-Plus 集成
- [ ] 内容导入接口（URL + 手动文本）
- [ ] 蒸馏管线（RabbitMQ 异步 + LLM 调用）
- [ ] 知识卡片 CRUD API
- [ ] Milvus 集成 & 向量化存储
- [ ] 基础 RAG 问答（向量检索 + LLM 生成）

### Phase 2：AI 网关 + Agent（第 3-4 周）

> 目标：Go 网关上线 + AI 开始"主动思考"

- [ ] Go AI Gateway 搭建（gRPC + HTTP 代理）
- [ ] 智能路由（按任务类型选模型）
- [ ] 限流熔断（令牌桶 + 断路器）
- [ ] 语义缓存
- [ ] Token 计量 & 成本统计
- [ ] Java 侧 Agent 引擎框架
- [ ] 价值评估 Agent
- [ ] 关联发现 Agent

### Phase 3：主动式能力（第 5-6 周）

> 目标：AI 真正"主动起来"

- [ ] 学习状态 Agent（遗忘曲线）
- [ ] 认知冲突 Agent
- [ ] 推送决策 Agent
- [ ] 自我反思 Agent（反馈闭环）
- [ ] 每日知识简报生成
- [ ] 混合检索（关键词 + 向量 + RRF 融合）

### Phase 4：前端 & 完善（第 7-8 周）

> 目标：可演示的完整产品

- [ ] Vue 3 前端：知识看板
- [ ] Vue 3 前端：问答对话界面
- [ ] Vue 3 前端：知识图谱可视化
- [ ] 浏览器插件（Chrome Extension）
- [ ] 性能优化 & 压测数据准备

---

## 八、验证方案

### 自动化测试

```bash
# Java 后端单元测试
mvn test

# Java 后端集成测试（需要 MySQL + Redis + RabbitMQ）
mvn verify -Pintegration-test

# Go 网关单元测试
cd ai-gateway && go test ./...

# Go 网关集成测试
cd ai-gateway && go test -tags=integration ./...
```

### 功能验证

1. **蒸馏链路验证**：导入一篇文章 URL → 查看消息队列消费 → 验证知识卡片生成质量
2. **RAG 问答验证**：导入 5+ 篇文章 → 提问跨文章问题 → 验证检索和回答质量
3. **Agent 验证**：导入新内容 → 观察 Agent 是否自动触发关联发现和价值评估
4. **网关验证**：模拟高并发请求 → 验证限流、熔断、降级是否正常工作
5. **语义缓存验证**：发送语义相似的问题 → 验证缓存命中

### 性能指标（面试准备）

| 指标 | 目标值 |
|------|--------|
| 知识导入延迟（异步确认） | < 200ms |
| RAG 问答延迟（含 LLM） | < 3s |
| 向量检索延迟 | < 50ms |
| 网关代理延迟增加 | < 10ms |
| 语义缓存命中率 | > 25% |

---

## 九、项目结构

```
stellar-ionosphere/
├── stellar-server/              # Java Spring Boot 主服务
│   ├── src/main/java/
│   │   └── com/stellar/
│   │       ├── common/          # 公共模块（异常、工具类、常量）
│   │       ├── config/          # 配置类
│   │       ├── controller/      # REST API 控制器
│   │       ├── service/         # 业务逻辑层
│   │       │   ├── knowledge/   # 知识管理服务
│   │       │   ├── distill/     # 蒸馏引擎
│   │       │   ├── relation/    # 关联引擎
│   │       │   ├── qa/          # RAG 问答引擎
│   │       │   └── push/        # 推送引擎
│   │       ├── agent/           # AI Agent 引擎
│   │       │   ├── core/        # Agent 抽象、调度器
│   │       │   └── agents/      # 各个具体 Agent 实现
│   │       ├── datasource/      # 数据源接入（解析器）
│   │       ├── model/           # 数据模型（Entity/DTO/VO）
│   │       ├── mapper/          # MyBatis Mapper
│   │       └── mq/              # 消息队列消费者
│   ├── src/main/resources/
│   │   ├── application.yml
│   │   └── mapper/              # MyBatis XML
│   └── pom.xml
│
├── stellar-gateway/             # Go AI 网关
│   ├── cmd/
│   │   └── gateway/
│   │       └── main.go
│   ├── internal/
│   │   ├── router/              # 智能路由
│   │   ├── ratelimit/           # 限流熔断
│   │   ├── cache/               # 语义缓存
│   │   ├── metrics/             # Token 计量
│   │   ├── proxy/               # LLM API 代理
│   │   └── config/              # 配置
│   ├── api/
│   │   └── proto/               # gRPC Protobuf 定义
│   ├── go.mod
│   └── go.sum
│
├── stellar-web/                 # Vue 3 前端（Phase 4）
│
├── docs/                        # 文档
│   └── architecture.md
│
└── docker-compose.yml           # 本地开发环境（MySQL/Redis/RabbitMQ/Milvus）
```

---

## 十、面试叙事线

```
1. 【背景】"信息过载是所有知识工作者的痛点"
2. 【创新点】"我做了一个主动式 AI——它有自己的思考循环，会自主发现关联、检测矛盾、预测需求"
3. 【架构】"Java 做业务 + Go 做 AI 网关，用 gRPC 通信"
4. 【深度】（按面试官追问方向展开 2-3 个模块的技术细节）
5. 【成果】"语义缓存节省 30% LLM 成本，问答延迟 < 3s"
```
