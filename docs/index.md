# AnyBot 文档

AnyBot 是一个 Go module 里的三个项目目录：

1. 核心库：`core/`，负责运行时、路由、中间件、任务、observer、状态存储和协议无关接口。
2. 应用宿主：`app/` 与 `cmd/anybot`，负责配置、插件化运行、外部插件工作区和统一 CLI。
3. 插件 SDK：`sdk/`，负责可复用插件的 typed config、默认配置、校验、工厂和安装上下文。

这三个项目可以单独使用，也可以组合起来。面向最终用户的机器人应交给 `anybot`；想发布插件给别人用，应使用 `sdk`；高度定制的 Go 程序再直接使用 `core`。

## 阅读路径

第一次使用：

1. 阅读 [快速开始](getting-started.md)。
2. 如果你不想写 Go 入口，继续看 [anybot 宿主](anybot.md)。
3. 如果你要直接写 Go 程序，继续看 [核心库](core.md)。

准备写可复用插件：

1. 先看 [插件开发](plugin-development.md)。
2. 再看 [配置](configuration.md) 中的插件配置和 `plugins.d`。
3. 需要主动消息、后台任务、记忆或审计时，再回到 [核心库](core.md) 的 observer、task 和 adapter state。

需要接入协议端：

1. 阅读 [OneBot v11](onebot11.md)。
2. 使用 `anybot doctor` 检查宿主配置，或使用 `anybot dev doctor` 检查直接 core 项目的 OneBot 配置。

理解仓库结构和职责边界：

1. 阅读 [架构分层](architecture.md)。

## 文档列表

- [快速开始](getting-started.md)
- [核心库](core.md)
- [anybot 宿主](anybot.md)
- [插件开发](plugin-development.md)
- [配置](configuration.md)
- [OneBot v11](onebot11.md)
- [架构分层](architecture.md)

## 约定

文档中的用户 ID、群 ID 和模块路径都是示例。OneBot v11 协议端可以使用 NapCat，也可以使用任何兼容 OneBot v11 的实现。

如果配置了访问令牌，AnyBot 和协议端必须使用同一个令牌。推荐通过环境变量传递：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
```
