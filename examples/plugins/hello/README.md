# hello

最小 AnyBot SDK 插件示例：注册一个命令，读取 typed config，并回复一条文本。

本地测试：

```sh
go test ./examples/plugins/hello
```

在独立插件仓库中，核心结构仍然相同：导出 `Plugin`，让最终用户通过 `anybot plugin add <module>` 安装。
