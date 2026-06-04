package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := anybot.New(anybot.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	app.Use(anybot.Recover(), anybot.Trace(), anybot.RateLimit(5, time.Minute))

	app.Command("group").Use(anybot.OnlyGroup()).Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("这是一条群聊命令")
		return err
	})

	app.Command("admin").Use(anybot.SuperUser(os.Getenv("SUPER_USER_ID"))).Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("权限正常")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
