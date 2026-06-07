# 插件开发者手册

这份文档面向插件作者，目标是写一个可交付最终用户使用的 AnyBot 插件。

## 推荐形态

推荐优先写独立插件，独立插件有以下优点：

- 最终用户可以用 `anybot plugin add <module>` 安装。
- 插件有自己的 `go.mod`、测试和版本。
- 配置默认值可以由运行框架自动同步。
- 插件可以被多个机器人工作目录复用。

项目内插件只适合直接使用核心库写机器人程序时使用：

```sh
anybot dev plugin hello -in-project
```

## 从零生成插件

```sh
anybot dev plugin hello -dir ./anybot-hello -module github.com/acme/anybot-hello
cd anybot-hello
go test ./...
```

脚手架只生成一个薄的 starter 插件。多轮对话、群配置、后台任务等能力看 `examples/plugins`，不要让生成器替你选择业务形态。

本地框架源码开发时，脚手架会尽量给 `go.mod` 写入本地 `replace`。如果无法自动判断 AnyBot 版本或源码路径，显式传入：

```sh
anybot dev plugin hello \
  -dir ./anybot-hello \
  -module github.com/acme/anybot-hello \
  -replace /path/to/anybot
```

或：

```sh
anybot dev plugin hello \
  -dir ./anybot-hello \
  -module github.com/acme/anybot-hello \
  -anybot-version v1.2.3
```

## 插件最小结构

脚手架生成的核心代码大致是：

```go
package hello

import (
	"fmt"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command string `yaml:"command"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("command is required")
	}
	return nil
}

var Plugin = absdk.Define(
	absdk.Manifest{
		Name:        "hello",
		Version:     "0.1.0",
		Description: "hello 插件",
	},
	Config{Command: "hello"},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).
			Name("command").
			Handle(func(c *absdk.EventContext) error {
				_, err := c.ReplyText("hello")
				return err
			})
		return nil
	},
)
```

插件对外导出的是 `Plugin`。最终用户添加插件时，`-symbol` 默认就是 `Plugin`。

`Manifest.Name` 是插件在运行框架里的稳定名字，会用于配置键、`plugins.d/<name>.yaml`、日志、路由命名空间和插件数据目录。名字只能使用小写字母、数字和下划线，并且必须以字母开头，例如 `weather`、`group_memo`。

## 安装到本地机器人工作目录

假设插件 module 是 `github.com/acme/anybot-hello`，本地插件目录是 `../anybot-hello`：

```sh
cd ../mybot
anybot plugin add github.com/acme/anybot-hello -name hello -replace ../anybot-hello
anybot plugin enable hello
anybot up
```

构建后用生成的二进制做最终检查：

```sh
./anybot-bot plugin sync
./anybot-bot plugin check
./anybot-bot plugin inspect hello
```

原因是基础 `anybot` 二进制不能预先知道你的外部插件。`anybot up` 构建出的 `anybot-bot` 才包含完整插件注册表。

## 配置模型

插件配置使用普通 Go struct，并通过 YAML 标签映射到 `plugins.d/<name>.yaml` 的 `config` 下。

```go
type Config struct {
	Command       string   `yaml:"command"`
	AllowedGroups []string `yaml:"allowed_groups"`
	MemoryTTL     string   `yaml:"memory_ttl"`
}
```

`absdk.Define` 的第二个参数是默认配置。运行框架同步配置时，会递归补齐缺失字段，保留用户已经写过的字段。

配置可实现 `Validate() error`：

```go
func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if cfg.MemoryTTL != "" {
		if _, err := time.ParseDuration(cfg.MemoryTTL); err != nil {
			return err
		}
	}
	return nil
}
```

最终用户配置可以使用 `!env`：

```yaml
enabled: true
config:
  api_key: !env WEATHER_API_KEY
```

插件收到的 typed config 中，`api_key` 会是环境变量值。

## 路由

`*absdk.Context` 是插件安装上下文。插件通常通过它注册路由：

```go
ctx.Command("hello").Handle(func(c *absdk.EventContext) error {
	_, err := c.ReplyText("hello")
	return err
})
```

常用入口：

- `ctx.On(rules...)`：通用事件路由。
- `ctx.OnMessage(rules...)`：消息事件路由。
- `ctx.Command(names...)`：命令路由，默认识别 `/`、`!`、`.`。
- `ctx.Use(middleware...)`：只影响当前插件后续注册的路由。

常用规则：

- `absdk.Any()`：匹配所有事件。
- `absdk.MessageEvent()`：匹配消息事件。
- `absdk.Group()`、`absdk.Private()`：匹配群聊或私聊。
- `absdk.ToMe()`：私聊或群聊中提及机器人。
- `absdk.Contains("文本")`：文本包含。
- `absdk.Prefix("前缀")`：文本前缀，并写入 `rest`。
- `absdk.CommandRule("name")`：命令规则。
- `absdk.RegexRule(pattern)`：正则，并写入 `matches` 和命名分组。
- `absdk.All(...)`、`absdk.AnyOf(...)`、`absdk.Not(...)`：组合规则。
- `absdk.AllowedGroups(cfg.AllowedGroups...)`：按配置限制群消息；私聊是否处理由其他规则决定。

路由可以命名、设置优先级、添加局部中间件：

```go
ctx.OnMessage(absdk.ToMe()).
	Name("hello.mention").
	Priority(10).
	Use(absdk.Timeout(5 * time.Second)).
	Handle(func(c *absdk.EventContext) error {
		return nil
	})
```

处理函数中的 `EventContext` 是 SDK 暴露的事件上下文，常用方法：

- `c.Text()`：当前事件文本。
- `c.Command()`、`c.Args()`、`c.Argv()`：命令匹配结果。
- `c.UserID()`、`c.GroupID()`、`c.SelfID()`：标准 ID。
- `c.Target()`：自然回复目标。
- `c.ReplyText(text)`、`c.Reply(chain)`：回复当前事件。
- `c.Session()`、`c.UserSession()`、`c.GroupSession()`：会话存储。
- `c.Pass()`：让出当前路由，继续匹配后续路由。
- `c.Stop()`、`c.StopError()`：停止事件传播。

## 消息

SDK 的 `sdk/message` 是协议无关消息链：

```go
import "github.com/tty00a381/anybot/sdk/message"

chain := message.New(
	message.Text("你好"),
)
_, err := c.Reply(chain)
```

通用消息段：

- `message.Text(text)`
- `message.Image(file)`
- `message.Video(file)`
- `message.Raw(kind, data)`

如果插件确实绑定 OneBot v11，可以使用 `adapters/onebot11` 的消息段和客户端：

```go
import "github.com/tty00a381/anybot/adapters/onebot11"

_, err := c.Reply(message.New(
	onebot11.At(c.UserID()),
	onebot11.Image("file:///tmp/a.png"),
))
```

## 会话存储

插件应优先使用 SDK 的 typed state helper，让状态自动落在当前插件命名空间：

```go
state, err := absdk.UserState[State](ctx, c, "state").LoadOr(State{})
if err != nil {
	return err
}
state.Count++
return absdk.UserState[State](ctx, c, "state").Save(state, time.Hour)
```

读改写可以用 `Update` 收束成一次状态操作：

```go
state, err := absdk.UserState[State](ctx, c, "state").Update(State{}, time.Hour, func(state *State) error {
	state.Count++
	return nil
})
```

常用维度：

- `absdk.ConversationState[T](ctx, c, "key")`：自然会话 typed 状态。
- `absdk.UserState[T](ctx, c, "key")`：用户维度 typed 状态。
- `absdk.GroupState[T](ctx, c, "key")`：群或频道维度 typed 状态。
- `absdk.NamedState[T](ctx, c, "key", "scope")`：插件自定义维度 typed 状态。
- `ctx.Session(c)`：自然会话。群里按群和用户区分，私聊按用户区分。
- `ctx.UserSession(c)`：用户维度。
- `ctx.GroupSession(c)`：群或频道维度。
- `ctx.SessionBy("key")`：插件自定义维度。

底层 `Session` 仍可用于原始字节或特殊 JSON 读写。默认运行框架使用文件存储，位置在 `runtime.data_dir/store.path`。

## 多轮对话

`ctx.Dialogue` 提供轻量状态机，适合问答式配置、确认流程和小型引导。

```go
dialogue := ctx.Dialogue(
	"bind",
	absdk.DialogueWithTTL(5*time.Minute),
	absdk.DialogueWithScope(absdk.DialogueScopeUser),
)

dialogue.Step("ask_name", func(turn *absdk.DialogueTurn) error {
	name := strings.TrimSpace(turn.Text())
	if name == "" {
		return turn.NextText("ask_name", nil, "请输入名称")
	}
	return turn.EndText("已绑定：" + name)
})

ctx.Command("bind").Handle(func(c *absdk.EventContext) error {
	return dialogue.BeginText(c, "ask_name", nil, "请输入要绑定的名称")
})
```

作用域：

- `DialogueScopeConversation`：自然会话，默认。
- `DialogueScopeUser`：用户维度。
- `DialogueScopeGroup`：群或频道维度。

## 后台任务与主动消息

需要后台循环时使用生命周期托管任务：

```go
ctx.Every("tick", time.Minute, func(runCtx context.Context) error {
	ctx.Logger().Info("tick")
	return nil
}, absdk.TaskImmediate())
```

如果任务失败应停止整个机器人：

```go
ctx.Go("worker", worker, absdk.TaskCritical())
```

主动发送消息时先等待动作通道可用：

```go
import "github.com/tty00a381/anybot/adapters/onebot11"

ctx.Go("welcome", func(runCtx context.Context) error {
	if err := ctx.WaitActionReady(runCtx); err != nil {
		return err
	}
	_, err := ctx.SendText(runCtx, absdk.ReplyTarget{
		Protocol: onebot11.Protocol,
		GroupID:  "123456",
	}, "机器人已启动")
	return err
})
```

`OnReady` 的含义是事件调度器已经准备好，但适配器还没有进入 `Start`。因此需要动作客户端时不要只依赖 `OnReady`，应使用 `WaitActionReady` 或 `OnAdapterState`。

## 插件私有数据目录

需要自管文件、缓存、索引或数据库时使用：

```go
dir, err := ctx.DataDir()
if err != nil {
	return err
}
path := filepath.Join(dir, "cache.json")
```

运行框架会创建：

```text
runtime.data_dir/plugins/<插件名>/
```

小型状态优先使用 `Session`，只有需要控制文件格式或接入外部存储时再用 `DataDir()`。

## 运行期写回配置

插件可以写回自己的 `config` 字段：

```go
if ctx.Config().Available() {
	err := ctx.Config().Set(c.Context, "command", "hello")
	if err != nil {
		return err
	}
}
```

批量写入：

```go
err := ctx.Config().SetAll(c.Context,
	absdk.ConfigAssignment{Path: []string{"city", "default"}, Value: "Shanghai"},
	absdk.ConfigAssignment{Path: []string{"reply", "suffix"}, Value: "今天也要开心。"},
)
```

重置字段：

```go
err := ctx.Config().Reset(c.Context, "city.default")
```

写回能力只在运行框架注入配置存储时可用。测试或直接核心库嵌入场景可能不可用，要检查 `Available()` 或处理 `ErrConfigStoreUnavailable`。

## 权限与安全

框架超级用户来自 `security.superusers`。插件中常用：

```go
route.Use(absdk.RequireSuperUser())
```

如果插件自己有管理员配置：

```go
route.Use(absdk.RequireAdmin(cfg.Admins...))
```

`RequireAdmin` 在配置为空时会回退到框架超级用户；如果 `security.superusers` 也为空，这条管理命令就没有可授权用户。

群白名单只筛群消息，私聊不会因为白名单非空而被拦掉：

```go
ctx.OnMessage(absdk.AllowedGroups(cfg.AllowedGroups...)).
	Handle(handler)
```

## OneBot v11 专属能力

如果只需要回复消息，优先用协议无关的 `Reply`、`Send`。需要群管、文件、好友列表等 OneBot v11 动作时再取类型化客户端：

```go
client, ok := onebot11.ClientFrom(c)
if !ok {
	return fmt.Errorf("需要 OneBot v11 客户端")
}
return client.SetGroupBan(c.Context, c.GroupID(), c.UserID(), 10*time.Minute)
```

常用动作：

- `SendPrivateMessage`、`SendGroupMessage`、`SendMessage`
- `DeleteMessage`、`GetMessage`
- `GetGroupList`、`GetGroupInfo`、`GetGroupMemberInfo`、`GetGroupMemberList`
- `SetGroupBan`、`SetGroupWholeBan`、`SetGroupKick`、`SetGroupAdmin`
- `UploadGroupFile`、`GetGroupRootFiles`、`GetGroupFileURL`
- `GetLoginInfo`、`GetStatus`、`GetVersionInfo`

## 测试插件

脚手架会生成基础测试。推荐至少覆盖：

- 默认配置可以安装。
- `Validate()` 会拒绝非法配置。
- 命令路由能注册并处理消息。
- 会话、多轮对话或配置写回这类有状态逻辑。

使用 `sdk/testkit` 可以写轻量单元测试，不需要手写假 Adapter 或假 Client：

```go
app := testkit.NewApp()
if err := app.InstallDefault(Plugin); err != nil {
	t.Fatal(err)
}
if err := app.DispatchText("/hello"); err != nil {
	t.Fatal(err)
}
if got := app.LastReplyText(); got != "hello" {
	t.Fatalf("reply = %q", got)
}
```

群聊提及规则可以显式构造：

```go
err := app.DispatchText("看看这个", testkit.InGroup("100"), testkit.MentionSelf())
```

需要端到端验证时，把插件用 `-replace` 安装到一个临时机器人工作目录，再跑：

```sh
anybot build
./anybot-bot plugin sync
./anybot-bot plugin check
go test ./...
```

## 插件交付前自查

- `go test ./...` 通过。
- `go vet ./...` 没有明显问题。
- `Plugin` 的 `Manifest.Name` 稳定，只使用小写字母、数字和下划线，且不随包名或仓库名随意变化。
- 默认配置够保守，插件安装后默认禁用，由用户显式启用。
- 配置项能通过 `anybot plugin inspect <name>` 看懂。
- 外部服务密钥支持 `!env`，不要鼓励用户明文写进仓库。
- 后台任务能响应 context 退出，不泄漏 goroutine。
- 主动消息等待 `WaitActionReady`。
