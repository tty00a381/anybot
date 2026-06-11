# dialogue

多轮对话插件示例：`/profile` 依次询问昵称和城市，保存到用户维度状态；`/whoami` 查看已保存资料。

示例覆盖：

- `ctx.Dialogue` 多步骤流程。
- `DialogueWithScope(DialogueScopeUser)` 用户维度对话。
- `UserState[T]` typed state。

在仓库根目录本地测试：

```sh
go test ./examples/plugins/dialogue
```
