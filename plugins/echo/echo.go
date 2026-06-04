// echo 包提供一个复读命令示例插件。
package echo

import (
	"strings"

	"github.com/tty00a381/anybot"
)

type plugin struct {
	command string
}

// Option 调整复读插件配置。
type Option func(*plugin)

// WithCommand 配置复读命令名。
func WithCommand(command string) Option {
	return func(p *plugin) {
		if command != "" {
			p.command = command
		}
	}
}

// New 创建复读插件，默认注册 /echo。
func New(opts ...Option) anybot.Plugin {
	p := &plugin{command: "echo"}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *plugin) Manifest() anybot.Manifest {
	return anybot.Manifest{Name: "echo", Version: "1.0.0", Description: "复读命令示例"}
}

func (p *plugin) Install(app *anybot.App) error {
	app.Command(p.command).Handle(func(c *anybot.Context) error {
		text := strings.TrimSpace(c.Args())
		if text == "" {
			text = "请输入要复读的内容"
		}
		_, err := c.ReplyText(text)
		return err
	})
	return nil
}
