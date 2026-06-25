# My-Agent · 项目总览与架构

> 这份文档是**通读整个项目的入口**:目标、架构、目录、数据流、配置体系、各模块职责、路线图、变更记录。
> **约定:每次改动都同步更新本文档**(尤其底部「变更记录」),让它始终等于项目的真实状态。

## 目标

做一个**好用、易上手、可演进到企业级**的 agent,并**开源**。

- **教学友好**:核心(harness 循环、工具、上下文)用最少代码摊开,不被框架糊住——看得懂"裸模型 API 怎么变成会自己干活的 agent"。
- **工程可用**:配置/密钥分离、记忆后端可插拔、本地后台可视化管理,逐步往企业级补齐(安全、可观测、并发、流式…见路线图)。
- **可贡献**:模块边界清晰、接口先行,新功能多是"加一个实现",而非改核心。

## 第一设计原则:一切能力皆可插拔、皆可配置

**同一种能力抽成一个接口,给多个实现,启动时用配置里的一个开关选用哪个——绝不写死一种再加兜底。**

这条贯穿全项目:记忆是 `Memory` 接口 + 五个后端 +「`memory.backend` 开关」;执行是 `Runner` 接口 + local/docker +「`runner.mode` 开关」;分层记忆是「`daily_summary` 开关」;将来的模型、检索、工具集都照此办理。于是:

- **加能力 = 加一个实现 + 一个选项**,核心代码不动,回归风险低。
- **每个开关都有合理默认 + 用户可改**(settings.json / 后台 / 启动 flag),开箱即用又不锁死。
- 接口先行、实现在各自的包里(`memstore` / `runner` / …),边界清晰、好测试、好贡献。

## 架构总览

分层,自下而上:

```
模型底座 (DeepSeek, OpenAI 兼容 HTTP)
        ▲  文本进 / 文本出
┌───────┴─────────────────────────────────────────────────┐
│ harness  ——  agent 的"灵魂":调模型→执行工具→塞回→再调          │
│   · client   裸 net/http 调 DeepSeek                       │
│   · tools    工具注册表 + 内置工具(now/read_file/spawn)      │
│   · agent    那个循环(带 session 记忆 + 权限闸)             │
│   · memory   Memory 接口(存取会话历史)         ← 选实现       │
│   · runner   Runner 接口(子 agent 在哪儿跑)    ← 选实现       │
└──┬──────────────────┬───────────────────────┬───────────┘
   │ 注入实现           │ 注入实现               │ 读设置
┌──┴───────┐     ┌─────┴──────┐         ┌──────┴─────┐
│ memstore │     │ runner     │         │ settings   │
│ inmem    │     │ local      │         │ settings.  │◀─ admin(网页改)
│ postgres │     │ docker     │         │ json       │
│ redis    │     └────────────┘         └────────────┘
└──────────┘            ▲ docker 地址/镜像
        ▲ 连接串/密钥     │
   config.local.json (gitignored) ─┘
```

要点(贯穿全项目的一个模式):
- **harness 只认接口**(`Memory`、`Runner`、`Tool`),不关心具体怎么实现;实现放在各自的包里(`memstore` / `runner`),**启动时按配置注入**。
- 这就是项目的可扩展骨架:**加能力 = 加一个接口实现,而非改核心**。换记忆后端、换执行后端、加工具、将来接 MCP / 语义记忆,都套这个模式。
- **设置(可改的数据)** 和 **密钥(secret)** 物理分离:设置在 `settings.json`(后台可改),密钥/基础设施地址在 `config.local.json`(永不进 git、永不进后台)。

## 目录结构

```
My-Agent/
├── main.go                # 入口:加载密钥+设置、组装、子命令(repl / admin / subagent)、REPL
├── harness/               # agent 核心(教学重点,裸 net/http,不套 SDK)
│   ├── types.go           #   消息/工具调用结构(OpenAI 兼容)
│   ├── client.go          #   DeepSeek 客户端
│   ├── tools.go           #   工具注册表 + 内置工具(now / read_file)
│   ├── memory.go          #   Memory 接口
│   ├── runner.go          #   Runner 接口 + spawn_subagent 工具
│   ├── perms.go           #   权限策略(allow/ask/deny)
│   └── agent.go           #   那个循环(带 session 记忆 + 权限闸)
├── memstore/              # Memory 的几种实现(启动时选)
│   ├── memstore.go        #   Config + New() 工厂
│   ├── layered.go         #   分层记忆装饰器(按天归档 + 每日摘要 + 保留期),包住任意后端
│   ├── filewiki.go        #   本地文件 .jsonl(默认,无需 DB,落盘持久)
│   ├── postgres.go        #   Postgres(落盘持久)
│   ├── redis.go           #   Redis(对话维度,带 TTL)
│   ├── inmem.go           #   内存(重启即丢)
│   └── none.go            #   不做记忆
├── runner/                # Runner 的几种实现(子 agent 在哪儿跑,启动可选)
│   ├── local.go           #   本进程内(默认,无需 Docker)
│   └── docker.go          #   一次性容器隔离(需 docker 地址/镜像)
├── settings/              # 可调设置的读写(settings.json)
│   └── settings.go
├── admin/                 # 本地后台:网页 + 设置读写接口
│   ├── admin.go
│   └── index.html         #   go:embed 进二进制
├── Dockerfile             # 子 agent 容器镜像(runner=docker 时用)
├── config.example.json    # 密钥模板(committed)
├── config.local.json      # 真实密钥/连接串/docker 地址(gitignored,绝不提交)
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
| `config.local.json` | 密钥、连接串、基础设施地址(DeepSeek key、PG DSN、Redis 地址密码、docker 地址/镜像) | 人手动 | ❌ gitignored |
| `settings.json` | 系统提示词、模型、max_steps、记忆后端、执行后端(runner.mode)、权限 | 后台网页 / 手改 | ❌ gitignored(缺则按默认生成) |
| 代码默认值 | 上面两者的兜底默认 | 改代码 | ✅ |

> 铁律:**密钥只在 `config.local.json`,绝不进 git、绝不进后台网页**(后台是本地 http 服务,放密钥会被读到)。

## 记忆后端(启动可选)

记忆后端是 `Memory` 接口的一个实现,**启动时选**——不替你拍一个再兜底:首次启动(无 `settings.json`)会**问一嘴**让你选,选完存进 `settings.json`;之后直接用,`--memory xxx` 仍可临时覆盖,后台网页也能改。

| backend | 适合 | 特点 |
|---|---|---|
| `filewiki` | **默认**,无依赖的持久记忆 | 每 session 一个本地 `.jsonl`(逐条消息一行),无需 DB/容器,落盘持久、可直接打开看,JSONL 忠实保留工具调用 |
| `postgres` | 结构化/可查询的持久记忆 | 落盘、可查询、事务;表 `agent_messages(session, seq, data jsonb)`;需在 config.local.json 配 DSN |
| `redis` | 对话/session 维度 | 一 session 一个 List,带 24h TTL 自动过期;需配地址 |
| `inmem` | 开发/试跑 | 纯内存,**重启即丢**(启动会打印提示) |
| `none` | 一次性问答 | 不做记忆,每轮都是干净的、无跨轮上下文 |

### 分层记忆(短期 + 长期,架在后端之上)

光把原始消息堆着会越攒越大、也不分轻重。`Layered`(`memstore/layered.go`)是一层**装饰器**:它本身实现 `Memory`、内部包一个上面的后端,把记忆分成短期和长期——**两者共用同一套抽象接口,跟具体后端无关**。`daily_summary` 开关控制(filewiki/postgres/redis 下默认开)。

底层 key 约定(后端只当普通 session 名,filewiki 落成按天的目录):

```
<base>/<YYYY-MM-DD>/raw      当天原始消息(短期/工作记忆)
<base>/<YYYY-MM-DD>/digest   当天摘要(长期记忆,单条 ≤summary_max_chars)
<base>/_index               出现过哪些天(保留期清理用)
```

- **每日摘要**:一轮对话的最终答案落地后,**异步**把当天对话重新总结成一份 ≤2000 字摘要覆盖写回;进程退出前再同步 `Flush` 一次,防短会话丢摘要。不用定时任务——"每次对话后更新",一天结束时摘要自然就是完整的。
- **召回(带权重)**:`Load` 时按**新近度**从近往老装每日摘要,装满 `recall_budget` 字符预算为止(老的丢),再按时间正序拼在当天原始对话前面 → agent 记得"前天、昨天"。语义相关度权重以后接 embedding 再加,接口不变。
- **保留期**:默认 **30 天**(`retention_days` 可调),`Load` 时顺手把超期的天连原始带摘要一起清掉、更新 `_index`。
- **`/reset`** 只清当天的工作记忆,不动更早的每日摘要——别误删积累的长期记忆。

> 子 agent 用未分层的 baseMem(临时,不需要长期记忆/每日总结)。配置在 `settings.json` 的 `memory.{daily_summary,retention_days,summary_max_chars,recall_budget}`,后台可改。

## 子 agent 与执行后端(agent team)

主 agent 可以把一个子任务**派给一个角色化的子 agent**(各司其职),自己拿回结果再继续——这就是 "agent team"。靠一个内置工具 `spawn_subagent(role, task)` 触发,子 agent 在哪儿跑由 `Runner` 接口决定,启动时按 `settings.json` 的 `runner.mode` 选实现:

| mode | 子 agent 跑在哪 | 隔离 | 需要 Docker? | 用途 |
|---|---|---|---|---|
| `local` | **默认**,本进程内(goroutine 级) | 共用同一 client / Memory 后端(状态面共享),独立 session + 角色 prompt | 否 | 开箱即用的"共享上下文、各司其职" |
| `docker` | 一次性容器(`docker run --rm`) | blast-radius 隔离:容器内执行 `my-agent subagent`,自带 inmem 记忆、结果从 stdout 带回 | 是(本机 daemon 或远程 `DOCKER_HOST`) | 任务要跑不可信代码/命令时把本地破坏关进容器 |

两条铁律(两平面拆分):
- **计算可隔离,副作用不可**:容器只挡住"本地破坏"(乱写文件/乱跑命令),挡不住对外副作用(调外部 API、发消息)——那些仍走权限闸。
- **状态面是 Memory**:local 模式下子 agent 共用同一 Memory 后端;docker 模式下容器拿不到主进程 Memory,上下文靠 `task` 自包含注入(像 Claude Code 给 subagent 一个完整任务)。

防套娃:子 agent 的工具表**不含** `spawn_subagent`,不会无限往下派。`spawn_subagent` 敏感度为 `exec`,默认要过权限闸。

> 代码:`harness/runner.go`(接口 + 工具)、`runner/local.go`、`runner/docker.go`;容器侧入口是 `main.go` 的 `subagent` 子命令;镜像见 `Dockerfile`。

## 操作权限(安全闸)

工具执行前过一道权限闸,防止 agent(或被注入的提示)乱动危险操作。两层:

1. **按敏感度的策略**:每个工具标 `Sensitivity`(`read` / `write` / `exec`);`settings.json` 的 `permissions` 给每类一个动作 `allow` / `ask` / `deny`(默认 `read:allow, write:ask, exec:ask`,后台可改)。
   - `ask` → 执行前在 REPL 弹确认(`允许执行 X? y/N`);headless/后台无确认者时**默认拒**。
   - 被拒就把 "denied" 当工具结果喂回模型,它能换路子(自适应)。
2. **密钥硬拒**:不论策略如何,`read_file` 一律拒读像密钥/凭证的文件——`config.local.json`、`*.env`、`~/.ssh/*`、`id_rsa`、`*.pem`、含 secret/credential 的路径。**堵死"agent 读出你的 API key"**。

> 代码:`harness/perms.go`(策略)、`harness/agent.go` 的 `execTool`(闸)、`harness/tools.go` 的 `isSecretPath`(密钥硬拒)。

## 运行

```bash
cp config.example.json config.local.json   # 填 deepseek_api_key
go run .                 # REPL(默认),读 settings.json
go run . -memory inmem   # 临时换记忆后端
go run . admin           # 本地后台 http://127.0.0.1:7788,网页改设置

# agent team:在 REPL 里让主 agent 用 spawn_subagent 派子 agent(默认 runner=local,本进程跑)
# 想用容器隔离:settings.json 改 runner.mode=docker,config.local.json 填 docker_image,再:
docker build -t my-agent:latest .
```
REPL 里 `/reset` 清空当前 session 记忆。子命令 `subagent --role .. --task ..` 是单次跑一个子 agent(容器内由 docker runner 调起,也可手动跑)。

## 路线图(待完善,欢迎认领)

- [x] **安全闸 v1**:敏感度策略(allow/ask/deny)+ ask 确认 + 密钥文件硬拒。**待补**:WorkDir 根目录限定(只许在某目录树下读写)、工具级沙箱。
- [x] **agent team v1**:`spawn_subagent` + `Runner` 接口(local 本进程 / docker 容器,settings 切换)。**待补**:docker 子 agent 共享 Memory(挂同一后端)、并行多子 agent、子 agent 结果回流主记忆、按角色预设工具集。
- [ ] **健壮性**:HTTP 重试 + 限流退避;工具输出按 rune 安全截断。
- [ ] **流式输出**。
- [ ] **上下文压缩**:长对话不撑爆窗口。
- [ ] **可观测**:token usage / 成本 / 每步 trace。
- [ ] **多会话**:REPL/后台支持多 session 切换与查看。
- [ ] **MCP 接入**:实现 MCP client,自动发现并注册外部工具。
- [ ] **语义记忆**:pgvector / Qdrant 实现。
- [ ] **后台增强**:试运行面板、运行历史、工具开关。

## 变更记录

- **2026-06-26** —— **分层记忆(短期 + 长期)**:新增 `memstore/layered.go`——一个 `Memory` 装饰器,把对话按天归档(`<base>/<date>/raw`+`digest`,filewiki 落成按天目录),每轮对话后异步把当天总结成 ≤2000 字摘要、退出前 `Flush` 兜底;`Load` 按新近度权重 + `recall_budget` 预算召回近若干天摘要 + 当天原始,默认保留 `retention_days=30` 天、超期自动清理;`/reset` 只清当天不动历史摘要。短期/长期共用同一抽象接口、与后端无关(filewiki/postgres/redis 都能套)。settings 加 `memory.{daily_summary,retention_days,summary_max_chars,recall_budget}`,后台可改。recall/retention/digest 生成均已实测。
- **2026-06-25** —— **记忆后端:首次选择 + 本地文件后端**:记忆后端改为"启动时选"而非默认+兜底——首次启动(无 settings.json)弹菜单让用户选、存盘记住,`-memory` 仍可覆盖。新增两实现:`filewiki`(本地 `.jsonl`,无需 DB/容器、落盘持久,设为新默认)、`none`(不做记忆);连同原 inmem/postgres/redis 共五种。`settings.Memory` 加 `dir`;admin 网页加全部后端选项。filewiki 跨进程持久、首次选择、none 无记忆均已实测。
- **2026-06-25** —— **agent team(子 agent + 执行后端开关)**:新增内置工具 `spawn_subagent(role, task)` 让主 agent 派角色化子 agent 各司其职;抽出 `Runner` 接口(`harness/runner.go`),两实现 `runner/local.go`(本进程,默认,共用 Memory)、`runner/docker.go`(一次性容器隔离);`settings.json` 加 `runner.mode`(local/docker,后台可切),`config.local.json` 加 `docker_host` / `docker_image`;`main.go` 加 `subagent` 子命令(容器内入口,密钥读 env);新增 `Dockerfile`(18MB distroless 镜像)。local 与 docker 两条链路均已实测跑通(主 agent 派子 agent → 拿回结果)。
- **2026-06-24** —— **操作权限(安全闸)**:工具加 `Sensitivity`(read/write/exec)+ 按敏感度的 allow/ask/deny 策略(settings.json `permissions`,后台可改);`ask` 在 REPL 弹确认;`read_file` 硬拒密钥/凭证文件(config.local.json / *.env / ~/.ssh / id_rsa / *.pem…)。三种场景已实测:密钥拒读、ask 拒、ask 准。
- **2026-06-23** —— 修 bug:`read_file` 截断原本按字节切,会把多字节字符(中文)从中间切断、产生非法 UTF-8;改为退到 rune 边界安全截断。
- **2026-06-23** —— 从"最小单文件 harness"演进为可配置工程版:
  - 抽出 `settings`(系统提示词/模型/max_steps/记忆后端 → `settings.json`),`main` 瘦身。
  - 新增 `harness.Memory` 接口 + `memstore`(inmem / postgres / redis 三实现,启动可选,默认 postgres);`Agent.Run` 改为带 `session` 的记忆循环,**跨轮记忆**落地。
  - 新增 `admin` 本地后台(网页改设置,go:embed)。
  - 配置体系重整:**密钥从 env 迁到 `config.local.json`(gitignored)**,设置进 `settings.json`,职责分离。
  - 依赖:`jackc/pgx/v5`、`redis/go-redis/v9`。
- **2026-06-22** —— 初版:最小 agent harness(Go + DeepSeek,裸 net/http,now/read_file 两个工具),开源到 GitHub。
