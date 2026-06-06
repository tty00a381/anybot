![AnyBot](assets/waifu.png)

# AnyBot

AnyBot 是一个 Go 实现的聊天机器人框架。它的目标不是只提供一层协议封装，而是把一个机器人项目真正会遇到的运行时、插件、配置、后台任务、主动消息、状态观察和 OneBot v11 接入组织成一套清爽的工程模型。

当前重点支持 OneBot v11。主入口是 `anybot` CLI：最终用户通过配置和插件运行机器人，插件作者通过 SDK 发布能力，高阶开发者仍然可以直接使用 `core` 内核。

## 安装

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

本仓库使用 Go 1.24。

## 快速开始

```sh
anybot init -dir ./mybot
cd ./mybot
anybot doctor
anybot up
```

生成的配置默认使用 OneBot v11 反向 WebSocket，监听 `ws://127.0.0.1:6700/`。让 NapCat 或其他 OneBot v11 协议端连到这个地址即可。

如果你要写一个直接使用 core 的 Go 程序，可以走开发者子命令：

```sh
anybot dev init -module example.com/bot -dir ./mybot
cd ./mybot
go mod tidy
anybot dev doctor
go run .
```

core 最小代码如下：

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

## 框架里有什么

- `anybot`：统一 CLI 和产品门面，负责初始化、诊断、运行、构建和插件管理。
- `app/host`：插件化宿主，负责配置、内置插件、外部插件工作区和运行时装配。
- `core`：机器人运行时内核，负责适配器、事件调度、路由、中间件、插件、任务和关闭流程。
- `Route`：事件处理规则，支持优先级、命名、中间件、`Stop` 和 `Pass`。
- `Observer`：旁路事件观察者，适合审计、记忆、指标、日志，不参与路由控制。
- `Task`：随 `App` 生命周期托管的后台任务和周期任务，统一处理取消、panic 和失败日志。
- `AdapterState`：协议无关的动作通道状态，用于主动消息前等待连接可用。
- `sdk`：面向可复用插件的 typed config、默认配置、校验、工厂和安装上下文。
- `adapters/onebot11`：OneBot v11 适配器，支持反向 WebSocket、正向 WebSocket 和 HTTP。

## 仓库结构

AnyBot 是一个 Go module 下的三个项目目录：

- `core/`：核心库和协议无关抽象。
- `app/`：插件化宿主、内置插件和 AnyBot 产品运行模型。
- `sdk/`：插件作者使用的配置、清单和安装 SDK。

`cmd/anybot` 是统一 CLI。这样可以让核心库、宿主应用和插件 SDK 拥有明确边界，同时保持一个版本、一套依赖和一个发布单元。

## 文档

完整文档放在 `docs/` 下：

- [文档总览](docs/index.md)
- [快速开始](docs/getting-started.md)
- [核心库](docs/core.md)
- [anybot 宿主](docs/anybot.md)
- [插件开发](docs/plugin-development.md)
- [配置](docs/configuration.md)
- [OneBot v11](docs/onebot11.md)
- [架构分层](docs/architecture.md)

## 示例

- `examples/ping`：最小 ping 机器人。
- `examples/reversews`：反向 WebSocket 接入。
- `examples/websocket`：正向 WebSocket。
- `examples/httpaction`：HTTP 接口调用和事件回调。
- `examples/proactive`：主动消息、adapter readiness、observer 和生命周期任务。
- `examples/dialogueplugin`：只使用 SDK 的对话插件示例。
- `examples/companionplugin`：人格化聊天插件起步示例。
- `examples/media`：登录信息读取和图片发送。
- `examples/pluginbot`：直接安装核心插件。
- `examples/permission`：权限控制和限速。
- `examples/session`：会话状态。

## 命令

`anybot` 的主命令面向最终用户和插件化机器人：

```sh
anybot init [-dir 目录] [-force]
anybot run [-config anybot.yaml]
anybot doctor [-config anybot.yaml] [-connect]
anybot build [-dir 目录] [-o anybot-bot] [-skip-tidy]
anybot up [-dir 目录] [-o anybot-bot] [-skip-tidy] [-skip-build] [-skip-sync] [-skip-check]
anybot plugins
anybot plugin add <module[@version]> [-name 名称] [-symbol Module] [-version 版本] [-replace 本地路径] [-dir 目录]
anybot plugin update <name> [-version 版本] [-symbol Module] [-replace 本地路径|-clear-replace] [-dir 目录]
anybot plugin remove <name> [-dir 目录] [-config anybot.yaml]
anybot plugin list [-dir 目录]
anybot plugin status [-dir 目录] [-config anybot.yaml]
anybot plugin inspect <name> [-dir 目录] [-config anybot.yaml]
anybot plugin config <name> <key=value>... [-dir 目录] [-config anybot.yaml]
anybot plugin config <name> -reset <key>... [-dir 目录] [-config anybot.yaml]
anybot plugin check [-dir 目录] [-config anybot.yaml]
anybot plugin sync [-dir 目录] [-config anybot.yaml]
anybot plugin enable <name> [-dir 目录] [-config anybot.yaml]
anybot plugin disable <name> [-dir 目录] [-config anybot.yaml]
anybot version
```

`anybot dev` 面向直接写 Go 的开发者：

```sh
anybot dev init [-module 模块名] [-dir 目录] [-force]
anybot dev plugin <名称> [-dir 目录] [-force] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]
anybot dev doctor [-config core.yaml] [-connect]
anybot dev run [go run 参数...]
```

发布给别人安装的插件可以直接生成独立 Go module：

```sh
anybot dev plugin hello -dir ./anybot-hello -module github.com/acme/anybot-hello
cd ./anybot-hello
go test ./...
```

源码开发版会自动写入本仓库 `replace`，生成后可直接测试；指定远端 `-anybot-version` 时先运行 `go mod tidy`。

## 本地检查

```sh
make check
```
