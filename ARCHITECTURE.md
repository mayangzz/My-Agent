# My-Agent · 项目总览与架构

> 这份文档是**通读整个项目的入口**:目标、架构、目录、数据流、配置体系、各模块职责、路线图、变更记录。
> **约定:每次改动都同步更新本文档**(尤其底部「变更记录」),让它始终等于项目的真实状态。

## 目标

做一个**好用、易上手、可演进到企业级**的 agent,并**开源**。

- **教学友好**:核心(harness 循环、工具、上下文)用最少代码摊开,不被框架糊住——看得懂"裸模型 API 怎么变成会自己干活的 agent"。
- **工程可用**:配置/密钥分离、记忆后端可插拔、本地后台可视化管理,逐步往企业级补齐(安全、可观测、并发、流式…见路线图)。
- **可贡献**:模块边界清晰、接口先行,新功能多是"加一个实现",而非改核心。

## 架构总览

分层,自下而上:

```
模型底座 (DeepSeek, OpenAI 兼容 HTTP)
        ▲  文本进 / 文本出
┌───────┴─────────────────────────────────────────┐
│ harness  ——  agent 的"灵魂":调模型→执行工具→塞回→再调  │
│   · client   裸 net/http 调 DeepSeek               │
│   · tools    工具注册表 + 内置工具                    │
│   · agent    那个循环(带 session 记忆)              │
│   · memory   Memory 接口(存取会话历史)              │
└───────┬───────────────────────┬─────────────────┘
        │ 注入实现                │ 读设置
   ┌────┴─────┐            ┌─────┴──────┐
   │ memstore │            │ settings   │
   │ inmem    │            │ settings.  │◀── admin(本地后台网页改它)
   │ postgres │            │ json       │
   │ redis    │            └────────────┘
   └──────────┘
        ▲ 连接串/密钥
   config.local.json (gitignored)
```

要点:
- **harness 只认接口**(`Memory`),不关心记忆存哪;具体实现由 `memstore` 提供、启动时注入。
- **设置(可改的数据)** 和 **密钥(secret)** 物理分离:设置在 `settings.json`(后台可改),密钥在 `config.local.json`(永不进 git、永不进后台)。

## 目录结构

```
My-Agent/
├── main.go                # 入口:加载密钥+设置、组装、子命令(repl / admin)、REPL
├── harness/               # agent 核心(教学重点,裸 net/http,不套 SDK)
│   ├── types.go           #   消息/工具调用结构(OpenAI 兼容)
│   ├── client.go          #   DeepSeek 客户端
│   ├── tools.go           #   工具注册表 + 内置工具(now / read_file)
│   ├── memory.go          #   Memory 接口
│   └── agent.go           #   那个循环(带 session 记忆)
├── memstore/              # Memory 的几种实现(启动可选)
│   ├── memstore.go        #   Config + New() 工厂
│   ├── inmem.go           #   内存(重启即丢)
│   ├── postgres.go        #   Postgres(默认,落盘持久)
│   └── redis.go           #   Redis(对话维度,带 TTL)
├── settings/              # 可调设置的读写(settings.json)
│   └── settings.go
├── admin/                 # 本地后台:网页 + 设置读写接口
│   ├── admin.go
│   └── index.html         #   go:embed 进二进制
├── config.example.json    # 密钥模板(committed)
├── config.local.json      # 真实密钥(gitignored,绝不提交)
├── settings.json          # 运行时设置(gitignored,缺则按默认生成)
├── README.md              # 快速上手
└── ARCHITECTURE.md        # 本文(项目总览)
```

## 数据流:一次对话怎么走

1. `main` 读 `config.local.json`(密钥/连接串)+ `settings.json`(系统提示词/模型/max_steps/记忆后端)。
2. 按 `memory.backend` 用 `memstore.New` 造一个 `Memory` 实现,注入 `Agent`。
3. REPL 收到一句输入,调 `Agent.Run(ctx, session, input)`:
   - 从 `Memory` 取回该 session 历史 → 拼上 system + 历史 + 这次输入;
   - 进循环:调模型 → 若要工具则逐个执行、结果塞回 → 再调,直到模型给最终答案或撞 `MaxSteps`;
   - 每条新消息(user/assistant/tool)都 `Append` 进 `Memory` → **跨轮(持久后端下跨重启)自动有记忆**。

## 配置体系(三层,职责不重叠)

| 放哪 | 内容 | 谁改 | 进 git? |
|---|---|---|---|
| `config.local.json` | 密钥、连接串(DeepSeek key、PG DSN、Redis 地址密码) | 人手动 | ❌ gitignored |
| `settings.json` | 系统提示词、模型、max_steps、记忆后端 | 后台网页 / 手改 | ❌ gitignored(缺则按默认生成) |
| 代码默认值 | 上面两者的兜底默认 | 改代码 | ✅ |

> 铁律:**密钥只在 `config.local.json`,绝不进 git、绝不进后台网页**(后台是本地 http 服务,放密钥会被读到)。

## 记忆后端(启动可选)

`settings.json` 里 `memory.backend` 选,或启动 `--memory xxx` 覆盖:

| backend | 适合 | 特点 |
|---|---|---|
| `postgres` | **默认**,持久/结构化记忆 | 落盘、可查询、事务;表 `agent_messages(session, seq, data jsonb)` |
| `redis` | 对话/session 维度 | 一 session 一个 List,带 24h TTL 自动过期 |
| `inmem` | 开发/试跑 | 纯内存,**重启即丢**(启动会打印提示) |

未来要"按意思召回"的语义记忆(RAG),加一个 pgvector / Qdrant 实现即可,接口不变。

## 运行

```bash
cp config.example.json config.local.json   # 填 deepseek_api_key
go run .                 # REPL(默认),读 settings.json
go run . -memory inmem   # 临时换记忆后端
go run . admin           # 本地后台 http://127.0.0.1:7788,网页改设置
```
REPL 里 `/reset` 清空当前 session 记忆。

## 路线图(待完善,欢迎认领)

- [ ] **安全闸**:工具沙箱 / 路径白名单 / 危险操作确认(当前 `read_file` 可读任意路径)。
- [ ] **健壮性**:HTTP 重试 + 限流退避;工具输出按 rune 安全截断。
- [ ] **流式输出**。
- [ ] **上下文压缩**:长对话不撑爆窗口。
- [ ] **可观测**:token usage / 成本 / 每步 trace。
- [ ] **多会话**:REPL/后台支持多 session 切换与查看。
- [ ] **MCP 接入**:实现 MCP client,自动发现并注册外部工具。
- [ ] **语义记忆**:pgvector / Qdrant 实现。
- [ ] **后台增强**:试运行面板、运行历史、工具开关。

## 变更记录

- **2026-06-23** —— 从"最小单文件 harness"演进为可配置工程版:
  - 抽出 `settings`(系统提示词/模型/max_steps/记忆后端 → `settings.json`),`main` 瘦身。
  - 新增 `harness.Memory` 接口 + `memstore`(inmem / postgres / redis 三实现,启动可选,默认 postgres);`Agent.Run` 改为带 `session` 的记忆循环,**跨轮记忆**落地。
  - 新增 `admin` 本地后台(网页改设置,go:embed)。
  - 配置体系重整:**密钥从 env 迁到 `config.local.json`(gitignored)**,设置进 `settings.json`,职责分离。
  - 依赖:`jackc/pgx/v5`、`redis/go-redis/v9`。
- **2026-06-22** —— 初版:最小 agent harness(Go + DeepSeek,裸 net/http,now/read_file 两个工具),开源到 GitHub。
