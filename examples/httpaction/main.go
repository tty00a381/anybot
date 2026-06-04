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

	app := anybot.New(
		anybot.WithAdapter(onebot11.HTTP("http://127.0.0.1:3000", "127.0.0.1:6701",
			onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
		)),
	)
	app.Use(anybot.Recover(), anybot.Trace())

	app.Command("me").Handle(func(c *anybot.Context) error {
		client := onebot11.MustClient(c)
		info, err := client.GetLoginInfo(c.Context)
		if err != nil {
			return err
		}
		_, err = c.ReplyText(info.Nickname)
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
