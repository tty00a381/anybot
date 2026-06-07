# 核心库

核心库位于 `github.com/tty00a381/anybot/core`。它不关心配置文件格式，也不关心插件从哪里下载；它只负责把事件、安全地调度到处理代码，并提供一套协议无关的动作、消息、存储和生命周期模型。

## App

`App` 是运行时入口：

```go
app := core.New(
	core.WithAdapter(adapter),
	core.WithWorkers(8),
	core.WithBuffer(256),
	core.WithSerialByConversation(),
	core.WithSuperUsers("10000"),
)
```

常用 option：

- `WithAdapter`：配置协议适配器。
- `WithLogger`：配置 `slog.Logger`。
- `WithStore`：配置会话存储。
- `WithWorkers`：配置并发处理事件的 worker 数。不大于 0 时由适配器回调同步处理。
- `WithBuffer`：配置事件队列大小。
- `WithObserverBuffer`：配置 observer 并发队列容量。
- `WithSerialByConversation`：同一会话内事件串行处理。
- `WithSerialBy`：自定义串行键。
- `WithSuperUsers`：配置 App 级超级用户。
- `WithErrorHandler`：注册路由错误处理器。

`Run(ctx)` 会执行启动钩子、绑定适配器状态、启动后台任务、启动适配器，并在 `ctx` 结束后关闭。

## 路由

路由描述“什么事件交给什么函数处理”。

```go
app.Command("echo").Handle(func(c *core.Context) error {
	_, err := c.ReplyText(c.Args())
	return err
})

app.Regex(`^复读\s+(.+)$`).Handle(func(c *core.Context) error {
	matches := c.Match().Vars["matches"].([]string)
	_, err := c.ReplyText(matches[1])
	return err
})

app.OnMessage(core.ToMe(), core.Contains("帮助")).Handle(func(c *core.Context) error {
	_, err := c.ReplyText("我在")
	return err
})
```

路由可以命名、设置优先级、追加局部中间件：

```go
app.Command("admin").
	Name("admin.command").
	Priority(100).
	Use(core.RequireSuperUser()).
	Handle(func(c *core.Context) error {
		_, err := c.ReplyText("权限正常")
		return err
	})
```

优先级越大越先执行。同优先级保持注册顺序。

## 规则

常用规则：

- `Any`：匹配所有事件。
- `EventType`、`DetailType`：匹配标准化事件类型。
- `MessageEvent`：匹配消息事件。
- `Group`、`Private`：匹配群聊或私聊消息。
- `FromUser`、`FromSelf`、`NotFromSelf`：匹配发送者。
- `InGroup`：匹配群。
- `Mentioned`、`ToMe`：匹配提及或面向机器人的消息。
- `Contains`、`Prefix`：匹配文本。
- `CommandRule`、`CommandWithPrefixes`：命令规则。
- `RegexRule`、`RegexpRule`：正则规则。
- `All`、`AnyOf`、`Not`：组合规则。

规则可以向 `Match.Vars` 写变量。路由匹配后，变量会进入 `Context`：

```go
app.OnMessage(core.Prefix("天气")).Handle(func(c *core.Context) error {
	city := c.String("rest")
	_, err := c.ReplyText("查询：" + city)
	return err
})
```

## Context

`Context` 是处理函数的工具箱：

- `Event()`：当前标准化事件。
- `RawEvent()`：适配器保留的原始协议事件。
- `Text()`：消息文本。
- `UserID()`、`GroupID()`、`SelfID()`：常用 ID。
- `Command()`、`Args()`、`Argv()`：命令信息。
- `Match()`：匹配详情。
- `Client()`：动作客户端。
- `Store()`：会话存储。
- `Session()`、`UserSession()`、`GroupSession()`：会话视图。
- `Reply()`、`ReplyText()`：回复当前事件的自然目标。
- `Stop()`、`StopError()`、`Pass()`：控制路由传播。
- `IsSuperUser()`：判断事件发送者是否为 App 级超级用户。

`Pass()` 表示当前路由让出处理权，后续路由继续匹配；`Stop()` 和 `StopError()` 表示停止传播。

## 中间件

中间件包裹处理函数，适合日志、恢复、超时、权限和限速：

```go
app.Use(
	core.Recover(),
	core.Trace(),
	core.Timeout(10*time.Second),
)

app.Command("group").
	Use(core.OnlyGroup(), core.RateLimit(5, time.Minute)).
	Handle(func(c *core.Context) error {
		_, err := c.ReplyText("群聊命令")
		return err
	})
```

内置中间件：

- `Recover`：捕获 panic，记录栈，并转成 `PanicError`。
- `Trace`：记录路由耗时和事件关键信息。
- `Timeout`：为后续处理函数派生带超时的 context。
- `OnlyPrivate`、`OnlyGroup`：限制消息场景。
- `SuperUser`：按显式用户列表鉴权。
- `RequireSuperUser`：按 App 级超级用户鉴权。
- `RateLimit`、`RateLimitBy`：进程内限速。

## 消息

`message.Chain` 是协议中立消息链：

```go
chain := message.New(
	message.Reply(c.Event().ID),
	message.Text("你好 "),
	message.At(c.UserID()),
	message.Image("file:///tmp/a.png"),
)
_, err := c.Reply(chain)
```

内置段：

- `Text`
- `At`
- `Image`
- `Reply`
- `Face`
- `Record`
- `Video`
- `Raw`

适配器负责把协议中立消息段转为具体协议格式。需要 OneBot v11 专属消息段时，可使用 `onebot11.JSON`、`onebot11.XML`、`onebot11.Share`、`onebot11.Node` 等构造器。

## 会话存储

直接使用 `core.New()` 时默认 `MemoryStore` 是进程内存储，支持 TTL，适合测试和短期状态。标准 `anybot` 插件运行框架会额外注入文件持久化 Store。

```go
app.Command("count").Handle(func(c *core.Context) error {
	session := c.Session()
	var count int
	_, _ = session.LoadJSON(c.Context, "count", &count)
	count++
	if err := session.SaveJSON(c.Context, "count", count, 24*time.Hour); err != nil {
		return err
	}
	_, err := c.ReplyText(fmt.Sprintf("当前会话计数：%d", count))
	return err
})
```

需要文件持久化时可直接使用 `FileStore`；它面向单个进程内的 goroutine 并发，不提供多框架进程之间的文件锁：

```go
store, err := core.NewFileStore(".anybot/store.json")
if err != nil {
	log.Fatal(err)
}
app := core.New(core.WithStore(store))
```

需要数据库时，实现 `Store`：

```go
type Store interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
}
```

然后通过 `WithStore` 注入。

## 后台任务

`App.Go` 和 `App.Every` 注册随生命周期托管的后台任务。任务会在 `App.Run` ready 后启动，随运行 context 取消而停止；panic 会被恢复并记录。

```go
app.Go("startup", func(ctx context.Context) error {
	// 启动后执行一次
	return nil
})

app.Every("refresh-cache", time.Minute, func(ctx context.Context) error {
	// 周期刷新
	return nil
}, core.TaskImmediate())
```

选项：

- `TaskImmediate`：周期任务启动后立即执行一次。
- `TaskCritical`：任务失败时取消 `App.Run`，并把任务错误作为运行错误返回。

## Observer

Observer 是旁路事件观察者。它会匹配事件并异步执行，但不参与 route ordering、`Stop` 或 `Pass`。

适用场景：

- 记忆捕获。
- 审计日志。
- 指标统计。
- 消息回放。
- 低优先级分析。

```go
app.Observe(core.MessageEvent()).Name("audit").Handle(func(ctx context.Context, event *core.Event) error {
	app.Logger().Info("观察消息", "user_id", event.UserID, "text", event.Text)
	return nil
})
```

Observer 收到的是事件副本，避免和主处理链共享可变消息段。默认错误会记录日志，也可以注册错误处理器：

```go
app.OnObserverError(func(ctx context.Context, event *core.Event, err error) {
	app.Logger().Warn("observer failed", "error", err)
})
```

## Adapter State 与主动消息

主动消息不能只靠发送失败来猜连接是否可用。AnyBot 提供协议无关状态：

```go
app.OnAdapterState(func(ctx context.Context, state core.AdapterState) {
	app.Logger().Info("adapter state", "kind", state.Kind, "ready", state.Ready())
})

app.Go("proactive", func(ctx context.Context) error {
	if err := app.WaitActionReady(ctx); err != nil {
		return err
	}
	_, err := app.Client().Send(ctx, core.ReplyTarget{
		Protocol: core.ProtocolOneBot11,
		UserID:   "10000",
	}, message.New(message.Text("已连接")))
	return err
})
```

状态种类：

- `unknown`
- `starting`
- `ready`
- `disconnected`
- `stopped`

`WaitActionReady` 在动作客户端 ready 后返回；运行结束或上下文取消时返回错误。

## 插件

核心库仍然支持低层插件接口：

```go
type Plugin interface {
	Manifest() Manifest
	Install(*App) error
}
```

直接写 Go 项目时可以使用：

```go
if err := app.UsePlugin(myPlugin{}); err != nil {
	log.Fatal(err)
}
```

面向复用和 `anybot` 的插件应使用 [插件 SDK](plugin-development.md)。
