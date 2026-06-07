package groupmemo

import (
	"fmt"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	ViewCommand   string   `yaml:"view_command"`
	SetCommand    string   `yaml:"set_command"`
	AllowedGroups []string `yaml:"allowed_groups"`
	Admins        []string `yaml:"admins"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.ViewCommand) == "" {
		return fmt.Errorf("view_command is required")
	}
	if strings.TrimSpace(cfg.SetCommand) == "" {
		return fmt.Errorf("set_command is required")
	}
	return nil
}

type memoState struct {
	Text string `json:"text"`
}

var Plugin = absdk.Define(
	absdk.Manifest{Name: "groupmemo_example", Version: "0.1.0", Description: "群便签插件示例"},
	Config{
		ViewCommand:   "memo",
		SetCommand:    "remember",
		AllowedGroups: nil,
		Admins:        nil,
	},
	func(ctx *absdk.Context, cfg Config) error {
		groupRules := []absdk.Rule{absdk.Group(), absdk.AllowedGroups(cfg.AllowedGroups...)}

		ctx.OnMessage(append(groupRules, absdk.CommandRule(cfg.ViewCommand))...).
			Name("memo.view").
			Handle(func(c *absdk.EventContext) error {
				state, err := absdk.GroupState[memoState](ctx, c, "memo").LoadOr(memoState{})
				if err != nil {
					return err
				}
				text := strings.TrimSpace(state.Text)
				if text == "" {
					text = "这个群还没有便签。"
				}
				_, err = c.ReplyText(text)
				return err
			})

		ctx.OnMessage(append(groupRules, absdk.CommandRule(cfg.SetCommand))...).
			Name("memo.set").
			Use(absdk.RequireAdmin(cfg.Admins...)).
			Handle(func(c *absdk.EventContext) error {
				text := strings.TrimSpace(c.Args())
				if text == "" {
					_, err := c.ReplyText("用法：/" + cfg.SetCommand + " <便签内容>")
					return err
				}
				if err := absdk.GroupState[memoState](ctx, c, "memo").Save(memoState{Text: text}, 0); err != nil {
					return err
				}
				_, err := c.ReplyText("群便签已更新。")
				return err
			})

		return nil
	},
)
