// ratelimit 包提供把会话限速中间件打包后的示例插件。
package ratelimit

import (
	"time"

	"github.com/tty00a381/anybot"
)

type plugin struct {
	limit  int
	window time.Duration
}

// New 创建限速插件，并把 RateLimit 注册为全局中间件。
func New(limit int, window time.Duration) anybot.Plugin {
	return &plugin{limit: limit, window: window}
}

func (p *plugin) Manifest() anybot.Manifest {
	return anybot.Manifest{Name: "ratelimit", Version: "1.0.0", Description: "会话限速中间件"}
}

func (p *plugin) Install(app *anybot.App) error {
	app.Use(anybot.RateLimit(p.limit, p.window))
	return nil
}
