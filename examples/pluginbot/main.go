package main

import (
	"context"
	"log"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
	absdk "github.com/tty00a381/anybot/sdk"
)

var helloModule = absdk.Define(
	absdk.Manifest{Name: "hello", Version: "0.1.0"},
	struct{}{},
	func(ctx *absdk.Context, _ struct{}) error {
		ctx.Command("hello").Handle(func(c *absdk.EventContext) error {
			_, err := c.ReplyText("world")
			return err
		})
		return nil
	},
)

func installRoutes(app *core.App) {
	app.Command("ping").Handle(func(c *core.Context) error {
		_, err := c.ReplyText("pong")
		return err
	})
}

func main() {
	app := core.New(core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	installRoutes(app)
	if err := absdk.Install(app, helloModule); err != nil {
		log.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
