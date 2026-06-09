# AnyBot

AnyBot 是一个面向 Go 插件作者的聊天机器人框架。普通插件只依赖 `github.com/tty00a381/anybot/sdk`，宿主负责插件安装、配置文件、状态存储、私有数据目录和协议适配。

当前内置 OneBot v11 适配器，默认通过反向 WebSocket 接入 NapCat、Lagrange 等 OneBot v11 协议端。

## 机器人工作目录

需要 Go 1.24 或更新版本。

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
mkdir my-bot
anybot init -dir my-bot
cd my-bot
export ONEBOT_ACCESS_TOKEN=你的令牌
anybot doctor
anybot run
```

协议端连接 `ws://127.0.0.1:6700/`，并填写同一个访问令牌。

工作目录中，`anybot.yaml` 保存框架配置，`anybot.lock` 保存插件安装记录，`plugins.d/<PluginID>.yaml` 保存插件实例配置，`.anybot/` 保存运行时状态和插件私有数据。

## 写插件

```sh
anybot dev plugin hello \
  -dir ../anybot-hello \
  -module github.com/you/anybot-hello
cd ../anybot-hello
go test ./...
```

插件入口是导出的 `Plugin` 定义：

```go
var Plugin = sdk.Define(
	sdk.Manifest{Name: "hello", Version: "0.1.0"},
	Config{Command: "hello"},
	func(ctx *sdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Handle(func(c *sdk.EventContext) error {
			_, err := c.ReplyText("hello")
			return err
		})
		return nil
	},
)
```

本地装进机器人工作目录：

```sh
anybot plugin add github.com/you/anybot-hello -replace ../anybot-hello -dir ../my-bot
anybot plugin status -dir ../my-bot
anybot plugin enable <id> -dir ../my-bot
anybot up -dir ../my-bot
```

`anybot up` 会构建带外部插件的生成宿主，同步默认配置，检查配置，然后运行。

## 继续阅读

- [插件作者指南](docs/plugin-developers/README.md)
- [普通用户指南](docs/users/README.md)
- [维护者与发布](docs/maintainers/README.md)
- [示例插件](examples/README.md)

## 质量闸

```sh
make release-check
make release-build VERSION=v1.2.3
```

`release-check` 会运行格式化、测试、vet、race smoke，并实际生成插件和机器人工作目录完成本地安装检查。
