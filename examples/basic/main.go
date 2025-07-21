package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/maxbolgarin/bote"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatalln("TELEGRAM_BOT_TOKEN is not set")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	b, err := bote.New(token)
	if err != nil {
		log.Fatalln(err)
	}

	stopCh := b.Start(ctx, startHandler, nil)
	<-stopCh
}

func startHandler(ctx bote.Context) error {
	kb := bote.InlineBuilder(3, bote.OneBytePerRune,
		ctx.Btn("1", oneNumberHandler),
		ctx.Btn("2", twoNumbersHandler),
		ctx.Btn("3", nil),
		ctx.Btn("4", nil),
		ctx.Btn("some text for a long inline button", nil),
		ctx.Btn("use Bote to build bots", nil),
	)
	return ctx.SendMain(bote.NoChange, "Main message", kb)
}

func oneNumberHandler(ctx bote.Context) error {
	return ctx.SendMain(bote.NoChange, "One number", nil)
}

func twoNumbersHandler(ctx bote.Context) error {
	return ctx.SendMain(bote.NoChange, "Two numbers", nil)
}
