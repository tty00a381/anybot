// help 包提供一个可配置的基础帮助命令插件。
package help

import (
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command string   `yaml:"command"`
	Lines   []string `yaml:"lines"`
}

var Plugin = absdk.Define(
	absdk.Manifest{Name: "help", Version: "1.0.0", Description: "基础帮助命令"},
	Config{
		Command: "help",
		Lines:   []string{"可用命令：", "/help 显示帮助", "/ping 检查机器人是否在线"},
	},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Name("command").Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText(strings.Join(cfg.Lines, "\n"))
			return err
		})
		return nil
	},
)
