package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot"
	"github.com/tty00a381/anybot/adapters/onebot11"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := anybot.New(anybot.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	app.Use(anybot.Recover(), anybot.Trace())
	app.Command("ping").Handle(func(c *anybot.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
