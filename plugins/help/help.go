// help 包提供一个可配置的基础帮助命令插件。
package help

import (
	"strings"

	"github.com/tty00a381/anybot"
)

// Option 调整帮助插件的命令名或输出内容。
type Option func(*plugin)

type plugin struct {
	command string
	lines   []string
}

// WithCommand 配置帮助命令名。
func WithCommand(command string) Option {
	return func(p *plugin) {
		if command != "" {
			p.command = command
		}
	}
}

// WithLine 追加一行帮助文本。
func WithLine(line string) Option {
	return func(p *plugin) {
		if line != "" {
			p.lines = append(p.lines, line)
		}
	}
}

// New 创建帮助插件，默认注册 /help。
func New(opts ...Option) anybot.Plugin {
	p := &plugin{
		command: "help",
		lines:   []string{"可用命令：", "/help 显示帮助", "/ping 检查机器人是否在线"},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *plugin) Manifest() anybot.Manifest {
	return anybot.Manifest{Name: "help", Version: "1.0.0", Description: "基础帮助命令"}
}

func (p *plugin) Install(app *anybot.App) error {
	app.Command(p.command).Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText(strings.Join(p.lines, "\n"))
		return err
	})
	return nil
}
