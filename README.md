# AnyBot

AnyBot 是一个 Go 实现的聊天机器人框架。它把聊天协议接入、事件路由、插件运行、配置同步、状态持久化和本地机器人工作目录放在同一套清晰边界里；当前内置 OneBot v11 适配器，可直接对接 NapCat、Lagrange 等 OneBot v11 实现。

![AnyBot](assets/waifu.png)

AnyBot 既可以作为 Go 库嵌入到自己的程序里，也可以通过 `anybot` CLI 运行一个可安装插件的机器人工作目录。普通插件作者应优先依赖 `github.com/tty00a381/anybot/sdk`，而不是直接操作底层 `core`。

## 特性

- 协议无关的事件、路由、中间件、回复和主动发送接口。
- 内置 OneBot v11 适配器，支持反向 WebSocket、正向 WebSocket 和 HTTP 传输。
- 插件使用 typed config 定义默认配置、校验逻辑和安装入口。
- 插件安装由 `anybot.lock` 记录，运行身份由本地生成的 `PluginID` 管理，插件名只用于展示。
- 插件配置固定放在 `plugins.d/<PluginID>.yaml`，框架运行状态默认放在 `.anybot/`。
- 生命周期托管后台任务、周期任务、适配器状态观察、`WaitActionReady` 和事件旁路观察者。
- 内置 `help`、`echo`、`admin`、`ratelimit` 插件，并支持外部 Go module 插件。

## 安装

需要 Go 1.24 或更新版本。

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

在源码仓库中开发时，也可以直接运行：

```sh
go run ./cmd/anybot version
go run ./cmd/anybot plugins
```

## 快速开始

创建机器人工作目录：

```sh
anybot init mybot
cd mybot
```

默认配置使用 OneBot v11 反向 WebSocket，监听 `ws://127.0.0.1:6700/`。在协议端填入同一个访问令牌，然后启动：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
anybot doctor
anybot run
```

当工作目录启用了外部插件时，使用完整的生成宿主流程：

```sh
anybot up
```

`anybot up` 会生成宿主代码、同步 `go.mod`、构建 `anybot-bot`、同步插件配置、检查配置，然后运行机器人。

## 工作目录

`anybot init` 会生成一个自包含的机器人目录：

```text
mybot/
  anybot.yaml
  anybot.lock
  plugins.d/
  main.go
  plugins.gen.go
  go.mod
  .env.example
```

这些文件的职责是固定的：

- `anybot.yaml`：框架配置，包括运行时、协议适配器和安全配置。
- `anybot.lock`：插件安装锁，记录内置插件和外部 module 插件，是插件安装实例的权威来源。
- `plugins.d/<PluginID>.yaml`：单个插件实例的启用状态和 typed config。
- `main.go`、`plugins.gen.go`：由 AnyBot 生成的本地宿主入口和外部插件注册代码。
- `.anybot/`：默认运行时数据目录，包含 `store.json` 和插件私有数据目录。

不要把插件配置写回 `anybot.yaml`。AnyBot 会拒绝顶层 `plugins` 配置，因为插件实例配置已经由 `PluginID` 和 `plugins.d/` 承担。

## 配置文件

新目录的默认 `anybot.yaml` 大致如下：

```yaml
runtime:
  log_level: info
  workers: auto
  buffer: 256
  serial: conversation
  data_dir: .anybot
  store:
    type: file
    path: store.json

adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:6700"
    path: "/"
    access_token_env: ONEBOT_ACCESS_TOKEN
    max_event_bytes: 1048576
    action_timeout: 10s

security:
  superusers: []
```

插件配置位于 `plugins.d/<PluginID>.yaml`：

```yaml
enabled: true
config:
  command: help
```

typed config 支持 `!env` 读取环境变量，适合保存令牌、密钥和部署环境差异：

```yaml
enabled: true
config:
  api_key: !env WEATHER_API_KEY
```

## 管理插件

查看随框架发布的内置插件：

```sh
anybot plugins
```

查看当前工作目录中的插件实例：

```sh
anybot plugin status
anybot plugin inspect <id>
```

`<id>` 可以使用完整 `PluginID`，也可以使用 `anybot plugin status` 首列显示的唯一前缀。插件的 `Manifest.Name` 只用于展示，不作为配置路径、状态命名空间或 CLI 目标。

安装外部插件：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0
anybot plugin enable <id>
anybot up
```

本地开发插件时使用 `-replace`：

```sh
anybot plugin add github.com/acme/anybot-weather -replace ../anybot-weather
anybot plugin enable <id>
anybot up
```

更新、配置和移除插件：

```sh
anybot plugin update <id> -version v0.2.0
anybot plugin config <id> key=value
anybot plugin config <id> -reset key
anybot plugin disable <id>
anybot plugin remove <id>
```

外部插件的完整配置检查需要生成宿主参与。`anybot doctor` 可以检查基础框架配置；最终运行前用 `anybot up`，或构建后运行：

```sh
./anybot-bot plugin check
./anybot-bot plugin inspect <id>
```

## 编写插件

创建独立插件模块：

```sh
anybot dev plugin hello -dir ../anybot-hello -module github.com/acme/anybot-hello
cd ../anybot-hello
go test ./...
```

最小插件形态：

```go
package hello

import absdk "github.com/tty00a381/anybot/sdk"

type Config struct {
	Command  string `yaml:"command"`
	Greeting string `yaml:"greeting"`
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest: absdk.Manifest{
		Name:        "hello",
		Version:     "0.1.0",
		Description: "最小命令插件",
	},
	DefaultConfig: Config{
		Command:  "hello",
		Greeting: "你好，我是 AnyBot 插件。",
	},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText(cfg.Greeting)
			return err
		})
		return nil
	},
})
```

插件作者通常只需要 SDK：

- `ctx.Command`、`ctx.OnMessage`、`ctx.On` 注册事件路由。
- `ctx.Use` 注册当前插件内的中间件。
- `c.ReplyText`、`ctx.Send`、`ctx.SendText` 发送消息。
- `ctx.Go`、`ctx.Every`、`ctx.GoWhenActionReady` 托管后台任务。
- `ctx.Observe` 旁路观察事件。
- `absdk.ConversationState`、`absdk.UserState`、`absdk.GroupState` 保存 typed runtime state。
- `ctx.DataDir()` 获取并创建当前 `PluginID` 的私有数据目录。

成熟插件应把三类数据分清楚：

- 部署配置：写在 `plugins.d/<PluginID>.yaml`，由 typed config 解析和校验。
- 运行状态：使用 `Session` / typed state，由框架存储托管。
- 私有文件：使用 `DataDir()`，只放插件自己真正拥有的文件。

把本地插件装进机器人工作目录：

```sh
anybot plugin add github.com/acme/anybot-hello -replace ../anybot-hello -dir ../mybot
anybot plugin status -dir ../mybot
anybot plugin enable <id> -dir ../mybot
anybot up -dir ../mybot
```

示例位于 `examples/plugins/`：

- `hello`：最小命令插件。
- `groupmemo`：群白名单、管理员权限和群维度 typed state。
- `dialogue`：多轮对话和用户维度 typed state。

## 包结构

- `core`：底层运行时、事件路由、中间件、生命周期任务、状态存储和协议适配接口。
- `sdk`：普通插件作者的公共入口。
- `adapters/onebot11`：OneBot v11 适配器、消息段和类型化动作客户端。
- `app/host`：机器人工作目录、插件安装锁、配置同步、生成宿主和运行装配。
- `cmd/anybot`：CLI。
- `app/plugins`：随框架发布的内置插件。
- `examples/plugins`：面向插件作者的可编译示例。

## 开发与验证

常用检查：

```sh
go test ./...
go vet ./...
```

构建发版产物：

```sh
scripts/release.sh v1.2.3
```

本地调试 CLI 和生成宿主可以使用单独目录：

```sh
go run ./cmd/anybot init -dir .local/debug
go run ./cmd/anybot doctor -config .local/debug/anybot.yaml
go run ./cmd/anybot up -dir .local/debug
```

生成宿主行为最终发生在 `.local/debug/anybot-bot` 中。外部插件相关的 `plugin check`、`plugin inspect` 等最终校验，应以构建后的宿主为准。

## 许可证

MIT
