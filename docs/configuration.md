# 配置

AnyBot 有两套配置入口：

- `anybot.yaml`：插件化运行框架配置。
- `core.yaml`：直接写 Go 项目时，OneBot v11 适配器读取的简单配置。

成熟项目建议使用 `anybot.yaml`。它把运行时、协议适配器、安全策略和插件配置放在同一套框架模型下。

## anybot.yaml

默认配置：

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

## runtime

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
```

- `log_level`：框架日志级别。
- `workers`：事件处理 worker 数；`auto` 使用运行时默认值。
- `buffer`：事件队列大小。
- `serial`：事件串行策略。`conversation` 表示同一会话内串行处理。
- `data_dir`：框架运行时数据目录，相对路径按 `anybot.yaml` 所在目录解析。
- `store`：框架托管的会话状态存储。默认 `file` 会写入 `runtime.data_dir/store.json`；相对 `path` 必须留在 `runtime.data_dir` 内，绝对 `path` 会按原样使用；需要完全进程内状态时可设为 `memory`。

标准运行框架会把 `runtime.data_dir/plugins/<插件名>/` 作为插件私有数据目录暴露给 SDK。小型状态优先用 SDK 会话存储；需要自管文件、缓存、索引或数据库时再使用插件数据目录。

## adapter

当前框架支持 OneBot v11：

```yaml
adapter:
  protocol: onebot11
  transport:
    type: reverse_ws
```

传输类型：

- `reverse_ws`：AnyBot 监听 WebSocket，协议端连入。
- `websocket`：AnyBot 主动连接协议端 WebSocket。
- `http`：HTTP API 加事件回调。

反向 WebSocket：

```yaml
transport:
  type: reverse_ws
  listen: "127.0.0.1:6700"
  path: "/"
  access_token_env: ONEBOT_ACCESS_TOKEN
  action_timeout: 10s
```

正向 WebSocket：

```yaml
transport:
  type: websocket
  url: "ws://127.0.0.1:3001"
  access_token_env: ONEBOT_ACCESS_TOKEN
  dial_timeout: 10s
  action_timeout: 10s
  reconnect_interval: 3s
  reconnect_max_interval: 30s
```

HTTP：

```yaml
transport:
  type: http
  url: "http://127.0.0.1:3000"
  listen: "127.0.0.1:6701"
  path: "/"
  access_token_env: ONEBOT_ACCESS_TOKEN
  action_timeout: 10s
```

公共字段：

- `path`：HTTP 回调或反向 WebSocket 请求路径，必须以 `/` 开头。
- `access_token`：直接写在配置中的 token。
- `access_token_env`：从环境变量读取 token。
- `headers`：出站连接附加 HTTP header。
- `action_timeout`：动作调用默认超时。

## security

```yaml
security:
  superusers:
    - "10000"
```

`superusers` 是框架级管理员 ID。SDK 插件可以通过 `absdk.RequireSuperUser()` 复用这套身份；直接使用核心库时可以用 `core.RequireSuperUser()` 和 `Context.IsSuperUser()`。

内置 `admin` 插件在未单独配置 `users` 时也会使用 `security.superusers`。

## plugins

插件配置项：

```yaml
plugins:
  help:
    enabled: true
    config:
      command: help
```

- `enabled`：是否启用。缺省时视为启用。
- `config`：传给插件 typed config 的原始 YAML。

建议显式写 `enabled`，这样 `plugin status` 和人工审阅更清楚。

## plugins.d

复杂插件不要挤在主配置里。默认启用：

```yaml
plugin_config_dir: plugins.d
```

插件配置文件名就是运行名：

```yaml
# plugins.d/companion.yaml
enabled: true
config:
  command: ask
  providers:
    - name: main
      api_key: !env OPENAI_API_KEY
```

主配置和 `plugins.d/<name>.yaml` 不能同时配置同名插件。这样归属明确，不会出现 include 顺序覆盖问题。

AnyBot 管理的配置文件统一使用 `.yaml` 扩展名；`plugins.d/*.yml` 会被拒绝，避免同一目录里出现两套命名约定。

## !env

插件配置支持 `!env`：

```yaml
api_key: !env OPENAI_API_KEY
```

解析规则：

- 只在插件 SDK 构建 typed config 前解析。
- 环境变量不存在会报错。
- 适合 token、API key 和其他 secret。

OneBot v11 适配器的 `access_token_env` 是适配器配置字段，不需要写 `!env`。

## 外部插件工作区

`anybot.plugins.yaml` 记录外部插件：

```yaml
module: anybot.local/bot
plugins:
  - name: weather
    module: github.com/acme/anybot-weather
    version: v0.1.0
    symbol: Module
```

本地开发替换：

```yaml
plugins:
  - name: weather
    module: github.com/acme/anybot-weather
    replace: ../anybot-weather
    symbol: Module
```

通常不要手写这个文件，使用：

```sh
anybot plugin add github.com/acme/anybot-weather@v0.1.0
anybot plugin remove weather
```

远端插件省略版本时，`anybot plugin add` 会解析当前 `latest` 并把具体版本写入清单；`-version latest` 也会被收敛成具体版本。本地 `-replace` 开发不会解析远端版本；没有版本时生成框架会按模块主版本写入占位 `require` 并加上 `replace`，例如普通模块使用 `v0.0.0`，`/v2` 模块使用 `v2.0.0`，已有版本则保留版本配合 `replace`。`anybot build/up` 会同步生成框架对 AnyBot 自身和外部插件的 `require/replace`。发布版使用当前框架版本；源码开发版使用本地 `replace`。

## core.yaml

直接写 Go 项目时，`anybot dev doctor` 和 `onebot11.LoadAdapter` 读取 `core.yaml`：

```yaml
protocol: onebot11
transport:
  type: reverse_ws
  listen: "127.0.0.1:6700"
  path: "/"
  access_token_env: ONEBOT_ACCESS_TOKEN
```

它只描述 OneBot v11 适配器，不包含插件运行框架、security 或 `plugins.d`。
