package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
	"github.com/tty00a381/anybot/core/message"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := core.New(core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	app.Use(core.Recover(), core.Trace())

	app.Command("login").Handle(func(c *core.Context) error {
		client := onebot11.MustClient(c)
		info, err := client.GetLoginInfo(c.Context)
		if err != nil {
			return err
		}
		_, err = c.ReplyText("当前账号：" + info.Nickname)
		return err
	})

	app.Command("image").Handle(func(c *core.Context) error {
		file := os.Getenv("IMAGE_FILE")
		if file == "" {
			_, err := c.ReplyText("请先设置 IMAGE_FILE，例如 https://example.com/a.png 或 file:///容器内可见路径/core.png")
			return err
		}
		_, err := c.Reply(message.New(message.Image(file)))
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
