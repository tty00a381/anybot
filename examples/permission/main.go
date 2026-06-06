package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := core.New(core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	app.Use(core.Recover(), core.Trace(), core.RateLimit(5, time.Minute))

	app.Command("group").Use(core.OnlyGroup()).Handle(func(c *core.Context) error {
		_, err := c.ReplyText("这是一条群聊命令")
		return err
	})

	app.Command("admin").Use(core.SuperUser(os.Getenv("SUPER_USER_ID"))).Handle(func(c *core.Context) error {
		_, err := c.ReplyText("权限正常")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
