// ratelimit 包提供框架级会话限速插件。
package ratelimit

import (
	"fmt"
	"time"

	absdk "github.com/tty00a381/anybot/sdk"
)

type Config struct {
	Limit  int    `yaml:"limit"`
	Window string `yaml:"window"`
}

func (cfg Config) Validate() error {
	if cfg.Limit <= 0 {
		return fmt.Errorf("ratelimit.limit 必须大于 0")
	}
	if _, err := cfg.window(); err != nil {
		return err
	}
	return nil
}

func (cfg Config) window() (time.Duration, error) {
	window, err := time.ParseDuration(cfg.Window)
	if err != nil || window <= 0 {
		return 0, fmt.Errorf("ratelimit.window 必须是正数时长")
	}
	return window, nil
}

var Module = absdk.Define(
	absdk.Manifest{Name: "ratelimit", Version: "1.0.0", Description: "会话限速中间件"},
	Config{Limit: 5, Window: "1m"},
	func(ctx *absdk.Context, cfg Config) error {
		window, err := cfg.window()
		if err != nil {
			return err
		}
		ctx.UseGlobal(absdk.RateLimit(cfg.Limit, window))
		return nil
	},
)
