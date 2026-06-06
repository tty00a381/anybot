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
  anybot dev init [-module 模块名] [-dir 目录] [-force]
  anybot dev plugin <名称> [-dir 目录] [-force] [-module 插件模块] [-anybot-version 版本] [-replace AnyBot源码路径]
  anybot dev doctor [-config core.yaml] [-connect]
  anybot dev run [go run 参数...]
  anybot version`)
}

const defaultConfig = `runtime:
  log_level: info
  workers: auto
  buffer: 256
  serial: conversation

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

plugin_config_dir: plugins.d

plugins:
  help:
    enabled: true
    config:
      command: help
  echo:
    enabled: false
    config:
      command: echo
`

const defaultReadme = `# AnyBot

本目录由 ` + "`anybot init`" + ` 生成。编辑 ` + "`anybot.yaml`" + ` 后运行：

` + "```sh" + `
anybot doctor
anybot run
# 或使用本目录的可构建宿主工程：
anybot up
` + "```" + `

默认使用 OneBot v11 反向 WebSocket，请让协议端连接 ` + "`ws://127.0.0.1:6700/`" + `。

` + "`anybot init`" + ` 已生成 ` + "`anybot.plugins.yaml`" + `、` + "`plugins.gen.go`" + `、` + "`main.go`" + ` 和 ` + "`go.mod`" + `。` + "`plugins.gen.go`" + ` 与生成宿主 ` + "`main.go`" + ` 由 anybot 管理，日常主要编辑 ` + "`anybot.yaml`" + ` 和 ` + "`plugins.d/*.yaml`" + `。

## 外部插件

安装第三方插件：

` + "```sh" + `
anybot plugin add github.com/acme/anybot-weather@v0.1.0 -symbol Module
anybot plugin status
anybot plugin enable anybot_weather
anybot plugin inspect anybot_weather
anybot plugin check
anybot up
` + "```" + `

命令会更新外部插件注册代码，并在 ` + "`plugins.d/<name>.yaml`" + ` 中加入默认禁用的插件配置项；可用 ` + "`module@version`" + ` 或 ` + "`-version`" + ` 固定版本，省略版本或使用 ` + "`latest`" + ` 时会解析并记录具体版本。本地开发时可加 ` + "`-replace ../path/to/plugin`" + `，它按当前命令目录解析并写成相对宿主目录的路径，不解析远端版本；没有版本时生成宿主会按模块主版本写入占位 ` + "`require`" + `，例如普通模块使用 ` + "`v0.0.0`" + `，` + "`/v2`" + ` 模块使用 ` + "`v2.0.0`" + `。确认配置后用 ` + "`anybot plugin enable`" + ` 启用插件，` + "`anybot plugin config <name> key=value`" + ` 调整配置；要撤回某个字段的本地覆盖并回到 typed config 默认值，用 ` + "`anybot plugin config <name> -reset key`" + `。` + "`anybot plugin inspect`" + ` 查看当前配置和默认配置，` + "`anybot plugin check`" + ` 检查可加载插件配置，` + "`anybot up`" + ` 会构建宿主、同步并检查插件配置，然后运行。要调整外部插件版本、导出符号或本地替换路径，可使用 ` + "`anybot plugin update <name>`" + `，它不会改写插件配置。` + "`anybot build/up`" + ` 也会固定生成宿主对 AnyBot 自身的依赖，避免追随不确定的 ` + "`latest`" + `。移除外部插件可使用 ` + "`anybot plugin remove <name>`" + `。
`
