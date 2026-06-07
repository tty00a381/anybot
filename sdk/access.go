package sdk

import (
	"context"
	"slices"
	"strings"
)

// AllowedGroups 按配置中的群 ID 列表限制群聊事件；列表为空时不限制。
func AllowedGroups(ids ...string) Rule {
	allowed := cleanStringSet(ids...)
	return RuleFunc(func(_ context.Context, c *EventContext) (Match, bool) {
		if len(allowed) == 0 {
			return Match{Score: 1, Reason: "allowed_groups:all"}, true
		}
		if c == nil || c.GroupID() == "" {
			return Match{}, false
		}
		return Match{Score: 1, Reason: "allowed_groups"}, slices.Contains(allowed, c.GroupID())
	})
}

// RequireAdmin 使用插件配置中的管理员列表鉴权；列表为空时回退到框架超级用户。
func RequireAdmin(ids ...string) Middleware {
	allowed := cleanStringSet(ids...)
	if len(allowed) == 0 {
		return RequireSuperUser()
	}
	return SuperUser(stringsToAny(allowed)...)
}

func cleanStringSet(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(out, value) {
			continue
		}
		out = append(out, value)
	}
	return out
}

func stringsToAny(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}
