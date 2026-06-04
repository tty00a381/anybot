// admin 包提供一个超级用户命令示例插件。
package admin

import (
	"strings"

	"github.com/tty00a381/anybot"
)

type plugin struct {
	users []any
}

// New 创建管理员示例插件，仅允许传入的用户 ID 执行 /admin。
func New(users ...any) anybot.Plugin {
	return &plugin{users: users}
}

func (p *plugin) Manifest() anybot.Manifest {
	return anybot.Manifest{Name: "admin", Version: "1.0.0", Description: "超级用户命令示例"}
}

func (p *plugin) Install(app *anybot.App) error {
	app.Command("admin").Use(anybot.SuperUser(p.users...)).Handle(func(c *anybot.Context) error {
		args := strings.TrimSpace(c.Args())
		if args == "" {
			args = "权限正常"
		}
		_, err := c.ReplyText(args)
		return err
	})
	return nil
}
