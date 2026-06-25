# My-Agent

> 🛠️ 从零手写一个 agent harness,**搞懂"裸大模型 API 到底怎么变成一个会自己调工具干活的 agent"**。目标是做成一个**好用、可演进到企业级、可开源**的 agent;现在是基础工程版,会持续完善。欢迎提 [issue](https://github.com/mayangzz/My-Agent/issues) 和 PR。
>
> 📖 想通读整个项目(架构 / 数据流 / 配置体系 / 路线图 / 变更记录)看 **[ARCHITECTURE.md](ARCHITECTURE.md)**。

Go 实现,后端接 DeepSeek。harness 核心**故意只用标准库 `net/http`、不套 SDK**——把循环、工具、上下文摊开看清楚;存储等外围则用成熟库(pgx / go-redis)。

## 这是什么 / 不是什么

- 裸大模型 API 只是 **文本进 → 文本出**,一次调用就结束,它不记事、不会动手、不会循环。
- **harness 就是包在这个裸接口外面、把它变成"能反复干活"的那层脚手架**。Claude Code、Cursor 本质都是 harness,只是做得很成熟。
- 本项目是这层脚手架的**最小可跑版**:几十行循环 + 一个工具注册表 + 一个 HTTP 客户端。骨架同源,工程量和 Claude Code 差着十万八千里,但够你看懂"它为什么这么设计"。

## 解剖:一个 harness 的五个零件

| 零件 | 文件 | 干什么 |
|---|---|---|
| 会话状态 | `harness/types.go` (`Message`) | 一个消息列表(system / user / assistant / tool 结果),每轮往里追加 |
| 模型客户端 | `harness/client.go` | 把消息 + 工具定义 POST 给 DeepSeek,拿回一条回复 |
| 工具注册表 | `harness/tools.go` | 每个工具 = 定义(名字/描述/参数 schema) + 真正执行它的 Go 函数 |
| 循环 | `harness/agent.go` | **harness 的灵魂**:调模型 → 有工具调用就执行、塞回结果 → 再调,直到最终答案;带 session 记忆 + 权限闸 |
| 记忆 | `harness/memory.go` + `memstore/` | `Memory` 接口 + 五种后端(filewiki / postgres / redis / inmem / none)+ 分层记忆装饰器(按天归档 + 每日摘要 + 30 天保留),启动时选 |
| 执行后端 | `harness/runner.go` + `runner/` | `Runner` 接口 + 两种实现(local 本进程 / docker 容器);决定子 agent 在哪儿跑 |
| 设置 / 后台 | `settings/` + `admin/` | 系统提示词、记忆/执行后端、权限抽到 `settings.json`,本地后台网页可改 |
| 装配 + REPL | `main.go` | 读密钥+设置、注册工具、起命令行循环 |

> 全项目就一个可扩展模式:**harness 只认接口(`Memory` / `Runner` / `Tool`),加能力 = 加一个实现、不动核心**。`memstore/` 和 `runner/` 就是这么长出来的。

> 完整目录结构与数据流见 [ARCHITECTURE.md](ARCHITECTURE.md)。

## 灵魂:那个循环(`harness/agent.go`)

```
loop:
  ① 调模型(带上所有工具定义)
  ② 模型没要工具  → 这就是最终答案,返回
  ③ 模型要调工具  → 逐个真执行(harness 的"手")
  ④ 把每个工具结果按 role:"tool" 塞回上下文
  ⑤ 带着新结果再循环,让模型决定下一步
  (兜底:撞到 MaxSteps 就停,防跑飞)
```

模型只会"开口点单"(说要调哪个工具),**真正去执行的永远是 harness**——模型自己碰不到文件、网络。这就是 "tool use" 和 "harness 的手" 的分工。

## 跑起来

```bash
cp config.example.json config.local.json   # 填 deepseek_api_key
go run .                  # 首次启动会问一句"记忆存哪",选完记进 settings.json
go run . -memory inmem    # 临时指定后端(none|inmem|filewiki|postgres|redis),跳过询问
go run . admin            # 本地后台,见下
```

首次启动会让你选记忆后端(默认 **filewiki**,本地文件、无需任何 DB):

```
首次启动:对话记忆存哪?(直接回车 = 默认 filewiki)
  1) filewiki  本地文件,落盘持久,无需 DB/容器(默认,推荐)
  2) inmem     纯内存,重启即丢
  ...
选择 [1-5]:
```

选完就能聊,记忆跨轮、跨重启(filewiki/postgres/redis 下):

```
you> 我叫张三,帮我记住
agent> 好的,张三 ...
you> 我刚才说我叫什么?
agent> 你叫张三。       # ← 跨轮记忆生效
```

跑的时候会看到 `method=Agent.Run step=1 tool=now args={}`——那是循环在工作。REPL 里 `/reset` 清空当前会话记忆。

## 本地管理后台

```bash
go run . admin     # 起在 http://127.0.0.1:7788(仅本机)
```

浏览器打开 **http://127.0.0.1:7788**,一个网页表单,改完点保存就写进 `settings.json`、**下次启动 agent 生效**。能改:

- **系统提示词 / 模型 / 最大步数**——agent 的人设、用哪个模型、防跑飞的步数上限。
- **记忆后端**——filewiki / postgres / redis / inmem / none 之间切换。
- **执行后端(runner)**——子 agent 跑本进程(local)还是容器(docker)。
- **工具操作权限**——read / write / exec 各自 allow / ask / deny。

> 后台**只碰设置、绝不碰密钥**;密钥/连接串只在 `config.local.json`,后台读不到。

## agent team:派子 agent 各司其职

主 agent 可以用内置工具 `spawn_subagent(role, task)` 把子任务**派给一个角色化的子 agent**,自己拿回结果再继续——这就是 "agent team"。子 agent 在哪儿跑由 `settings.json` 的 `runner.mode` 决定:

| mode | 跑在哪 | 需要 Docker? | 怎么回事 |
|---|---|---|---|
| `local`(默认) | 本进程内 | 否 | 共用同一 Memory 后端(状态面共享),各有独立 session + 角色 prompt |
| `docker` | 一次性容器(`--rm`) | 是 | 把不可信代码/命令的"本地破坏"关进容器;上下文随 `task` 注入,结果从 stdout 带回 |

```
you> 派一个"唐代诗人"子 agent 写一句关于秋天的诗,把它写的告诉我
agent> 子代理(唐代诗人)写的诗句是:落叶满长安   # ← 主 agent 派活、拿回结果
```

切到容器模式:`settings.json` 设 `runner.mode=docker`、`config.local.json` 填 `docker_image`,再 `docker build -t my-agent:latest .`。两条铁律:① 容器只隔离**本地破坏**,挡不住对外副作用(那些仍走权限闸);② 子 agent 的工具表不含 `spawn_subagent`,不会无限套娃。详见 [ARCHITECTURE.md](ARCHITECTURE.md#子-agent-与执行后端agent-team)。

## 记忆:短期 + 长期

记忆是 `Memory` 接口,五种后端启动时选(默认 filewiki,本地文件、无需 DB)。在后端之上还套了一层**分层记忆**(`daily_summary` 默认开):

- 对话按天归档,filewiki 落成 `memory/<session>/2026-06-26/raw.jsonl`。
- 每轮对话后把当天总结成一份 ≤2000 字摘要(`digest`),退出前兜底再总结一次——不用定时任务。
- 下次对话按**新近度权重 + 字符预算**召回近 N 天的每日摘要 + 当天原始,于是它"记得前天昨天"。
- 默认保留 **30 天**,超期自动清理;这些都在 `settings.json` 的 `memory.*` 或后台里调。

> 设计见 [ARCHITECTURE.md 的分层记忆](ARCHITECTURE.md#分层记忆短期--长期架在后端之上)。短期/长期共用同一抽象接口,跟用哪个后端无关。

## 加一个自己的工具

在 `harness/tools.go` 照着 `NowTool` / `ReadFileTool` 写一个,然后在 `main.go` 里 `reg.Add(YourTool())`。一个工具就三件事:**名字 + 给模型看的参数 schema + 真正执行的 Go 函数**。

## 配置与安全

两层,职责不重叠(详见 [ARCHITECTURE.md](ARCHITECTURE.md)):

- **密钥/连接串/基础设施地址** → `config.local.json`(DeepSeek key、PG DSN、Redis 地址、docker 地址/镜像),**gitignored,绝不进 git、绝不进后台网页**。
- **可调设置** → `settings.json`(系统提示词、模型、max_steps、记忆后端、执行后端 runner.mode、权限),后台网页可改,缺失会按默认生成。

## 路线图

已知缺口与企业级待补项(安全闸 / 健壮性 / 流式 / 上下文压缩 / 可观测 / MCP / 语义记忆…)统一维护在 **[ARCHITECTURE.md 的路线图](ARCHITECTURE.md#路线图待完善欢迎认领)**,欢迎认领。

## 参与

练手项目,欢迎一起折腾:发现问题或有想法就提 issue;想加功能就 fork → PR。路线图里任意一条都可以认领。

## License

[MIT](LICENSE)
