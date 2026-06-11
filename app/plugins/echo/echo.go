// echo 包提供一个复读命令插件。
package echo

import (
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command string `yaml:"command"`
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest:      absdk.Manifest{Name: "echo", Version: "1.0.2", Description: "复读命令"},
	DefaultConfig: Config{Command: "echo"},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).Name("command").Handle(func(c *absdk.EventContext) error {
			text := strings.TrimSpace(c.Args())
			if text == "" {
				text = "请输入要复读的内容"
			}
			_, err := c.ReplyText(text)
			return err
		})
		return nil
	},
})
