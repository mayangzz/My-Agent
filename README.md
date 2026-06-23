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
| 循环 | `harness/agent.go` | **harness 的灵魂**:调模型 → 有工具调用就执行、塞回结果 → 再调,直到最终答案;带 session 记忆 |
| 记忆 | `harness/memory.go` + `memstore/` | `Memory` 接口 + 三种实现(inmem / postgres / redis),启动可选 |
| 设置 / 后台 | `settings/` + `admin/` | 系统提示词等抽到 `settings.json`,本地后台网页可改 |
| 装配 + REPL | `main.go` | 读密钥+设置、注册工具、起命令行循环 |

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
cp config.example.json config.local.json   # 填 deepseek_api_key(默认记忆后端是 postgres,本地要有库)
go run .                  # REPL
go run . -memory inmem    # 临时换内存后端(无需 DB,重启即丢)
go run . admin            # 本地后台 http://127.0.0.1:7788,网页改设置
```

```
you> 我叫马炀,帮我记住
agent> 好的,马炀 ...
you> 我刚才说我叫什么?
agent> 你叫马炀。       # ← 跨轮记忆生效
```

跑的时候会看到 `method=Agent.Run step=1 tool=now args={}`——那是循环在工作。REPL 里 `/reset` 清空当前会话记忆。

## 加一个自己的工具

在 `harness/tools.go` 照着 `NowTool` / `ReadFileTool` 写一个,然后在 `main.go` 里 `reg.Add(YourTool())`。一个工具就三件事:**名字 + 给模型看的参数 schema + 真正执行的 Go 函数**。

## 配置与安全

两层,职责不重叠(详见 [ARCHITECTURE.md](ARCHITECTURE.md)):

- **密钥/连接串** → `config.local.json`(DeepSeek key、PG DSN、Redis 地址),**gitignored,绝不进 git、绝不进后台网页**。
- **可调设置** → `settings.json`(系统提示词、模型、max_steps、记忆后端),后台网页可改,缺失会按默认生成。

## 路线图

已知缺口与企业级待补项(安全闸 / 健壮性 / 流式 / 上下文压缩 / 可观测 / MCP / 语义记忆…)统一维护在 **[ARCHITECTURE.md 的路线图](ARCHITECTURE.md#路线图待完善欢迎认领)**,欢迎认领。

## 参与

练手项目,欢迎一起折腾:发现问题或有想法就提 issue;想加功能就 fork → PR。路线图里任意一条都可以认领。

## License

[MIT](LICENSE)
