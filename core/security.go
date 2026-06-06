package core

import (
	"fmt"
	"slices"
)

// SuperUsers 返回当前 App 配置的超级用户 ID。
func (a *App) SuperUsers() []string {
	if a == nil {
		return nil
	}
	return append([]string(nil), a.superUsers...)
}

// IsSuperUser 判断给定用户 ID 是否属于当前 App 的超级用户。
func (a *App) IsSuperUser(id any) bool {
	if a == nil {
		return false
	}
	return slices.Contains(a.superUsers, fmt.Sprint(id))
}

// IsSuperUser 判断当前事件发送者是否属于当前 App 的超级用户。
func (c *Context) IsSuperUser() bool {
	if c == nil || c.app == nil {
		return false
	}
	return c.app.IsSuperUser(c.UserID())
}
