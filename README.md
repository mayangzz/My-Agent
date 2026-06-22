# My-Agent

> 🛠️ **这是一个练手项目**:从零手写一个最小 agent harness,用来**搞懂"裸大模型 API 到底怎么变成一个会自己调工具干活的 agent"**。当前是**基础版本**,功能会**慢慢完善**。欢迎提 [issue](https://github.com/mayangzz/My-Agent/issues) 和 PR 一起折腾。

Go 实现,后端接 DeepSeek。**故意只用标准库 `net/http`、不套任何 SDK**——目的就是把循环、工具调用、上下文这些事用最少的代码摊开看清楚,而不是被框架糊住。

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

模型只会"开口点单"(说要调哪个工具),**真正去执行的永远是 harness**——模型自己碰不到文件、网络。这就是 "tool use" 和 "harness 的手" 的分工。

## 跑起来

```bash
cp config.example.env config.local.env   # 填入你的 DEEPSEEK_API_KEY
go run .
```

```
you> 现在几点了?
agent> 现在是 2026年6月22日 ...

you> 读取 /etc/hostname 告诉我主机名
agent> ...
```

跑的时候会看到一行 `method=Agent.Run step=1 tool=now args={}`——那就是循环在工作:模型这一步决定调 `now` 工具。

## 加一个自己的工具

在 `harness/tools.go` 照着 `NowTool` / `ReadFileTool` 写一个,然后在 `main.go` 里 `reg.Add(YourTool())`。一个工具就三件事:**名字 + 给模型看的参数 schema + 真正执行的 Go 函数**。

## 配置与安全

- 配置走 `config.local.env`(`DEEPSEEK_API_KEY` / `DEEPSEEK_BASE_URL` / `DEEPSEEK_MODEL`),已被 `.gitignore` 忽略,**密钥不进 git**。环境变量同名可覆盖。
- 模型默认 `deepseek-v4-pro`;DeepSeek 是 OpenAI 兼容接口,换别的兼容模型只改 `config.local.env` 即可。

## 路线图(待完善)

基础版只搭了骨架,以下是已知缺口,会逐步补上(也欢迎来认领):

- [ ] **安全闸**:工具无沙箱,`read_file` 现在能读任意路径(含密钥 / `~/.ssh`)。要加根目录约束 + 危险操作确认。
- [ ] **对话记忆**:REPL 每轮重开,记不住上下文。要跨轮累积消息。
- [ ] **项目感(WorkDir)**:工具锁到某工作目录,启动时把目录树注进 system prompt。
- [ ] **健壮性**:HTTP 重试 / 限流退避;按 rune 安全截断工具输出。
- [ ] **流式输出**:别等整段才出。
- [ ] **上下文管理**:长任务的压缩,避免撑爆窗口。
- [ ] **MCP 接入**:实现 MCP client,自动从外部 server 发现并注册工具。
- [ ] **观测性**:打印 token usage / 成本。

## 参与

练手项目,欢迎一起折腾:发现问题或有想法就提 issue;想加功能就 fork → PR。路线图里任意一条都可以认领。

## License

[MIT](LICENSE)
