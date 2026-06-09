# 最终用户手册

这份文档面向“我要把机器人跑起来、装插件、改配置、检查问题”的用户。这里不假设你会写 Go 代码。

## 基本概念

AnyBot 的最终用户入口是一个工作目录。你日常操作的是配置和插件，不直接编辑 Go 代码。

工作目录里最重要的文件：

- `anybot.yaml`：框架配置，包含运行时、协议端和超级用户。
- `plugins.d/<PluginID>.yaml`：插件配置。默认工作目录使用这种拆分配置，便于一个插件一个文件。
- `anybot.lock`：插件安装锁，记录本地生成的 `PluginID` 以及内置来源或外部 module 来源。
- `plugins.gen.go`、`main.go`、`go.mod`：AnyBot 生成的可构建宿主。不要手动改，日常用命令更新。
- `.anybot/`：运行时创建的数据目录，默认包含 `.anybot/store.json` 会话状态文件和插件私有数据目录。

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

如果已经启用外部插件，`anybot run` 会自动构建并运行生成宿主。希望显式执行完整流程，或者需要跳过构建、同步、检查中的某一步时，使用：

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
```

### `runtime`

- `log_level`：日志等级，常用 `debug`、`info`、`warn`、`error`。
- `workers`：事件处理并发数。`auto` 使用默认并发；`0` 走同步处理；正整数表示固定 worker 数量。
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

插件默认放在 `plugins.d/<PluginID>.yaml`：

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

安装本地插件：

```sh
anybot plugin add example.com/anybot-plugin/hello -replace ../anybot-hello
```

常用参数：

- `-version v0.1.0`：指定版本。也可写成 `module@version`。
- `-replace ../plugin`：使用本地插件目录，适合本地开发和调试。

安装已发布插件时去掉 `-replace` 并写入实际 module 版本：

```sh
anybot plugin add example.com/anybot-plugin/hello@v0.1.0
```

安装后先运行 `anybot plugin status` 查看首列 ID。CLI 支持使用唯一 ID 前缀操作插件；插件 Manifest 里的名称只用于展示。

`PluginID` 是全框架唯一的插件身份概念。内置插件和外部插件都会在 `anybot.lock` 中拥有随机生成的 `PluginID`；`help`、`echo`、`admin`、`ratelimit` 只是内置来源名，不是命令目标 ID。

启用或禁用：

```sh
anybot plugin enable <id>
anybot plugin disable <id>
```

查看单个插件：

```sh
anybot plugin inspect <id>
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
anybot plugin config <id> command=weather
anybot plugin config <id> city.default=Shanghai
```

撤回某个字段，让它回到插件默认值：

```sh
anybot plugin config <id> -reset city.default
```

更新外部插件元数据：

```sh
anybot plugin update <id> -version v0.2.0
anybot plugin update <id> -replace ../weather
anybot plugin update <id> -clear-replace
```

移除插件记录：

```sh
anybot plugin remove <id>
```

## 两条真实流程

### 流程一：只使用内置插件

1. 初始化工作目录。

```sh
mkdir mybot
cd mybot
anybot init
```

2. 设置超级用户，查看随机 `PluginID`，启用 `admin` 和 `echo` 对应的插件记录。

```yaml
security:
  superusers: ["123456"]
```

```sh
anybot plugin status
anybot plugin enable <admin-id>
anybot plugin enable <echo-id>
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

假设插件目录在 `../anybot-hello`，module 是 `example.com/anybot-plugin/hello`。

1. 添加插件。

```sh
anybot plugin add example.com/anybot-plugin/hello -replace ../anybot-hello
```

2. 看状态。

```sh
anybot plugin status
anybot plugin inspect <id>
```

这时插件通常会显示为外部、待构建。基础 `anybot` 命令只知道插件锁，完整默认配置要等生成宿主加载外部插件后才能同步。

3. 启用。

```sh
anybot plugin enable <id>
```

4. 启动。启用外部插件后，`anybot run` 会自动构建、同步、检查并运行生成宿主。

```sh
anybot run
```

5. 如果要只检查不启动：

```sh
anybot build
./anybot-bot plugin sync
./anybot-bot plugin check
./anybot-bot plugin inspect <id>
```

## 常见问题

外部插件尚未构建到当前框架

说明插件已经写入 `anybot.lock`，但当前运行的基础 `anybot` 二进制没有编进这个外部插件。执行 `anybot run` 或 `anybot up`，也可以先 `anybot build` 后运行 `./anybot-bot plugin check`。

unknown plugin

ID 前缀写错、配置文件残留，或外部插件没有添加到插件锁。先运行 `anybot plugin status` 看真实 ID。

配置文件扩展名

AnyBot 生成的新插件配置统一使用 `.yaml`。`plugins.d` 中已有 `.yml` 插件配置时读取和写回会兼容；主配置文件请使用 `anybot.yaml`。

!env 报环境变量不存在

配置使用了 `!env NAME`，但运行时没有设置 `NAME`。设置环境变量后重新运行。

协议端连不上

检查三件事：协议端是否使用反向 WebSocket 客户端模式；URL 是否是 `ws://127.0.0.1:6700/` 或你实际暴露的地址；访问令牌是否和 `ONEBOT_ACCESS_TOKEN` 一致。
