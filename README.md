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

进入这个目录，列出其中的文件：

```sh
cd mybot && ls
```

看到的目录结构应该如下：

```text
mybot/
├── plugins.lock
├── config/
│   ├── anybot.yaml
│   ├── plg_xxx.yaml
├── main.go
├── plugins.gen.go
├── go.mod
├── .env.example
└── README.md
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

机器人目录下会有两类配置：

- `config/anybot.yaml`：机器人本身的配置，例如日志、运行时状态、协议适配器、监听地址、安全设置等。
- `config/<PluginID>.yaml`：插件实例配置文件。

一般只需要先改 `config/anybot.yaml`：

```yaml
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:6700"
    path: "/"
    access_token: !env ONEBOT_ACCESS_TOKEN

security:
  superusers: []
```

`listen` 是 AnyBot 监听的地址，实现端需要连接到这个地址。`access_token` 是访问令牌；不想把令牌明文写进配置时，可以使用 `!env` 从环境变量读取：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
```

修改后建议先检查配置：

```sh
anybot doctor
```

插件配置不写在 `config/anybot.yaml` 里，而是写在 `config/<PluginID>.yaml` 中。这样同一个插件可以安装多次，也不会因为插件改名影响本地数据和配置。

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

## 编写插件

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

如果只是想看写法，可以先读 `examples/plugins`，那里有一些示例插件。

## 做出贡献

AnyBot 还在快速成型中，欢迎围绕真实使用场景提交改进。比较适合贡献的方向有：协议适配器、内置插件、插件 SDK、示例等。

仓库结构大致如下：

- `cmd/anybot`：CLI。
- `app/host`：框架运行时。
- `app/plugins`：随框架发布的内置插件。
- `sdk`：插件开发套件。
- `examples/plugins`：编写插件时可参考的示例。
- `core`：核心库，是框架的能力基础。
- `adapters/onebot11`：OneBot v11 适配器。

提交前记得运行测试：

```sh
go test ./...
go vet ./...
```

欢迎各种功能性的贡献 —— 当然，**清爽的代码**、清晰的文档也很有必要。

## 许可证

MIT
