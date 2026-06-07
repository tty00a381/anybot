package main

import (
	"context"
	"log"
	"log/slog"
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

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	app := core.New(
		core.WithAdapter(onebot11.ReverseWS("127.0.0.1:6700",
			onebot11.WithAccessToken(os.Getenv("ONEBOT_ACCESS_TOKEN")),
		)),
		core.WithLogger(logger),
	)
	app.Use(core.Recover(logger), core.Trace(logger))

	app.OnAdapterState(func(_ context.Context, state core.AdapterState) {
		logger.Info("动作通道状态变化", "kind", state.Kind, "ready", state.Ready(), "transport", state.Transport)
	})

	app.Observe(core.MessageEvent()).Name("audit").Handle(func(_ context.Context, event *core.Event) error {
		logger.Info("观察消息", "user_id", event.UserID, "group_id", event.GroupID, "text", event.Text)
		return nil
	})

	targetUser := os.Getenv("PROACTIVE_USER_ID")
	app.Go("startup-message", func(taskCtx context.Context) error {
		if targetUser == "" {
			logger.Info("未设置 PROACTIVE_USER_ID，跳过主动消息示例")
			return nil
		}
		if err := app.WaitActionReady(taskCtx); err != nil {
			return err
		}
		_, err := app.Client().Send(taskCtx, core.ReplyTarget{
			Protocol: onebot11.Protocol,
			UserID:   targetUser,
		}, message.New(message.Text("AnyBot 已连接，主动消息通道可用。")))
		return err
	})

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
