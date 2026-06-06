# anybot 宿主

`anybot` 是 AnyBot 面向最终用户的插件化宿主。它让用户通过配置运行机器人，通过命令管理插件，而不是手写 Go 入口。

## 初始化

```sh
anybot init -dir ./mybot
cd ./mybot
```

生成文件：

- `anybot.yaml`：主配置。
- `plugins.d/`：插件拆分配置目录。
- `anybot.plugins.yaml`：外部插件工作区清单。
- `plugins.gen.go`：外部插件注册代码。
- `main.go`：生成宿主入口。
- `go.mod`：生成宿主 Go 模块。
- `.env.example`：访问令牌环境变量示例。
- `README.md`：当前目录说明。

`plugins.gen.go` 和生成宿主 `main.go` 由 anybot 管理。日常主要编辑 `anybot.yaml` 和 `plugins.d/*.yaml`。

## 启动方式

检查配置：

```sh
anybot doctor
```

使用基础宿主运行：

```sh
anybot run
```

使用生成宿主运行：

```sh
anybot up
```

`up` 做五件事：

1. 确保外部插件工作区存在。
2. 同步 AnyBot 自身和外部插件的 Go module 依赖。
3. 构建生成宿主。
4. 运行生成宿主的 `plugin sync` 和 `plugin check`。
5. 启动生成宿主。

已有构建产物时，也可以拆开执行：

```sh
anybot build
./anybot-bot plugin sync
./anybot-bot plugin check
./anybot-bot
```

生成宿主的 `plugin sync/status/inspect/config/check/enable/disable` 使用完整插件注册表。外部插件构建进宿主后，可以直接用 `./anybot-bot plugin enable <name>` 启用并同步 typed config 默认值，也可以用 `./anybot-bot plugin config <name> key=value` 调整配置。

## 内置插件

查看当前二进制内置插件：

```sh
anybot plugins
```

当前内置：

- `help`：基础帮助命令。
- `echo`：复读命令。
- `admin`：管理员命令。
- `ratelimit`：会话限速中间件。

内置插件也通过配置启用或禁用。

## 插件状态

```sh
anybot plugin status
```

状态表会合并三类信息：

- 注册表中可加载的内置或外部插件。
- `anybot.plugins.yaml` 里的外部插件。
- `anybot.yaml` 和 `plugins.d/` 里的配置。

列含义：

- `名称`：宿主配置名。
- `来源`：内置、外部或仅配置。
- `配置`：是否已有配置项。
- `启用`：是否启用。
- `可加载`：当前二进制是否已经注册这个插件。
- `版本`：插件清单版本。
- `模块`：外部插件模块引用。

## 添加外部插件

外部插件应导出 `sdk.Spec[T]`，默认变量名是 `Module`。

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0
```

如果省略版本，`plugin add` 会解析当前 `latest` 并把具体版本写入 `anybot.plugins.yaml`。这样命令仍然简短，生成宿主的构建却不会随远端最新版本漂移。显式写 `@latest` 或 `-version latest` 也会被收敛成解析到的版本。

指定导出变量名：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0 -symbol Weather
```

指定本地开发替换：

```sh
anybot plugin add github.com/acme/anybot-weather -replace ../anybot-weather
```

`-replace` 按执行命令的当前目录解析，然后写成相对宿主目录的路径。本地替换不会解析远端版本；没有版本时生成宿主会按模块主版本写入占位 `require` 并加上 `replace`，例如普通模块使用 `v0.0.0`，`/v2` 模块使用 `v2.0.0`；已有版本则保留版本配合 `replace`。

指定宿主配置名：

```sh
anybot plugin add github.com/acme/anybot-weather -name daily_weather
```

添加插件会：

1. 更新 `anybot.plugins.yaml`。
2. 重写 `plugins.gen.go`。
3. 在 `plugins.d/<name>.yaml` 中创建默认禁用的配置占位。
4. 同步外部插件的 `require` 和 `replace`。

确认配置后启用：

```sh
anybot plugin enable daily_weather
anybot up
```

## 更新外部插件

```sh
anybot plugin update daily_weather -version v0.2.0
anybot plugin update daily_weather -replace ../anybot-weather
anybot plugin update daily_weather -clear-replace
```

`update` 只修改 `anybot.plugins.yaml`、`plugins.gen.go` 和 `go.mod`，不会改写 `plugins.d/<name>.yaml`。它适合在不丢失对话插件复杂配置的前提下切换版本、修正导出符号或切换本地开发路径。`-version latest` 会先解析成具体版本再写入；解析失败时不会改写工作区清单。

## 移除插件

```sh
anybot plugin remove daily_weather
```

移除会更新工作区、重写生成代码、清理 Go module 条目，并删除对应插件配置。

内置插件不能移除，只能禁用：

```sh
anybot plugin disable help
```

## 同步插件配置

```sh
anybot plugin sync
```

同步会读取当前可加载插件的默认配置，递归补齐缺失字段。用户显式写下的字段不会被覆盖。

外部插件还没有构建进当前二进制时，会显示为待构建。运行 `anybot up` 后，生成宿主会完成同步。

如果已有生成宿主，也可以运行 `./anybot-bot plugin sync`；它会用外部插件自己的默认配置补齐 `plugins.d/<name>.yaml`，而不是只停留在基础 CLI 的待构建提示。

## 查看插件配置

```sh
anybot plugin inspect help
```

`inspect` 会显示插件来源、配置文件、当前配置和 typed config 默认值。外部插件还没构建进当前二进制时，基础 CLI 会显示为待构建；运行 `anybot up` 后，生成宿主可以展示外部插件的默认配置。

构建后也可以使用 `./anybot-bot plugin enable <name>` 和 `./anybot-bot plugin config <name> key=value` 管理外部插件配置。生成宿主会拒绝不存在的插件名，避免把拼写错误写进 `plugins.d/`。

## 检查插件配置

```sh
anybot plugin check
```

检查只构建当前启用插件的 typed config，不启动 adapter。禁用插件会显示为禁用；未知插件、尚未构建到当前宿主的外部插件、配置校验失败都会让命令返回失败。

基础 `anybot` CLI 可以发现外部插件待构建；构建后的 `./anybot-bot plugin check` 使用完整插件注册表检查外部插件配置。

## 配置拆分

默认配置启用：

```yaml
plugin_config_dir: plugins.d
```

主配置保存 runtime、adapter、security 和简单内置插件。复杂插件配置放在 `plugins.d/<name>.yaml`。文件名就是插件运行名。

例如：

```yaml
# plugins.d/weather.yaml
enabled: true
config:
  command: weather
  api_key: !env WEATHER_API_KEY
```

## 命令一览

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
anybot plugin check [-dir 目录] [-config anybot.yaml]
anybot plugin sync [-dir 目录] [-config anybot.yaml]
anybot plugin enable <name> [-dir 目录] [-config anybot.yaml]
anybot plugin disable <name> [-dir 目录] [-config anybot.yaml]
anybot version
```
