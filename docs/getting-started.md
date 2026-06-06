# 快速开始

AnyBot 有两条入口：

- 使用 `anybot` 主命令：不写 Go 入口，通过配置和插件运行机器人。
- 使用 `anybot dev` 与 `core`：直接写 Go 代码，自己组织路由、中间件、插件和启动流程。

如果目标是给最终用户部署一个机器人，优先使用 `anybot`。如果目标是写一个高度定制的程序，或把 AnyBot 嵌入已有 Go 服务，使用核心库。

## 准备 OneBot v11 协议端

默认示例使用 OneBot v11 反向 WebSocket。也就是说 AnyBot 监听端口，协议端主动连接 AnyBot。

默认地址：

```text
ws://127.0.0.1:6700/
```

如果协议端启用了访问令牌，启动 AnyBot 前设置：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
```

## 使用 anybot

安装：

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

初始化工作目录：

```sh
anybot init -dir ./mybot
cd ./mybot
```

生成内容包括：

- `anybot.yaml`：主配置。
- `plugins.d/`：插件拆分配置目录。
- `anybot.plugins.yaml`：外部插件工作区清单。
- `plugins.gen.go`：外部插件注册代码，由 anybot 管理。
- `main.go`：生成宿主入口，由 anybot 管理。
- `go.mod`：生成宿主的 Go 模块文件。
- `README.md`：当前目录的使用说明。

检查配置：

```sh
anybot doctor
```

直接使用基础宿主运行：

```sh
anybot run
```

使用可构建宿主运行：

```sh
anybot up
```

`up` 会在需要时补齐生成文件、同步 Go module 依赖、构建宿主、同步插件默认配置，然后启动生成的二进制。

## 使用核心库

生成项目：

```sh
anybot dev init -module example.com/bot -dir ./mybot
cd ./mybot
go mod tidy
anybot dev doctor
go run .
```

最小代码如下：

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := core.New(
		core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700",
			onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
		)),
	)
	app.Use(core.Recover(), core.Trace())

	app.Command("ping").Handle(func(c *core.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
```

## 第一个命令

`app.Command("ping")` 默认识别 `/ping`、`!ping` 和 `.ping`。

```go
app.Command("echo").Handle(func(c *core.Context) error {
	_, err := c.ReplyText(c.Args())
	return err
})
```

`c.Args()` 是命令名之后的原始文本；`c.Argv()` 会按空白拆分参数，并保留简单引号和转义处理。

## 下一步

- 想继续写 Go 业务代码，看 [核心库](core.md)。
- 想使用配置和插件运行，看 [anybot 宿主](anybot.md)。
- 想写可复用插件，看 [插件开发](plugin-development.md)。
