# 最终用户手册

这份文档面向“我要把机器人跑起来、装插件、改配置、检查问题”的用户。这里不假设你会写 Go 代码。

## 基本概念

AnyBot 的最终用户入口是一个工作目录。你日常操作的是配置和插件，不直接编辑 Go 代码。

工作目录里最重要的文件：

- `anybot.yaml`：框架配置，包含运行时、协议端、超级用户、插件目录。
- `plugins.d/<配置名>.yaml`：插件配置。默认工作目录使用这种拆分配置，便于一个插件一个文件。
- `anybot.lock`：外部插件锁，记录配置名、稳定实例 ID、Go module、版本、本地替换路径和导出符号。
- `plugins.gen.go`、`main.go`、`go.mod`：AnyBot 生成的可构建宿主。不要手动改，日常用命令更新。
- `.anybot/`：运行时数据，默认包含 `.anybot/store.json` 会话状态文件和插件私有数据目录。

## 初始化

```sh
mkdir mybot
cd mybot
anybot init
```

初始化后先检查配置：

```sh
anybot doctor
```

默认配置使用 OneBot v11 反向 WebSocket：

```text
ws://127.0.0.1:6700/
```

让协议端作为 WebSocket 客户端连接这个地址。NapCat 的 OneBot 网络模式可参考 [NapCatQQ OneBot 网络基础](https://www.napcat.wiki/onebot/network)，OneBot v11 标准可参考 [botuniverse/onebot-11](https://github.com/botuniverse/onebot-11)。

## 运行方式

开发或只使用内置插件时，可以直接运行：

```sh
anybot run
```

装了外部插件，或者希望先同步配置、检查插件、构建完整框架时，使用：

```sh
anybot up
```

`anybot up` 会做这些事：

1. 生成或更新插件锁和生成宿主。
2. 同步 `go.mod` 中 AnyBot 与外部插件依赖。
3. 执行 `go mod tidy`。
4. 构建 `anybot-bot`。
5. 运行生成后二进制的 `plugin sync` 和 `plugin check`。
6. 启动生成后的机器人。

如果你只想构建，不启动：

```sh
anybot build
```

## 配置文件

默认 `anybot.yaml` 大致长这样：

```yaml
runtime:
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
```

### `runtime`

- `log_level`：日志等级，常用 `debug`、`info`、`warn`、`error`。
- `workers`：事件处理并发数。`auto` 使用默认并发；设为 `0` 或负数时走同步处理。
- `buffer`：事件队列大小。
- `serial`：串行化策略。`conversation` 表示同一自然会话内按顺序处理。
- `data_dir`：运行时数据根目录。
- `store.type`：当前支持 `memory` 和 `file`。
- `store.path`：文件存储路径，默认相对 `runtime.data_dir`。

### `adapter`

当前框架配置只装配 OneBot v11：

- `transport.type: reverse_ws`：AnyBot 监听，协议端主动连进来。需要 `listen`。
- `transport.type: websocket`：AnyBot 主动连接协议端 WebSocket。需要 `url`。
- `transport.type: http`：AnyBot 使用 HTTP API；`listen` 非空时也接收 HTTP 事件回调。需要 `url`。
- `path`：反向 WebSocket 或 HTTP 回调路径，必须以 `/` 开头。
- `access_token`：直接写令牌。
- `access_token_env`：从环境变量读取令牌，默认用 `ONEBOT_ACCESS_TOKEN`。
- `headers`：正向连接时附加 HTTP 头。
- `dial_timeout`、`action_timeout`、`reconnect_interval`、`reconnect_max_interval`：连接和动作超时配置。

### `security`

`superusers` 是框架级超级用户 ID。内置 `admin` 插件、SDK 的 `RequireSuperUser()` 和直接核心库的 `core.RequireSuperUser()` 都会使用它。

### 插件配置

插件默认放在 `plugins.d/<配置名>.yaml`：

```yaml
enabled: true
config:
  command: help
```

`enabled` 省略时视为启用。`config` 是插件自己的配置。外部插件如果使用 typed config，`anybot plugin sync` 会把缺失的默认配置补进去，同时保留你已经改过的值。

配置里可以使用环境变量：

```yaml
enabled: true
config:
  api_key: !env WEATHER_API_KEY
```

运行时如果环境变量不存在，配置检查会失败。

## 内置插件

`help`：基础帮助命令，默认启用。

```yaml
enabled: true
config:
  command: help
  lines:
    - 可用命令：
    - /help 显示帮助
```

`echo`：复读命令，默认禁用。

```yaml
enabled: true
config:
  command: echo
```

`admin`：超级用户命令。`users` 为空时使用 `security.superusers`。

```yaml
enabled: true
config:
  users: []
```

`ratelimit`：框架级会话限速中间件。

```yaml
enabled: true
config:
  limit: 5
  window: 1m
```

## 插件管理命令

查看内置插件：

```sh
anybot plugins
```

查看当前工作目录中所有插件状态：

```sh
anybot plugin status
```

安装外部插件：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0 -symbol Plugin
```

常用参数：

- `-name weather`：指定配置名，只能使用小写字母、数字和下划线，并且必须以字母开头。省略时从 module 末尾推导，短横线会转换为下划线。配置名用于 `plugins.d/<name>.yaml` 和 CLI 命令；持久化实例 ID 会写入 `anybot.lock`，不要把配置名当成展示昵称频繁改。
- `-symbol Plugin`：插件 module 导出的 `sdk.Definition` 变量名，默认 `Plugin`。
- `-version v0.1.0`：指定版本。也可写成 `module@version`。
- `-replace ../plugin`：使用本地插件目录，适合本地开发和调试。

启用或禁用：

```sh
anybot plugin enable weather
anybot plugin disable weather
```

查看单个插件：

```sh
anybot plugin inspect weather
```

同步默认配置：

```sh
anybot plugin sync
```

检查配置：

```sh
anybot plugin check
```

修改配置：

```sh
anybot plugin config weather command=weather
anybot plugin config weather city.default=Shanghai
```

撤回某个字段，让它回到插件默认值：

```sh
anybot plugin config weather -reset city.default
```

更新外部插件元数据：

```sh
anybot plugin update weather -version v0.2.0
anybot plugin update weather -replace ../weather
anybot plugin update weather -clear-replace
```

移除外部插件：

```sh
anybot plugin remove weather
```

## 两条真实流程

### 流程一：只使用内置插件

1. 初始化工作目录。

```sh
mkdir mybot
cd mybot
anybot init
```

2. 设置超级用户，启用 `admin` 和 `echo`。

```yaml
security:
  superusers: ["123456"]
```

```sh
anybot plugin enable admin
anybot plugin enable echo
```

3. 检查。

```sh
anybot doctor
anybot plugin check
```

4. 配置协议端连接 `ws://127.0.0.1:6700/`。

5. 启动。

```sh
anybot run
```

6. 在聊天里尝试：

```text
/help
/echo hello
/admin 权限正常
```

### 流程二：安装一个本地外部插件

假设插件目录在 `../anybot-weather`，module 是 `github.com/acme/anybot-weather`。

1. 添加插件。

```sh
anybot plugin add github.com/acme/anybot-weather -name weather -replace ../anybot-weather
```

2. 看状态。

```sh
anybot plugin status
anybot plugin inspect weather
```

这时插件通常会显示为外部、待构建。基础 `anybot` 命令只知道插件锁，完整默认配置要等生成宿主加载外部插件后才能同步。

3. 启用。

```sh
anybot plugin enable weather
```

4. 构建、同步、检查、运行。

```sh
anybot up
```

5. 如果要只检查不启动：

```sh
anybot build
./anybot-bot plugin sync
./anybot-bot plugin check
./anybot-bot plugin inspect weather
```

## 常见问题

外部插件尚未构建到当前框架

说明插件已经写入 `anybot.lock`，但当前运行的基础 `anybot` 二进制没有编进这个外部插件。执行 `anybot up`，或先 `anybot build` 后运行 `./anybot-bot plugin check`。

unknown plugin

配置名写错、配置文件残留，或外部插件没有添加到插件锁。先运行 `anybot plugin status` 看真实名字。

配置文件扩展名

AnyBot 生成的新配置统一使用 `.yaml`。如果目录里已有 `.yml` 配置，读取时也会兼容。

!env 报环境变量不存在

配置使用了 `!env NAME`，但运行时没有设置 `NAME`。设置环境变量后重新运行。

协议端连不上

检查三件事：协议端是否使用反向 WebSocket 客户端模式；URL 是否是 `ws://127.0.0.1:6700/` 或你实际暴露的地址；访问令牌是否和 `ONEBOT_ACCESS_TOKEN` 一致。
