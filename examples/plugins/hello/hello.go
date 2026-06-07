package hello

import (
	"fmt"
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Command  string `yaml:"command"`
	Greeting string `yaml:"greeting"`
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.Command) == "" {
		return fmt.Errorf("command is required")
	}
	if strings.TrimSpace(cfg.Greeting) == "" {
		return fmt.Errorf("greeting is required")
	}
	return nil
}

var Plugin = absdk.Define(
	absdk.Manifest{Name: "hello_example", Version: "0.1.0", Description: "最小命令插件示例"},
	Config{
		Command:  "hello",
		Greeting: "你好，我是 AnyBot 插件。",
	},
	func(ctx *absdk.Context, cfg Config) error {
		ctx.Command(cfg.Command).
			Name("command").
			Handle(func(c *absdk.EventContext) error {
				_, err := c.ReplyText(cfg.Greeting)
				return err
			})
		return nil
	},
)
