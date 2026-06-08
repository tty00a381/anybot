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
- `plugins.d/<PluginID>.yaml`：每个插件的配置。
- `anybot.lock`：插件安装锁，记录本地生成的 `PluginID` 以及内置来源或外部 module 来源。
- `plugins.gen.go`、`main.go`、`go.mod`：由 AnyBot 管理的可构建生成宿主。
- `.anybot/`：运行时状态和插件私有数据目录，默认包含 `.anybot/store.json`。

## 安装插件

先生成一个本地插件并安装到机器人工作目录：

```sh
anybot dev plugin hello -dir ../anybot-hello -module example.com/anybot-plugin/hello
cd ../anybot-hello
go mod tidy
go test ./...

cd ../mybot
anybot plugin add example.com/anybot-plugin/hello -replace ../anybot-hello
anybot plugin status
anybot plugin inspect <id>
anybot plugin enable <id>
anybot up
```

安装已发布插件时使用 module 版本：

```sh
anybot plugin add example.com/anybot-plugin/hello@v0.1.0
anybot plugin status
anybot plugin enable <id>
anybot up
```

每个已安装插件只有一个身份：本地随机生成的 `PluginID`。`anybot plugin add` 会为外部插件生成新的 `PluginID` 并写入 `anybot.lock`；`anybot init` 也会为内置插件生成各自的 `PluginID`。CLI 表格首列会显示 ID 的前缀；只要前缀唯一，就可以用它执行 `inspect`、`config`、`enable`、`disable`、`update` 和 `remove`。插件作者在 Manifest 里声明的 `Name` 只用于展示，不参与唯一标识。

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
