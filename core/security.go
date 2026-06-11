package core

import (
	"fmt"
	"slices"
	"strings"
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

// HasPermission 判断当前事件是否满足一项标准权限标识。
//
// 支持的基础标识包括 all、user、private、group、group.member、
// group.manager、group.owner 与 superuser。group.manager 包含群主。
func (c *Context) HasPermission(permission string) bool {
	permission = strings.ToLower(strings.TrimSpace(permission))
	switch permission {
	case "", "none":
		return false
	case "all":
		return true
	case "user":
		return c != nil && c.UserID() != ""
	case "private":
		return c != nil && c.IsPrivate()
	case "group":
		return c != nil && c.IsGroup()
	case "group.member":
		return c != nil && c.IsGroup() && c.UserID() != ""
	case "group.manager":
		if c == nil || !c.IsGroup() {
			return false
		}
		role := c.GroupRole()
		return role == "manager" || role == "owner"
	case "group.owner":
		return c != nil && c.IsGroup() && c.GroupRole() == "owner"
	case "superuser":
		return c != nil && c.IsSuperUser()
	default:
		return false
	}
}

// HasAnyPermission 判断当前事件是否满足任一标准权限标识。
func (c *Context) HasAnyPermission(permissions ...string) bool {
	for _, permission := range permissions {
		if c.HasPermission(permission) {
			return true
		}
	}
	return false
}
