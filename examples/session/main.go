package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := core.New(core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	app.Use(core.Recover(), core.Trace())

	app.Command("count").Handle(func(c *core.Context) error {
		session := c.Session()
		var count int
		_, _ = session.LoadJSON(c.Context, "count", &count)
		count++
		if err := session.SaveJSON(c.Context, "count", count, 24*time.Hour); err != nil {
			return err
		}
		_, err := c.ReplyText("当前会话计数：" + strconv.Itoa(count))
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
