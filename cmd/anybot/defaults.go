package main

import "fmt"

func usage() {
	fmt.Fprintln(stdout, `anybot 命令：
  anybot init [-dir 目录] [-force]
  anybot run [-config anybot.yaml]
  anybot doctor [-config anybot.yaml] [-connect]
  anybot build [-dir 目录] [-o anybot-bot] [-skip-tidy]
  anybot up [-dir 目录] [-o anybot-bot] [-skip-tidy] [-skip-build] [-skip-sync] [-skip-check]
  anybot plugins
  anybot plugin add <module[@version]> [-version 版本] [-replace 本地路径] [-dir 目录]
  anybot plugin update <id> [-version 版本] [-replace 本地路径|-clear-replace] [-dir 目录]
  anybot plugin remove <id> [-dir 目录] [-config anybot.yaml]
  anybot plugin list [-dir 目录]
  anybot plugin status [-dir 目录] [-config anybot.yaml]
  anybot plugin inspect <id> [-dir 目录] [-config anybot.yaml]
  anybot plugin config <id> <key=value>... [-dir 目录] [-config anybot.yaml]
  anybot plugin config <id> -reset <key>... [-dir 目录] [-config anybot.yaml]
  anybot plugin check [-dir 目录] [-config anybot.yaml]
  anybot plugin sync [-dir 目录] [-config anybot.yaml]
  anybot plugin enable <id> [-dir 目录] [-config anybot.yaml]
  anybot plugin disable <id> [-dir 目录] [-config anybot.yaml]
  anybot dev init [-module 模块名] [-dir 目录] [-force]
  anybot dev plugin <名称> [-dir 目录] [-force] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]
  anybot dev new plugin <名称> [同 anybot dev plugin]
  anybot dev doctor [-config core.yaml] [-connect]
  anybot dev run [go run 参数...]
  anybot version`)
}

const defaultConfig = `runtime:
  log_level: info
  workers: auto
  buffer: 256
  serial: conversation
  data_dir: .anybot
  store:
    type: file
    path: store.json

adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
    listen: "127.0.0.1:6700"
    path: "/"
    access_token_env: ONEBOT_ACCESS_TOKEN
    action_timeout: 10s

security:
  superusers: []
`

const defaultHelpPluginConfig = `enabled: true
config:
  command: help
`

const defaultEchoPluginConfig = `enabled: false
config:
  command: echo
`

func defaultBuiltinPluginConfig(source string) string {
	switch source {
	case "help":
		return defaultHelpPluginConfig
	case "echo":
		return defaultEchoPluginConfig
	default:
		return "enabled: false\nconfig: {}\n"
	}
}

const defaultReadme = `# AnyBot

本目录由 ` + "`anybot init`" + ` 生成。编辑 ` + "`anybot.yaml`" + ` 后运行：

` + "```sh" + `
anybot doctor
anybot run
# 或显式执行完整构建/同步/检查流程：
anybot up
` + "```" + `

默认使用 OneBot v11 反向 WebSocket，请让协议端连接 ` + "`ws://127.0.0.1:6700/`" + `。

` + "`anybot init`" + ` 已生成 ` + "`anybot.lock`" + `、` + "`plugins.gen.go`" + `、` + "`main.go`" + ` 和 ` + "`go.mod`" + `。` + "`plugins.gen.go`" + ` 与生成宿主入口 ` + "`main.go`" + ` 由 anybot 管理。` + "`anybot.yaml`" + ` 只保存框架配置，插件配置固定放在 ` + "`plugins.d/<PluginID>.yaml`" + `；` + "`anybot.lock`" + ` 是插件安装实例、内置来源和外部 module 来源的权威记录。运行时状态默认保存到 ` + "`.anybot/`" + `，包括 ` + "`.anybot/store.json`" + ` 和插件私有数据目录。

## 外部插件

安装第三方插件：

` + "```sh" + `
anybot plugin add github.com/acme/anybot-weather@v0.1.0
anybot plugin status
anybot plugin enable <id>
anybot run
` + "```" + `

命令会生成本地 ` + "`PluginID`" + `、更新外部插件注册代码，并在 ` + "`plugins.d/<PluginID>.yaml`" + ` 中加入默认禁用的插件配置项；可用 ` + "`module@version`" + ` 或 ` + "`-version`" + ` 固定版本，省略版本或使用 ` + "`latest`" + ` 时会解析并记录具体版本。本地开发时可加 ` + "`-replace ../path/to/plugin`" + `，它按当前命令目录解析并写成相对机器人目录的路径，不解析远端版本；没有版本时生成宿主会按模块主版本写入占位 ` + "`require`" + `，例如普通模块使用 ` + "`v0.0.0`" + `，` + "`/v2`" + ` 模块使用 ` + "`v2.0.0`" + `。确认配置后用 ` + "`anybot plugin enable <id>`" + ` 启用插件，` + "`anybot plugin config <id> key=value`" + ` 调整配置；要撤回某个字段的本地覆盖并回到 typed config 默认值，用 ` + "`anybot plugin config <id> -reset key`" + `。命令里的 ` + "`<id>`" + ` 可使用 ` + "`anybot plugin status`" + ` 首列显示的唯一前缀。基础 ` + "`anybot plugin inspect`" + ` 和 ` + "`anybot plugin check`" + ` 可以发现外部插件待构建；` + "`anybot run`" + ` 会在启用外部插件后自动构建生成宿主、同步并检查插件配置，然后运行。构建后，` + "`./anybot-bot plugin inspect`" + ` 和 ` + "`./anybot-bot plugin check`" + ` 会使用完整插件注册表做最终校验。要调整外部插件版本或本地替换路径，可使用 ` + "`anybot plugin update <id>`" + `，它不会改写插件配置。` + "`anybot build/up`" + ` 也会固定生成宿主对 AnyBot 自身的依赖，源码开发版会使用本地源码 ` + "`replace`" + `，发布版会使用当前框架版本。移除外部插件可使用 ` + "`anybot plugin remove <id>`" + `。
`
