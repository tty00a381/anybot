# 维护者与发布

AnyBot 的公开边界分三层：

- `sdk`：普通插件作者入口。
- `app/host`：机器人工作目录、插件安装锁、配置同步、生成宿主。
- `core`：底层运行时和协议适配器使用的能力。

维护时优先保护这条边界。普通插件不应该为了命令、状态、配置、后台任务或对话流去 import `core`。

## 发布前检查

```sh
make release-check
RELEASE_CHECK_REQUIRE_CLEAN=1 make release-check
```

该命令会执行：

- `gofmt`
- `go test ./...`
- `go vet ./...`
- 关键包 `go test -race`
- 生成独立插件并运行插件测试
- 初始化机器人工作目录
- 通过 `plugin add -replace` 安装本地插件
- 构建生成宿主并运行 `./anybot-bot plugin check`

## 构建发布产物

```sh
make release-build VERSION=v1.2.3
```

产物写入 `dist/v1.2.3/`，包含 Darwin、Linux、Windows 常用架构包和 `checksums.txt`。

## 发版流程

```sh
make release-check
git status --short
git tag v1.2.3
make release-build VERSION=v1.2.3
git push origin v1.2.3
go list -m github.com/tty00a381/anybot@v1.2.3
```

确认远端 module 版本可解析后，再发布 release artifact。发布说明应强调插件作者主路径：`anybot dev plugin`、`sdk.Define`、`testkit`、`plugin add -replace`、`anybot up`。

## 设计守则

- 配置是用户显式设置，状态是运行时记忆，私有文件属于 `DataDir()`。
- `PluginID` 是宿主生成的稳定安装身份，不能由插件作者自定义。
- `Manifest.Name` 只用于展示，不参与持久化命名空间。
- 外部插件的完整校验发生在生成宿主里，基础 CLI 只做安装锁和框架配置检查。
- 聊天侧控制面保持保守，危险操作留给 CLI。
