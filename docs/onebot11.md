# OneBot v11

`adapters/onebot11` 把 OneBot v11 事件标准化为 AnyBot `Event`，并把 AnyBot 的动作调用和消息链转换成 OneBot v11 请求。

## 传输方式

反向 WebSocket：

```go
adapter := onebot11.ReverseWS("127.0.0.1:6700",
	onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
)
```

AnyBot 监听端口，协议端主动连接。推荐优先使用这种方式，尤其适合 NapCat。

正向 WebSocket：

```go
adapter := onebot11.WebSocket("ws://127.0.0.1:3001",
	onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
)
```

AnyBot 主动连接协议端，并支持重连间隔配置。

HTTP：

```go
adapter := onebot11.HTTP(
	"http://127.0.0.1:3000",
	"127.0.0.1:6701",
	onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
)
```

第一个参数是协议端 HTTP API 地址，第二个参数是 AnyBot 事件回调监听地址。监听地址为空时只启用动作调用。

## 选项

- `WithAccessToken`：配置 Bearer token 和反向连接校验。
- `WithPath`：配置回调或反向 WebSocket 路径。
- `WithHeader`：添加出站连接 HTTP header。
- `WithLogger`：配置适配器日志器。
- `WithConnectionHook`：注册 WebSocket 连接状态钩子。
- `WithDialTimeout`：配置正向 WebSocket 拨号超时。
- `WithActionTimeout`：配置动作调用默认超时。
- `WithReconnectInterval`：配置正向 WebSocket 初始重连间隔。
- `WithReconnectMaxInterval`：配置正向 WebSocket 最大重连间隔。

## 配置创建

可以从 YAML 创建适配器：

```go
adapter, err := onebot11.LoadAdapter("core.yaml")
if err != nil {
	log.Fatal(err)
}
```

也可以先读取配置再附加额外 option：

```go
cfg, err := onebot11.LoadConfig("core.yaml")
if err != nil {
	log.Fatal(err)
}
adapter, err := onebot11.AdapterFromConfig(cfg, onebot11.WithLogger(logger))
```

## 连接状态

WebSocket 连接状态可以通过 OneBot v11 hook 观察：

```go
adapter := onebot11.ReverseWS("127.0.0.1:6700",
	onebot11.WithConnectionHook(func(ctx context.Context, event onebot11.ConnectionEvent) {
		slog.Info("连接状态变化", "state", event.State, "transport", event.Transport)
	}),
)
```

AnyBot 运行时还会收到协议无关的 `AdapterState`：

```go
app.OnAdapterState(func(ctx context.Context, state core.AdapterState) {
	slog.Info("动作通道状态", "kind", state.Kind, "ready", state.Ready())
})
```

对于主动消息，使用 `WaitActionReady`：

```go
if err := app.WaitActionReady(ctx); err != nil {
	return err
}
```

反向 WebSocket 在协议端尚未连接时会报告 `disconnected`，表示 AnyBot 已在等待连接但动作客户端暂不可用；正向 WebSocket 拨号失败或断开重连期间也会报告 `disconnected`。插件需要主动消息时，不要直接猜连接状态，应等待 `WaitActionReady` 返回。

## 动作客户端

在处理函数中获取类型化客户端：

```go
client := onebot11.MustClient(c)
info, err := client.GetLoginInfo(c.Context)
if err != nil {
	return err
}
_, err = c.ReplyText(info.Nickname)
return err
```

未封装的 OneBot v11 API 可以使用底层调用：

```go
var out SomeResponse
err := client.Call(c.Context, "some_action", params, &out)
```

动作失败会返回 `core.ActionError`；连接不可用会包装为 `core.ErrActionUnavailable`。

## 消息

协议中立消息：

```go
chain := message.New(
	message.Text("你好"),
	message.At("10000"),
	message.Image("file:///tmp/a.png"),
)
_, err := c.Reply(chain)
```

OneBot v11 专属消息段：

```go
chain := message.New(
	onebot11.JSON(`{"foo":"bar"}`),
	onebot11.Share("https://example.com", "标题", "摘要", "https://example.com/cover.png"),
)
```

标准段会被适配器转换成 OneBot 消息段。`message.Raw` 可用于临时协议扩展，但稳定能力应优先放到 `adapters/onebot11` 中。

## NapCat 扩展

NapCat 专属 API 放在 `adapters/onebot11/napcat`：

```go
api := napcat.New(c.Client())
history, err := api.GetGroupMessageHistory(c.Context, c.GroupID(), 0, 20)
```

这样标准 OneBot v11 方法和协议端扩展不会混在一起。

## doctor

直接核心库项目：

```sh
anybot dev doctor -config core.yaml
```

插件化运行框架项目：

```sh
anybot doctor -config anybot.yaml
```

`doctor` 会检查配置、监听地址、URL scheme 和 token 环境变量提示。对正向 WebSocket 或 HTTP，可加 `-connect` 检查远端连接。
