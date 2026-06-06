package dialogueplugin

import (
	"context"
	"fmt"
	"strings"
	"time"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command     string        `yaml:"command"`
	MemoryTTL   time.Duration `yaml:"memory_ttl"`
	Subscribers []string      `yaml:"subscribers"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("dialogue.command 不能为空")
	}
	if cfg.MemoryTTL < 0 {
		return fmt.Errorf("dialogue.memory_ttl 不能小于 0")
	}
	return nil
}

type profile struct {
	Name string `json:"name"`
	City string `json:"city,omitempty"`
}

type profileDraft struct {
	Name string `json:"name"`
}

var Module = absdk.Define(
	absdk.Manifest{Name: "dialogue", Version: "1.0.0", Description: "SDK-only 对话插件示例"},
	Config{Command: "chat", MemoryTTL: 24 * time.Hour},
	func(ctx *absdk.Context, cfg Config) error {
		profileDialogue := ctx.Dialogue("profile",
			absdk.DialogueWithTTL(cfg.MemoryTTL),
			absdk.DialogueWithRules(absdk.NotFromSelf(), subscribed(cfg)),
		)
		profileDialogue.
			Step("name", func(turn *absdk.DialogueTurn) error {
				name := strings.TrimSpace(turn.Text())
				if isCancel(name) {
					return turn.EndText("已取消")
				}
				if name == "" {
					_, err := turn.ReplyText("名字不能为空。告诉我你的名字，或发送“取消”退出。")
					return err
				}
				return turn.NextText("city", profileDraft{Name: name}, "你在哪个城市？")
			}).
			Step("city", func(turn *absdk.DialogueTurn) error {
				city := strings.TrimSpace(turn.Text())
				if isCancel(city) {
					return turn.EndText("已取消")
				}
				if city == "" {
					_, err := turn.ReplyText("城市不能为空。告诉我你的城市，或发送“取消”退出。")
					return err
				}
				var draft profileDraft
				if err := turn.Load(&draft); err != nil {
					return err
				}
				if err := saveProfile(ctx, turn.EventContext, cfg, profile{Name: draft.Name, City: city}); err != nil {
					return err
				}
				return turn.EndText("记住了，" + draft.Name + "，来自" + city)
			})

		ctx.Command(cfg.Command).
			Name("dialogue.command").
			Handle(func(c *absdk.EventContext) error {
				args := strings.TrimSpace(c.Args())
				switch {
				case args == "" || args == "资料":
					return profileDialogue.BeginText(c, "name", nil, "我们来更新资料。你叫什么名字？")
				case strings.HasPrefix(args, "我叫"):
					return rememberName(ctx, c, cfg)
				default:
					_, err := c.ReplyText("用法：/" + cfg.Command + " 开始资料对话，或 /" + cfg.Command + " 我叫某某")
					return err
				}
			})

		ctx.OnMessage(absdk.All(absdk.ToMe(), absdk.NotFromSelf(), subscribed(cfg))).
			Name("dialogue.chat").
			Use(absdk.RateLimit(20, time.Minute)).
			Handle(func(c *absdk.EventContext) error {
				var p profile
				_, _ = ctx.UserSession(c).LoadJSON(c.Context, "profile", &p)
				_, err := c.ReplyText(displayName(p) + "，我听到了：" + strings.TrimSpace(c.Text()))
				return err
			})

		ctx.Every("memory-sweep", time.Hour, func(taskCtx context.Context) error {
			ctx.Logger().Debug("dialogue memory sweep tick")
			return nil
		})

		return nil
	},
)

func rememberName(ctx *absdk.Context, c *absdk.EventContext, cfg Config) error {
	name := strings.TrimSpace(strings.TrimPrefix(c.Args(), "我叫"))
	if name == "" || name == c.Args() {
		_, err := c.ReplyText("格式：/" + cfg.Command + " 我叫某某")
		return err
	}
	if err := saveProfile(ctx, c, cfg, profile{Name: name}); err != nil {
		return err
	}
	_, err := c.ReplyText("记住了，" + name)
	return err
}

func saveProfile(ctx *absdk.Context, c *absdk.EventContext, cfg Config, p profile) error {
	return ctx.UserSession(c).SaveJSON(c.Context, "profile", p, cfg.MemoryTTL)
}

func displayName(p profile) string {
	if p.Name == "" {
		return "朋友"
	}
	if p.City == "" {
		return p.Name
	}
	return p.Name + "（" + p.City + "）"
}

func isCancel(text string) bool {
	return strings.TrimSpace(text) == "取消"
}

func subscribed(cfg Config) absdk.Rule {
	allowed := map[string]struct{}{}
	for _, id := range cfg.Subscribers {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	return absdk.RuleFunc(func(_ context.Context, c *absdk.EventContext) (absdk.Match, bool) {
		if len(allowed) == 0 {
			return absdk.Match{Reason: "subscribed:all", Score: 1}, true
		}
		_, ok := allowed[c.UserID()]
		return absdk.Match{Reason: "subscribed", Score: 1}, ok
	})
}
