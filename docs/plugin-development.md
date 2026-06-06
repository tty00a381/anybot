# 插件开发

成熟插件应使用 `github.com/tty00a381/anybot/sdk` SDK。它让插件声明清单、typed config、默认配置、校验和安装逻辑，宿主负责解码配置并装进 `App`。

## 生成独立插件

发布给别人安装的插件应是独立 Go module：

```sh
anybot dev plugin hello -dir ./anybot-hello -module github.com/acme/anybot-hello
cd ./anybot-hello
go test ./...
```

生成内容包括：

- `go.mod`：插件 module，依赖当前 AnyBot。
- `go.sum`：使用本地 AnyBot 源码 `replace` 时生成的测试依赖校验和。
- `hello.go`：导出 `Module` 的 SDK 插件。
- `hello_test.go`：只使用 SDK 的路由和回复 smoke test。
- `README.md`：本地安装、配置和发布命令。

源码开发版的 `anybot` 会在插件 `go.mod` 里自动写入本仓库 `replace`，所以生成后可以直接测试。正式发布的 `anybot` 会写入自身版本；这种模式下先运行 `go mod tidy` 补齐远端模块校验和。需要指定框架依赖时，可以显式传：

```sh
anybot dev plugin hello \
  -dir ./anybot-hello \
  -module github.com/acme/anybot-hello \
  -anybot-version v1.0.0
cd ./anybot-hello
go mod tidy
go test ./...
```

可以用 `-template` 选择内置起点：

```sh
anybot dev plugin buddy -template companion -dir ./anybot-buddy -module github.com/acme/anybot-buddy
anybot dev plugin mc-admin -template minecraft -dir ./anybot-mc-admin -module github.com/acme/anybot-mc-admin
```

- `basic`：最小命令插件，适合从空白开始。
- `companion`：人格化聊天插件起点，包含 `ToMe`、群范围限制和用户短期记忆。
- `minecraft`：Minecraft 群管插件起点，包含管理员命令、玩家绑定表和 `ctx.DataDir()` 私有文件。

本地调试时，在机器人工作目录安装这个插件：

```sh
anybot plugin add github.com/acme/anybot-hello -name hello -replace ../anybot-hello
anybot plugin enable hello
anybot up
```

`anybot up` 会启动机器人并占用当前终端。停止机器人后，或在另一个终端里，可以运行 `./anybot-bot plugin inspect hello` 检查插件配置。

发布 Go module 版本后，最终用户安装固定版本：

```sh
anybot plugin add github.com/acme/anybot-hello@v0.1.0 -name hello
anybot plugin enable hello
anybot up
```

## 最小插件

```go
package hello

import (
	"fmt"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command string `yaml:"command"`
}

func (c Config) Validate() error {
	if c.Command == "" {
		return fmt.Errorf("command 不能为空")
	}
	return nil
}

var Module = absdk.Define(
	absdk.Manifest{Name: "hello", Version: "1.0.0", Description: "问候插件"},
	Config{Command: "hello"},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText("world")
			return err
		})
		return nil
	},
)
```

`Module` 是约定导出名。也可以使用其他导出名，但用户添加插件时要通过 `-symbol` 指定。

## Config

插件配置应使用明确的结构体：

```go
type Config struct {
	Command string   `yaml:"command"`
	Admins  []string `yaml:"admins"`
	Limit   int      `yaml:"limit"`
}
```

`sdk.Spec[T]` 会先深拷贝默认配置，再叠加用户 YAML。这样插件拿到的是独立实例，不会因为修改 slice、map 或指针字段污染后续构建。

如果配置类型实现 `Validate() error`，宿主会在安装前调用：

```go
func (c Config) Validate() error {
	if c.Limit < 0 {
		return fmt.Errorf("limit 不能小于 0")
	}
	return nil
}
```

## 环境变量

插件配置支持显式 YAML 标记 `!env`：

```yaml
enabled: true
config:
  api_key: !env OPENAI_API_KEY
```

环境变量解析发生在 typed config 解码之前。缺失环境变量会报错，而不是默默得到空字符串。

## Context

`sdk.Context` 是插件安装上下文。常用方法：

- `Manifest()`：当前插件清单。
- `Logger()`：带 `plugin=<name>` 字段的日志器。
- `Store()`：宿主会话存储。
- `Session()`、`UserSession()`、`GroupSession()`、`SessionBy()`：带插件命名空间的会话存储。
- `DataDir()`：当前插件的私有数据目录。
- `Client()`：动作客户端。
- `Send()`、`SendText()`：主动发送消息。
- `Use()`：注册插件级中间件。
- `UseGlobal()`：注册宿主级全局中间件，仅适合安全策略、全局限流等宿主策略。
- `On()`、`OnMessage()`、`Command()`：注册路由。
- `Observe()`：注册旁路观察者。
- `Go()`、`Every()`：注册生命周期托管任务。
- `OnStart()`、`OnReady()`、`OnShutdown()`、`OnError()`：生命周期和错误钩子。
- `WaitActionReady()`：等待动作客户端可用。
- `OnAdapterState()`：观察适配器状态变化。
- `App()`：返回底层 `*absdk.App`，只在确实需要低层能力时使用。

后台任务名称会自动加插件名前缀。例如插件名是 `memory`，`ctx.Every("compact", ...)` 的任务名会变成 `memory.compact`。

## 路由与中间件

插件注册路由和普通 App 一样：

```go
ctx.Command(cfg.Command).
	Name("hello.command").
	Use(absdk.RateLimit(3, time.Minute)).
	Handle(func(c *absdk.EventContext) error {
		_, err := c.ReplyText("world")
		return err
	})
```

`ctx.Use()` 只作用于当前插件通过 `ctx.On`、`ctx.OnMessage`、`ctx.Command` 注册的后续路由，不会影响其他插件。需要影响整个宿主的能力应显式使用 `ctx.UseGlobal()`；普通对话插件通常不应该使用它。

需要管理员权限时，优先使用宿主级超级用户：

```go
ctx.Command("reload").
	Use(absdk.RequireSuperUser()).
	Handle(func(c *absdk.EventContext) error {
		_, err := c.ReplyText("已重载")
		return err
	})
```

`anybot` 的 `security.superusers` 会进入 App 级超级用户列表。

SDK 暴露了对话插件常用的规则组合能力，插件不需要直接导入 `core`：

- `Private()`、`Group()`：区分私聊和群聊。
- `FromUser()`、`InGroup()`、`AllowedGroups()`、`Mentioned()`、`ToMe()`：限定来源、配置中的群范围和提及。
- `FromSelf()`、`NotFromSelf()`：处理协议端回显或自发消息。
- `All()`、`AnyOf()`、`Not()`：组合规则。
- `Prefix()`、`Contains()`、`RegexRule()`、`RegexpRule()`：匹配自然语言入口。
- `RuleFunc`、`Match`：编写配置驱动或状态驱动的自定义规则。

SDK 也导出了插件作者常用的错误、回执和会话类型：`MessageReceipt`、`ActionResponse`、`ActionError`、`PanicError`、`ErrPass`、`ErrStop`、`ErrUnauthorized`、`ErrRateLimited`、`Session`、`MemoryStore`、`FileStore`。常规插件不需要导入 `core`。

配置里常见的群和管理员列表可以直接接入 SDK helper：`absdk.AllowedGroups(cfg.AllowedGroups...)` 在列表为空时不限制群聊，`absdk.RequireAdmin(cfg.Admins...)` 在列表为空时回退到宿主 `security.superusers`，列表非空时只允许插件配置的管理员。

## 会话状态

对话插件应优先使用安装上下文上的会话 helper，它们会自动加插件名作为命名空间，避免多个插件都保存 `profile`、`state` 或 `history` 时互相覆盖：

```go
ctx.Command("remember").Handle(func(c *absdk.EventContext) error {
	err := ctx.UserSession(c).SaveJSON(c.Context, "profile", profile{Name: c.Args()}, 24*time.Hour)
	if err != nil {
		return err
	}
	_, err = c.ReplyText("记住了")
	return err
})
```

`c.Session()`、`c.UserSession()` 和 `c.GroupSession()` 是 core 级会话视图，不带插件命名空间；SDK 插件只有在明确需要共享状态时才应直接使用。

标准 `anybot` 宿主默认把会话状态持久化到 `runtime.data_dir/store.json`，重启后仍可读取。直接使用 `core.New()` 时默认仍是进程内 `MemoryStore`；测试或嵌入式宿主可用 `absdk.NewMemoryStore()` 或 `core.NewFileStore()` 显式注入。

## 数据目录

需要自管文件、缓存、索引或数据库的插件可以使用私有数据目录：

```go
dir, err := ctx.DataDir()
if err != nil {
	return err
}
path := filepath.Join(dir, "bindings.json")
```

目录由宿主创建，标准路径是 `runtime.data_dir/plugins/<插件名>/`。轻量键值状态优先用 `SessionBy()`；只有当插件确实需要控制文件格式或外部存储时才使用 `DataDir()`。

## Observer

需要观察消息但不参与路由时，使用 `Observe`：

```go
ctx.Observe(absdk.MessageEvent()).Name("memory").Handle(func(obsCtx context.Context, event *absdk.Event) error {
	// 写入记忆、审计或指标
	return nil
})
```

Observer 不受 `Stop` 和 `Pass` 影响，也不会决定命令路由是否继续执行。它适合做后台记录，不适合做交互式命令。

## 后台任务

```go
ctx.Every("refresh", time.Minute, func(taskCtx context.Context) error {
	// 刷新缓存、健康检查、清理临时状态
	return nil
}, absdk.TaskImmediate())
```

任务随 App 生命周期启动和停止。任务返回错误会记录日志；如果任务是插件运行的关键条件，可以使用 `absdk.TaskCritical()`。

## 主动消息

主动消息前应等待动作通道 ready：

消息链使用 `github.com/tty00a381/anybot/sdk/message`。

```go
ctx.Go("startup-message", func(taskCtx context.Context) error {
	if err := ctx.WaitActionReady(taskCtx); err != nil {
		return err
	}
	_, err := ctx.Client().Send(taskCtx, absdk.ReplyTarget{
		Protocol: absdk.ProtocolOneBot11,
		UserID:   "10000",
	}, message.New(message.Text("机器人已连接")))
	return err
})
```

不要通过第一次发送失败来猜测连接状态。目标不存在、权限不足、token 错误和连接未建立是不同问题，应由不同层处理。

## 注册到宿主

直接写 Go 宿主时：

```go
registry := absdk.NewRegistry()
if err := registry.Register(hello.Module.Factory()); err != nil {
	log.Fatal(err)
}
```

如果要把同一个插件注册成宿主中的另一个运行名：

```go
registry.Register(hello.Module.Factory().WithName("daily_hello"))
```

`anybot plugin add` 会根据外部插件工作区生成注册代码。插件作者只需要导出 `sdk.Spec[T]`。

## 本地开发插件

```sh
anybot plugin add github.com/acme/anybot-weather -replace ../anybot-weather
anybot plugin enable anybot_weather
anybot up
```

`anybot up` 会构建带外部插件的生成宿主、同步默认配置并检查插件配置，然后启动机器人。停止机器人后，也可以运行 `./anybot-bot plugin inspect anybot_weather` 和 `./anybot-bot plugin check` 复查。

`-replace` 按执行命令的当前目录解析，然后写成相对宿主目录的路径。本地替换不会解析远端版本；没有版本时生成宿主会按模块主版本写入占位 `require` 并加上 `replace`，例如普通模块使用 `v0.0.0`，`/v2` 模块使用 `v2.0.0`，已有版本则保留版本配合 `replace`。发布前可以用 `anybot plugin update anybot_weather -clear-replace -version v0.1.0` 切回远端版本；继续本地开发时也可以用 `anybot plugin update anybot_weather -replace ../anybot-weather` 改路径。远端插件省略版本或使用 `latest` 时，CLI 会解析并记录具体版本，避免生成宿主构建时追随浮动 latest。`update` 不会改写 `plugins.d/<name>.yaml`，适合保留对话插件的复杂配置。基础 `anybot plugin inspect` 和 `anybot plugin check` 可以发现外部插件待构建；外部插件构建进宿主后，`./anybot-bot plugin enable`、`./anybot-bot plugin config`、`./anybot-bot plugin inspect` 和 `./anybot-bot plugin check` 会使用完整插件注册表做最终校验。

插件可以通过 `ctx.Config().Set`、`SetAll` 和 `Reset` 写回自己的 `config` 字段。写回会立即落盘到 `anybot.yaml` 或对应的 `plugins.d/<name>.yaml`，适合由管理员命令保存插件偏好；当前已经完成解码的运行中配置不会因此自动热刷新，通常应在事件处理或显式迁移命令里使用，而不是在 `Setup` 中做隐式迁移。

## 生成插件骨架

在直接写 Go 的 core 项目里，可以生成项目内插件：

```sh
anybot dev plugin hello
anybot dev plugin buddy -template companion
anybot dev plugin mc-admin -template minecraft
```

生成的插件会写入 `plugins/hello/hello.go`，适合只在当前机器人项目中使用。要发布给别人安装，使用本页开头的 `-module` 独立插件模式。

## 插件示例

仓库里的 `examples/dialogueplugin` 是一个 SDK-only 对话插件示例，覆盖 typed config、自定义规则、会话记忆、自然语言回复、限速和后台任务。它可以作为复杂聊天插件的起点。

`examples/companionplugin` 是人格化聊天插件起步示例，展示 provider HTTP 调用、persona、短期记忆、群聊范围和服务不可用时的本地降级回复。

## 插件设计建议

- 配置使用结构体，不要让用户传 `map[string]any`。
- 默认配置应能代表插件的最小可运行状态。
- secret 使用 `!env`，不要写死在 YAML。
- 主动消息使用 `WaitActionReady`。
- 长期循环使用 `ctx.Go` 或 `ctx.Every`，不要裸起 goroutine。
- 记录、记忆、指标使用 observer，不要伪装成兜底 route。
- 管理员能力优先接入 `security.superusers`。
