package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/maxbolgarin/bote"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		panic("TELEGRAM_BOT_TOKEN is not set")
	}

	cfg := bote.Config{}

	b, err := bote.New(ctx, token, bote.WithConfig(cfg))
	if err != nil {
		panic(err)
	}

	b.SetStartHandler(func(ctx bote.Context) error {
		kb := bote.InlineBuilder(3, bote.OneBytePerRune,
			b.Btn("1", nil),
			b.Btn("2", nil),
			b.Btn("3", nil),
			b.Btn("4", nil),
			b.Btn("some text for a long inline button", nil),
			b.Btn("use Bote to build bots", nil),
		)
		return ctx.SendMain(bote.NoChange, "Main message", kb)
	})

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	b.Stop()
}
