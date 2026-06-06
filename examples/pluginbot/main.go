package main

import (
	"context"
	"log"

	"github.com/tty00a381/anybot/adapters/onebot11"
	"github.com/tty00a381/anybot/core"
)

type helloPlugin struct{}

func (helloPlugin) Manifest() core.Manifest {
	return core.Manifest{Name: "hello", Version: "0.1.0"}
}

func (helloPlugin) Install(app *core.App) error {
	app.Command("hello").Handle(func(c *core.Context) error {
		_, err := c.ReplyText("world")
		return err
	})
	return nil
}

func main() {
	app := core.New(core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700")))
	if err := app.UsePlugin(helloPlugin{}); err != nil {
		log.Fatal(err)
	}
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
