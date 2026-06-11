// admin 包提供一个超级用户命令插件。
package admin

import (
	"strings"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Users []string `yaml:"users"`
}

var Plugin = absdk.Define(absdk.Spec[Config]{
	Manifest:      absdk.Manifest{Name: "admin", Version: "1.0.1", Description: "超级用户命令"},
	DefaultConfig: Config{},
	Setup: func(ctx *absdk.Context, cfg Config) error {
		route := ctx.Command("admin").Name("command")
		if len(cfg.Users) > 0 {
			users := make([]any, len(cfg.Users))
			for i, user := range cfg.Users {
				users[i] = user
			}
			route.Use(absdk.SuperUser(users...))
		} else {
			route.Use(absdk.RequireSuperUser())
		}
		route.Handle(func(c *absdk.EventContext) error {
			args := strings.TrimSpace(c.Args())
			if args == "" {
				args = "权限正常"
			}
			_, err := c.ReplyText(args)
			return err
		})
		return nil
	},
})
