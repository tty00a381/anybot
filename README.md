![AnyBot](assets/waifu.png)

# AnyBot

AnyBot 是一个 Go 实现的聊天机器人框架，当前内置 OneBot v11 适配器。

## 安装

```sh
go get github.com/tty00a381/anybot
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

本仓库使用 Go 1.24。

## 快速开始

本指南假设你已经有一个 OneBot v11 协议端了。如果没有，可以试试 [NapCat](https://github.com/pcrbot/NapCat)。

先生成一个最小项目：

```sh
anybot init -module example.com/bot -dir ./mybot
cd ./mybot
go mod tidy
anybot doctor
go run .
```

生成的项目默认使用反向 WebSocket，地址 `ws://127.0.0.1:6700`。请在协议端启用反向 WebSocket，填入上述地址。

> 如果启用了访问令牌，AnyBot 和协议端要使用同一个令牌：
>
> ```sh
> export ONEBOT_ACCESS_TOKEN=你的令牌
> ```

## 最小代码

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := anybot.New(
		anybot.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700",
			onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
		)),
	)
	app.Use(anybot.Recover(), anybot.Trace())

	app.Command("ping").Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
```

## 一些概念

- `App`：一个机器人进程。适配器、路由、中间件、插件等所有组件，都挂在它上面。
- `Route`：一条处理规则。规则匹配后，即执行你编写的处理函数。
- `Context`：处理函数的工具箱。事件、接口调用、会话状态、路由结果等都从这里拿。
- `Event`：AnyBot 整理过的事件。包含一些常用字段，必要时也能取回原始事件。
- `message.Chain`：一条要发送的消息。由 `message.Segment` 组成。
- `Plugin`：一组可以手动安装的路由、中间件或 hook。

## 写路由

路由就是「什么事件交给什么函数处理」。常见写法有命令、正则、普通消息规则：

```go
app.Command("echo").Handle(func(c *anybot.Context) error {
	_, err := c.ReplyText(c.Args())
	return err
})

app.Regex(`^复读\s+(.+)$`).Handle(func(c *anybot.Context) error {
	matches := c.Match().Vars["matches"].([]string)
	_, err := c.ReplyText(matches[1])
	return err
})

app.OnMessage(anybot.ToMe(), anybot.Contains("帮助")).Handle(func(c *anybot.Context) error {
	_, err := c.ReplyText("我在")
	return err
})
```

常用规则有 `Private`、`Group`、`FromUser`、`InGroup`、`Mentioned`、`ToMe`、`Contains`、`Prefix`、`All`、`AnyOf` 和 `Not`。

处理函数里可以用 `c.Stop()` 不再让后面的路由处理这条事件，也可以 `return c.Pass()` 跳过当前路由，让后面的路由继续。

## 中间件 & 事件顺序

中间件放在处理函数外层，适合做日志、panic 恢复、超时、权限检查、限速等通用功能：

```go
app.Use(
	anybot.Recover(),
	anybot.Trace(),
	anybot.Timeout(10*time.Second),
	anybot.RateLimit(5, time.Minute),
)

app.Command("admin").
	Use(anybot.OnlyGroup(), anybot.SuperUser("10000")).
	Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("权限正常")
		return err
	})
```

AnyBot 默认并发处理事件。如果某个会话里的多条消息必须按顺序处理，如多轮问答、表单收集等临时事务，则可以启用会话串行：

```go
app := anybot.New(
	anybot.WithAdapter(adapter),
	anybot.WithSerialByConversation(),
)
```

## 保存会话状态

默认的内存存储，支持过期时间，适合保存短期状态。`Context` 提供三个常用入口：

```go
c.Session()      // 当前会话
c.UserSession()  // 当前用户
c.GroupSession() // 当前群或频道
```

若需要把状态放进数据库，请实现 `Store` 接口后通过 `WithStore` 传给 `App`。

## OneBot v11 适配器

`adapters/onebot11` 支持三种连接方式：

```go
onebot11.ReverseWS("127.0.0.1:6700")
onebot11.WebSocket("ws://127.0.0.1:6701")
onebot11.HTTP("http://127.0.0.1:3000", "127.0.0.1:6702")
```

推荐优先使用反向 WebSocket，即 AnyBot 监听端口、OneBot v11 协议端主动连进来。WebSocket 连接状态可以通过 hook 观察：

```go
adapter := onebot11.ReverseWS("127.0.0.1:6700",
	onebot11.WithConnectionHook(func(ctx context.Context, event onebot11.ConnectionEvent) {
		slog.Info("连接状态变化", "state", event.State, "transport", event.Transport)
	}),
)
```

常用 OneBot v11 接口已经封装成 Go 方法，如发送消息、撤回消息、读取登录信息、读取好友和群列表、管理群成员、处理加好友或加群请求、检查媒体能力、读取版本状态、操作群文件等。对于未封装的接口，可以直接用 `Call` 或 `CallRaw`。

```go
client := onebot11.MustClient(c)
info, err := client.GetLoginInfo(c.Context)
if err != nil {
	return err
}
_, err = c.ReplyText(info.Nickname)
return err
```

> NapCat 额外提供的接口放在 `adapters/onebot11/napcat`，避免和标准 OneBot v11 方法混在一起。其他协议端如果也有扩展能力，可以按同样思路放在独立包里。
> 
> ```go
> api := napcat.New(c.Client())
> history, err := api.GetGroupMessageHistory(c.Context, c.GroupID(), 0, 20)
> ```

## 发送消息

业务代码可以先拼一条消息，再让当前事件的自然目标接收它：

```go
chain := message.New(
	message.Reply(c.Event().ID),
	message.Text("你好"),
	message.At(c.UserID()),
	message.Image("file:///tmp/a.png"),
)
_, err := c.Reply(chain)
```

OneBot v11 适配器会把这条消息转成 OneBot 消息段。需要 OneBot v11 特有消息段时，可以使用 `onebot11.JSON`、`onebot11.XML`、`onebot11.Share`、`onebot11.Node` 等构造器。

## 插件

插件是一组可以手动安装的路由、中间件或 hook。

CLI 可以生成插件骨架：

```sh
anybot new plugin hello
```

插件就是普通类型，请显式安装到 `App` 上。

```go
type helloPlugin struct{}

func (helloPlugin) Manifest() anybot.Manifest {
	return anybot.Manifest{Name: "hello", Version: "1.0.0"}
}

func (helloPlugin) Install(app *anybot.App) error {
	app.Command("hello").Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("world")
		return err
	})
	return nil
}
```

安装插件：

```go
if err := app.UsePlugins(helloPlugin{}); err != nil {
	log.Fatal(err)
}
```

> 仓库内置了少量示例插件：`plugins/help`、`plugins/echo`、`plugins/admin`、`plugins/ratelimit`。

## CLI

```sh
anybot init [-module 模块名] [-dir 目录] [-force]
anybot run [go run 参数...]
anybot new plugin <名称> [-dir 目录] [-force]
anybot doctor [-config anybot.yaml] [-connect]
anybot version
```

## 示例

- `examples/ping`：最小 ping 机器人。
- `examples/reversews`：反向 WebSocket 接入。
- `examples/websocket`：正向 WebSocket。
- `examples/httpaction`：HTTP 接口调用 & 事件回调。
- `examples/media`：登录信息读取 & 图片发送。
- `examples/pluginbot`：显式插件。
- `examples/permission`：权限控制 & 限速。
- `examples/session`：会话状态。

## 发布

本地开发测试：

```sh
make check
```

发布前检查：

```sh
make release-check
```
