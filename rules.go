package anybot

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Any 匹配所有事件，常用于兜底路由或测试。
func Any() Rule {
	return RuleFunc(func(context.Context, *Context) (Match, bool) {
		return Match{Score: 1, Reason: "any"}, true
	})
}

// All 要求传入的所有规则都匹配，并合并它们的匹配结果。
func All(rules ...Rule) Rule {
	return RuleFunc(func(ctx context.Context, c *Context) (Match, bool) {
		var merged Match
		for _, rule := range rules {
			if rule == nil {
				continue
			}
			match, ok := rule.Match(ctx, c)
			if !ok {
				return Match{}, false
			}
			merged = mergeMatch(merged, match)
		}
		if merged.Reason == "" {
			merged.Reason = "all"
		}
		return merged, true
	})
}

// AnyOf 在任意一个规则匹配时通过，并返回第一个成功规则的匹配结果。
func AnyOf(rules ...Rule) Rule {
	return RuleFunc(func(ctx context.Context, c *Context) (Match, bool) {
		for _, rule := range rules {
			if rule == nil {
				continue
			}
			match, ok := rule.Match(ctx, c)
			if ok {
				if match.Reason == "" {
					match.Reason = "any_of"
				}
				return match, true
			}
		}
		return Match{}, false
	})
}

// Not 在传入规则不匹配时通过。
func Not(rule Rule) Rule {
	return RuleFunc(func(ctx context.Context, c *Context) (Match, bool) {
		if rule == nil {
			return Match{Score: 1, Reason: "not"}, true
		}
		_, ok := rule.Match(ctx, c)
		return Match{Score: 1, Reason: "not"}, !ok
	})
}

// EventType 匹配标准化事件类型，例如 message、notice 或 request。
func EventType(kind string) Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		return Match{Score: 1, Reason: "type:" + kind}, c.Event() != nil && c.Event().Type == kind
	})
}

// DetailType 匹配标准化事件细分类型，例如 group、private 或心跳事件。
func DetailType(kind string) Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		return Match{Score: 1, Reason: "detail:" + kind}, c.Event() != nil && c.Event().DetailType == kind
	})
}

// MessageEvent 匹配标准化消息事件。
func MessageEvent() Rule {
	return EventType("message")
}

// Group 匹配群消息。
func Group() Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		return Match{Score: 1, Reason: "group"}, event != nil && event.Type == "message" && event.GroupID != ""
	})
}

// Private 匹配私聊消息。
func Private() Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		return Match{Score: 1, Reason: "private"}, event != nil && event.Type == "message" && event.UserID != "" && event.GroupID == ""
	})
}

// FromUser 匹配指定发送者 ID。
func FromUser(ids ...any) Rule {
	want := stringSet(ids...)
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		return Match{Score: 1, Reason: "from_user"}, event != nil && slices.Contains(want, event.UserID)
	})
}

// FromSelf 匹配由当前机器人账号自己发送的消息。
func FromSelf() Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		if event == nil || event.SelfID == "" || event.UserID == "" {
			return Match{}, false
		}
		return Match{Score: 1, Reason: "from_self"}, event.SelfID == event.UserID
	})
}

// NotFromSelf 排除当前机器人账号自己发送的消息。
func NotFromSelf() Rule {
	return Not(FromSelf())
}

// InGroup 匹配指定群 ID。
func InGroup(ids ...any) Rule {
	want := stringSet(ids...)
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		return Match{Score: 1, Reason: "in_group"}, event != nil && slices.Contains(want, event.GroupID)
	})
}

// Mentioned 匹配提及指定用户的消息；未传 ID 时匹配任意提及。
func Mentioned(ids ...any) Rule {
	want := stringSet(ids...)
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		if event == nil || event.Type != "message" {
			return Match{}, false
		}
		for _, seg := range event.Message {
			if seg.Type != "at" {
				continue
			}
			id := fmt.Sprint(seg.Data["id"])
			if len(want) == 0 || slices.Contains(want, id) {
				return Match{Score: 2, Reason: "mentioned", Vars: map[string]any{"mentioned": id}}, true
			}
		}
		return Match{}, false
	})
}

// ToMe 匹配私聊消息，或群聊中提及当前机器人的消息。
func ToMe() Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		event := c.Event()
		if event == nil || event.Type != "message" {
			return Match{}, false
		}
		if event.GroupID == "" && event.UserID != "" {
			return Match{Score: 2, Reason: "to_me"}, true
		}
		if event.SelfID == "" {
			return Match{}, false
		}
		match, ok := Mentioned(event.SelfID).Match(c.Context, c)
		if !ok {
			return Match{}, false
		}
		match.Reason = "to_me"
		return match, true
	})
}

// Contains 匹配文本中包含指定片段的消息。
func Contains(substr string) Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		if substr == "" {
			return Match{}, false
		}
		return Match{Score: 1, Reason: "contains:" + substr}, strings.Contains(c.Text(), substr)
	})
}

// Prefix 匹配指定文本前缀，并把剩余文本写入 rest 变量。
func Prefix(prefix string) Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		text := strings.TrimSpace(c.Text())
		if prefix == "" || !strings.HasPrefix(text, prefix) {
			return Match{}, false
		}
		rest := strings.TrimSpace(strings.TrimPrefix(text, prefix))
		return Match{Score: 2, Reason: "prefix:" + prefix, Vars: map[string]any{
			"prefix": prefix,
			"rest":   rest,
		}}, true
	})
}

// CommandRule 使用默认前缀 /、!、. 匹配命令。
func CommandRule(names ...string) Rule {
	return CommandWithPrefixes([]string{"/", "!", "."}, names...)
}

// CommandWithPrefixes 使用显式前缀集合匹配命令，并写入 prefix、command、args 和 argv。
func CommandWithPrefixes(prefixes []string, names ...string) Rule {
	nameSet := map[string]struct{}{}
	for _, name := range names {
		if name != "" {
			nameSet[strings.ToLower(name)] = struct{}{}
		}
	}
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		text := strings.TrimSpace(c.Text())
		if text == "" {
			return Match{}, false
		}
		var usedPrefix string
		for _, prefix := range prefixes {
			if strings.HasPrefix(text, prefix) {
				usedPrefix = prefix
				text = strings.TrimSpace(strings.TrimPrefix(text, prefix))
				break
			}
		}
		if usedPrefix == "" {
			return Match{}, false
		}
		name, args := cutCommand(text)
		name = strings.ToLower(name)
		if len(nameSet) > 0 {
			if _, ok := nameSet[name]; !ok {
				return Match{}, false
			}
		}
		return Match{Score: 3, Reason: "command:" + name, Vars: map[string]any{
			"prefix":  usedPrefix,
			"command": name,
			"args":    strings.TrimSpace(args),
			"argv":    splitArgv(args),
		}}, true
	})
}

// RegexRule 使用正则表达式字符串匹配事件文本。
func RegexRule(pattern string) Rule {
	return RegexpRule(regexp.MustCompile(pattern))
}

// RegexpRule 使用已编译的正则表达式匹配事件文本，并写入 matches 与命名分组变量。
func RegexpRule(re *regexp.Regexp) Rule {
	return RuleFunc(func(_ context.Context, c *Context) (Match, bool) {
		if re == nil {
			return Match{}, false
		}
		matches := re.FindStringSubmatch(c.Text())
		if matches == nil {
			return Match{}, false
		}
		vars := map[string]any{"matches": matches}
		for i, name := range re.SubexpNames() {
			if i > 0 && name != "" && i < len(matches) {
				vars[name] = matches[i]
			}
		}
		return Match{Score: 2, Reason: "regex:" + re.String(), Vars: vars}, true
	})
}

func stringSet(values ...any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprint(value))
	}
	return out
}

func mergeMatch(left, right Match) Match {
	left.Score += right.Score
	if right.Reason != "" {
		if left.Reason != "" {
			left.Reason += "; "
		}
		left.Reason += right.Reason
	}
	for key, value := range right.Vars {
		if left.Vars == nil {
			left.Vars = map[string]any{}
		}
		left.Vars[key] = value
	}
	return left
}

func cutCommand(text string) (string, string) {
	index := strings.IndexFunc(text, unicode.IsSpace)
	if index < 0 {
		return text, ""
	}
	return text[:index], strings.TrimSpace(text[index:])
}

func splitArgv(input string) []string {
	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	seen := false
	flush := func() {
		if seen {
			out = append(out, b.String())
			b.Reset()
			seen = false
		}
	}
	for _, r := range strings.TrimSpace(input) {
		switch {
		case escaped:
			b.WriteRune(r)
			seen = true
			escaped = false
		case r == '\\':
			escaped = true
			seen = true
		case quote != 0:
			if r == quote {
				quote = 0
				seen = true
				continue
			}
			b.WriteRune(r)
			seen = true
		case r == '\'' || r == '"':
			quote = r
			seen = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			b.WriteRune(r)
			seen = true
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	flush()
	return out
}
