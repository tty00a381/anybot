# AnyBot

AnyBot 是一个 Go 实现的聊天机器人框架。

![AnyBot](assets/waifu.png)

## 安装

> 需要 Go 1.24 或更新版本。

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
```

## 快速开始

```sh
anybot init mybot
```

这将在当前目录下创建一个名为 `mybot` 的目录，并在其中生成一个机器人。

现在，进入这个目录：

```sh
cd mybot
```

在运行机器人之前，建议先检查一下环境：

```sh
anybot doctor
```

检查无误后，就可以运行机器人了：

```sh
anybot up
```

现在，AnyBot 已经在 `ws://127.0.0.1:6700/` 上运行，等待 OneBot v11 实现端连接。

默认情况下，AnyBot 会使用 **OneBot v11¹** 协议、**反向 WebSocket³** 连接 **实现端²**。若要改用其他协议或连接方式，可以参考下一节，修改配置文件中的相关字段。
>
> **¹ OneBot** 是一个通用的聊天机器人协议, 定义了事件、消息段、动作等抽象概念，屏蔽了不同聊天平台的差异。**v11** 是 OneBot 目前的主流版本。
>
> **² 实现端** 就是实现了上述协议的程序，负责与聊天平台通信、将平台消息转换为协议中的事件，供机器人应用端消费。常见的实现端有 [NapCatQQ](https://github.com/NapNeko/NapCatQQ)、[Lagrange](https://github.com/LagrangeDev/Lagrange) 等。
>
> **³ 反向 WebSocket** 是一种连接方式，机器人框架作为服务器监听一个地址，实现端作为客户端连接到这个地址。相比 HTTP 轮询、Webhook 等传统方式，反向 WebSocket 可以更高效地传递事件和消息，减少延迟和资源消耗。

> 除了 OneBot v11，AnyBot 还计划支持更多协议，如 OneBot v12、Telegram Bot API、Discord API 等。这将在未来的版本中逐步实现。



## 修改配置

AnyBot 的工作目录里主要有两类配置：

- `anybot.yaml`：机器人本身的配置，例如日志、运行时状态、协议适配器、监听地址、安全设置等。
- `plugins.d/`：插件配置目录，每个插件实例都有一个由 `PluginID` 命名的配置文件。

一般只需要先改 `anybot.yaml`：

```yaml
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:6700"
    path: "/"
    access_token_env: ONEBOT_ACCESS_TOKEN

security:
  superusers: []
```

`listen` 是 AnyBot 监听的地址，实现端需要连接到这个地址。`access_token_env` 是访问令牌所在的环境变量名，运行前设置对应环境变量即可：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
```

修改后建议先检查配置：

```sh
anybot doctor
```

插件配置不写在 `anybot.yaml` 里，而是写在 `plugins.d/<PluginID>.yaml` 中。这样同一个插件可以安装多次，也不会因为插件改名影响本地数据和配置。

## 管理插件

查看随框架发布的内置插件：

```sh
anybot plugins
```

查看当前机器人工作目录中的插件：

```sh
anybot plugin status
anybot plugin inspect <id>
```

这里的 `<id>` 可以写完整的 `PluginID`，也可以写 `status` 输出中能唯一匹配的短前缀。插件名称只是展示用，真正稳定的是本地安装时生成并写入 `anybot.lock` 的 `PluginID`。

安装外部插件：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0
anybot plugin status
anybot plugin enable <id>
anybot up
```

本地开发插件时，可以用 `-replace` 指向本地目录：

```sh
anybot plugin add github.com/acme/anybot-weather -replace ../anybot-weather
anybot plugin enable <id>
anybot up
```

调整插件配置：

```sh
anybot plugin config <id> key=value
anybot plugin config <id> -reset key
```

更新或移除插件：

```sh
anybot plugin update <id> -version v0.2.0
anybot plugin disable <id>
anybot plugin remove <id>
```

启用了外部插件后，推荐直接使用 `anybot up`。它会构建生成宿主、同步插件配置、检查插件配置，然后启动机器人。

## 编写插件

插件作者通常只需要依赖 `github.com/tty00a381/anybot/sdk`。不需要直接使用 `core`。

创建一个插件模块：

```sh
anybot dev plugin hello -dir ../anybot-hello -module github.com/acme/anybot-hello
cd ../anybot-hello
go test ./...
```

生成的插件会导出一个 `Plugin`：

```go
package hello

import absdk "github.com/tty00a381/anybot/sdk"

type Config struct {
	Command string `yaml:"command"`
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest: absdk.Manifest{Name: "hello", Version: "0.1.0"},
	DefaultConfig: Config{Command: "hello"},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText("hello")
			return err
		})
		return nil
	},
})
```

把本地插件装进机器人工作目录：

```sh
anybot plugin add github.com/acme/anybot-hello -replace ../anybot-hello -dir ../mybot
anybot plugin status -dir ../mybot
anybot plugin enable <id> -dir ../mybot
anybot up -dir ../mybot
```

如果只是想看写法，可以先读 `examples/plugins`。那里放的是能编译、能运行、面向插件作者的示例。

## 做出贡献

AnyBot 还在快速成型中，欢迎围绕真实使用场景提交改进。比较适合贡献的方向包括：协议适配器、内置插件、插件 SDK、示例、文档，以及让 CLI 更清爽的细节。

仓库结构大致如下：

- `cmd/anybot`：CLI。
- `app/host`：框架运行时。
- `app/plugins`：随框架发布的内置插件。
- `sdk`：插件开发套件。
- `examples/plugins`：编写插件时可参考的示例。
- `core`：核心库，是框架的能力基础。
- `adapters/onebot11`：OneBot v11 适配器。

提交前至少运行：

```sh
go test ./...
go vet ./...
```

如果改动涉及 CLI、脚手架或插件安装流程，建议再真实跑一遍：

```sh
anybot init .local/debug
anybot up -dir .local/debug
```

文档和示例也按这个标准写：让第一次接触 AnyBot 的人能顺着命令跑起来，让插件作者能直接照着结构写出第一个插件。

## 许可证

MIT
