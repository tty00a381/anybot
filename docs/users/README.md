# 普通用户指南

这份文档写给只想运行机器人和安装插件的人。

## 初始化

需要 Go 1.24 或更新版本。

```sh
go install github.com/tty00a381/anybot/cmd/anybot@latest
mkdir my-bot
anybot init -dir my-bot
cd my-bot
```

配置协议端连接：

```sh
export ONEBOT_ACCESS_TOKEN=你的令牌
anybot doctor
anybot run
```

默认监听 `ws://127.0.0.1:6700/`。让 OneBot v11 协议端通过反向 WebSocket 连接该地址，并填入同一个访问令牌。

## 安装插件

```sh
anybot plugin add github.com/friend/anybot-weather@v0.1.0
anybot plugin status
anybot plugin inspect <id>
anybot plugin config <id> command=weather
anybot plugin enable <id>
anybot up
```

`<id>` 可以使用 `anybot plugin status` 第一列显示的短 ID。

## 常用命令

- `anybot doctor`：检查框架配置和监听地址。
- `anybot plugin status`：查看插件是否已配置、启用、可用。
- `anybot plugin inspect <id>`：查看插件来源、默认配置、本地配置。
- `anybot plugin config <id> key=value`：修改插件配置。
- `anybot plugin config <id> -reset key`：删除本地覆盖，回到插件默认值。
- `anybot up`：构建外部插件宿主、同步配置、检查配置并启动。

## 目录说明

- `anybot.yaml`：框架运行配置。
- `anybot.lock`：插件安装记录，包含内置插件和外部 module 来源。
- `plugins.d/<PluginID>.yaml`：每个插件实例的启用状态和配置。
- `.anybot/store.json`：框架托管的运行时状态。
- `.anybot/plugins/<PluginID>/`：插件私有数据目录。

不要手动改 `plugins.gen.go`、`main.go` 或生成宿主的 `go.mod`。这些文件由 `anybot` 管理。
