# groupmemo

群便签插件示例：管理员用 `/remember <内容>` 更新当前群便签，群友用 `/memo` 查看。

示例覆盖：

- `AllowedGroups` 群白名单。
- `RequireAdmin` 管理员权限。
- `GroupState[T]` 群维度 typed state。

在仓库根目录本地测试：

```sh
go test ./examples/plugins/groupmemo
```
