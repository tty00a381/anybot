package dialogue

import (
	"fmt"
	"strings"
	"time"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	ProfileCommand string `yaml:"profile_command"`
	SummaryCommand string `yaml:"summary_command"`
	TTL            string `yaml:"ttl"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.ProfileCommand) == "" {
		return fmt.Errorf("profile_command is required")
	}
	if strings.TrimSpace(cfg.SummaryCommand) == "" {
		return fmt.Errorf("summary_command is required")
	}
	if _, err := cfg.ttl(); err != nil {
		return err
	}
	return nil
}

func (cfg Config) ttl() (time.Duration, error) {
	ttl, err := time.ParseDuration(strings.TrimSpace(cfg.TTL))
	if err != nil {
		return 0, fmt.Errorf("ttl is invalid: %w", err)
	}
	if ttl <= 0 {
		return 0, fmt.Errorf("ttl must be positive")
	}
	return ttl, nil
}

type profile struct {
	Name string `json:"name"`
	City string `json:"city"`
}

type draftProfile struct {
	Name string `json:"name"`
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest: absdk.Manifest{Name: "dialogue_example", Version: "0.1.0", Description: "多轮对话插件示例"},
	DefaultConfig: Config{
		ProfileCommand: "profile",
		SummaryCommand: "whoami",
		TTL:            "10m",
	},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		ttl, err := cfg.ttl()
		if err != nil {
			return err
		}
		flow := ctx.Dialogue(
			"profile",
			absdk.DialogueWithScope(absdk.DialogueScopeUser),
			absdk.DialogueWithTTL(ttl),
		)
		flow.Step("name", func(turn *absdk.DialogueTurn) error {
			name := strings.TrimSpace(turn.Text())
			if name == "" {
				_, err := turn.ReplyText("昵称不能为空，请重新输入。")
				return err
			}
			return turn.NextText("city", draftProfile{Name: name}, "你常在哪个城市出没？")
		})
		flow.Step("city", func(turn *absdk.DialogueTurn) error {
			city := strings.TrimSpace(turn.Text())
			if city == "" {
				_, err := turn.ReplyText("城市不能为空，请重新输入。")
				return err
			}
			var draft draftProfile
			if err := turn.Load(&draft); err != nil {
				return err
			}
			value := profile{Name: draft.Name, City: city}
			if err := absdk.UserState[profile](ctx, turn.EventContext, "profile").Save(value, 0); err != nil {
				return err
			}
			return turn.EndText(fmt.Sprintf("已记住：%s，%s。", value.Name, value.City))
		})

		ctx.Command(cfg.ProfileCommand).
			Name("profile.begin").
			Handle(func(c *absdk.EventContext) error {
				return flow.BeginText(c, "name", nil, "你想让我怎么称呼你？")
			})

		ctx.Command(cfg.SummaryCommand).
			Name("profile.summary").
			Handle(func(c *absdk.EventContext) error {
				value, ok, err := absdk.UserState[profile](ctx, c, "profile").Load()
				if err != nil {
					return err
				}
				if !ok {
					_, err := c.ReplyText("我还不认识你。先发送 /" + cfg.ProfileCommand + " 建档。")
					return err
				}
				_, err = c.ReplyText(fmt.Sprintf("你是 %s，常在 %s。", value.Name, value.City))
				return err
			})

		return nil
	},
})
