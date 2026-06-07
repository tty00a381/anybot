# AnyBot 插件示例

这里放面向插件作者的可编译示例。它们都只依赖 `sdk` 和 `sdk/message`，展示普通插件应走的主路径。

建议按这个顺序看：

- `plugins/hello`：最小命令插件，包含 typed config、`Validate`、`Plugin` 导出和单元测试。
- `plugins/groupmemo`：群工具插件，展示群白名单、管理员权限和群维度 typed state。
- `plugins/dialogue`：多轮对话插件，展示 `Dialogue` 和用户维度 typed state。

这些示例是学习材料，不是脚手架模板。新插件从 `anybot dev plugin <name>` 生成 starter，再按示例补业务能力。
