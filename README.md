# mini-harness

一个**从零手写**的最小 agent harness,Go 实现,后端接 DeepSeek。**故意只用标准库 `net/http`、不套任何 SDK**——目的就是把"裸模型 API 怎么变成一个会自己调工具干活的 agent"这件事,用最少的代码摊开给你看。

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
| 循环 | `harness/agent.go` | **harness 的灵魂**:调模型 → 有工具调用就执行、塞回结果 → 再调,直到模型给最终答案 |
| 装配 + REPL | `main.go` | 读配置、注册工具、起一个命令行循环 |

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

模型只会"开口点单"(说要调哪个工具),**真正去执行的永远是 harness**——模型自己碰不到文件、网络。这就是"tool use"和"harness 的手"的分工。

## 跑起来

```bash
cp config.example.env config.local.env   # 填入你的 DEEPSEEK_API_KEY
go run .
```

```
you> 现在几点了?
agent> 现在是 2026年6月20日 下午4点46分。

you> 读取 /etc/hostname 告诉我主机名
agent> ...
```

跑的时候你会看到一行 `method=Agent.Run step=1 tool=now args={}`——那就是循环在工作:模型这一步决定调 `now` 工具。

## 加一个自己的工具

在 `harness/tools.go` 照着 `NowTool` / `ReadFileTool` 写一个,然后在 `main.go` 里 `reg.Add(YourTool())`。一个工具就三件事:**名字 + 给模型看的参数 schema + 真正执行的 Go 函数**。比如你能很快加个 `http_get`、`run_sql`、`lark_send` 把它接到你自己的系统。

## 配置与安全

- 配置走 `config.local.env`(`DEEPSEEK_API_KEY` / `DEEPSEEK_BASE_URL` / `DEEPSEEK_MODEL`),已被 `.gitignore` 忽略,**密钥不进 git**。环境变量同名可覆盖。
- 模型默认 `deepseek-v4-pro`;DeepSeek 是 OpenAI 兼容接口,换成别的兼容模型只改 `config.local.env` 即可。

## 接下来可以往上加(Claude Code 比这多做的事)

这版只有骨架。真要长成"好用",还差这些——每一项都是一道工程:
- **流式输出**(别等整段才出)
- **权限/沙箱**(执行危险工具前先确认)
- **上下文压缩**(聊久了不爆窗口)
- **更稳的工具**(改文件要 diff、行定位、防误改)
- **subagent / 并行**、**MCP 接入**、**错误重试与跑偏纠正**

骨架就在 `agent.go` 那个循环里,往上加料即可。
