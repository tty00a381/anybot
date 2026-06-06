package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := core.New(
		core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700",
			onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
		)),
	)
	app.Use(core.Recover(), core.Trace())

	app.Command("ping").Handle(func(c *core.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
