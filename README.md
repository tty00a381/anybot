# AnyBot

AnyBot 是一个 Go 实现的聊天机器人框架。其主要由三部分组成：
- 核心库：`core/`，负责事件、路由、中间件、生命周期、状态存储和协议无关动作。
- 插件 SDK：`sdk/`，让插件作者用 typed config、路由、会话、后台任务和私有数据目录写插件。
- 运行框架：`anybot` CLI 与 `app/host`，负责最终用户的工作目录、配置、插件安装、构建、检查和运行。

当前内置 OneBot v11 适配器，可以配合支持 OneBot v11 的协议端使用。默认工作方式是反向 WebSocket：AnyBot 监听 `ws://127.0.0.1:6700/`，协议端主动连接。

## 安装

首先 [安装 Go 1.24+](https://golang.org/dl/)，然后执行以下命令安装 AnyBot：

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

## 快速开始

```sh
mkdir mybot
cd mybot
anybot init
anybot doctor
anybot up
```

`anybot init` 会生成工作目录：

- `anybot.yaml`：运行配置。
- `plugins.d/*.yaml`：每个插件的配置。
- `anybot.lock`：外部插件锁，记录配置名、稳定实例 ID、插件来源、版本、本地替换路径和导出符号。
- `plugins.gen.go`、`main.go`、`go.mod`：由 AnyBot 管理的可构建生成宿主。
- `.anybot/`：运行时状态和插件私有数据目录，默认包含 `.anybot/store.json`。

## 安装插件

以安装一个天气插件为例：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0 -symbol Plugin
anybot plugin status
anybot plugin inspect anybot_weather
anybot plugin enable anybot_weather
anybot up
```

本地插件开发时使用 `-replace`：

```sh
anybot plugin add github.com/acme/anybot-weather -name weather -replace ../anybot-weather
anybot plugin enable weather
anybot up
```

插件配置名只能使用小写字母、数字和下划线，并且必须以字母开头。省略 `-name` 时，模块名末尾的短横线会自动转换为下划线，例如 `anybot-weather` 会得到 `anybot_weather`。配置名用于 CLI 和 `plugins.d/<name>.yaml`；持久化实例 ID 会单独写入 `anybot.lock`。

## 编写插件

独立插件从脚手架开始：

```sh
anybot dev plugin hello -dir ./anybot-hello -module github.com/acme/anybot-hello
cd anybot-hello
go test ./...
```

生成的插件导出 `Plugin`，运行框架通过 `anybot plugin add` 安装它。模板包含 typed config、默认配置、测试和本地安装提示。

## 直接使用核心库

如果你要写一个直接嵌入 AnyBot 核心库的 Go 程序，而不是让最终用户通过配置管理插件：

```sh
anybot dev init -module example.com/bot -dir ./mybot
cd mybot
go mod tidy
anybot dev doctor
go run .
```

核心库路径适合高度定制的程序；普通机器人工作目录优先使用 `anybot init`。

## 文档

- [最终用户手册](docs/users/README.md)：初始化、配置、运行、安装插件、检查问题。
- [插件开发者手册](docs/plugin-developers/README.md)：从零写独立插件，使用 SDK 能力，接入机器人工作目录。
- [维护者手册](docs/maintainers/README.md)：系统边界、运行链路、配置同步、生成代码和测试策略。
- [维护者 API 索引](docs/maintainers/api-index.md)：包、类型、函数和方法索引。

## 包结构

- `cmd/anybot`：统一 CLI。
- `app/host`：配置、插件锁、生成宿主和运行框架装配。
- `app/plugins/*`：内置插件。
- `sdk`：插件 SDK。
- `core`：协议无关核心库。
- `core/message`：协议无关消息链。
- `adapters/onebot11`：OneBot v11 适配器、消息段和动作客户端。
- `internal/scaffold`：项目和插件脚手架。
- `examples/plugins`：面向插件作者的可编译示例。
