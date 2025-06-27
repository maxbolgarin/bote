package main

import (
	"os"
	"os/signal"

	"github.com/maxbolgarin/bote"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		panic("TELEGRAM_BOT_TOKEN is not set")
	}

	cfg := bote.Config{
		DefaultLanguage: bote.LanguageRussian,
		NoPreview:       true,
	}

	b, err := bote.New(token, bote.WithConfig(cfg))
	if err != nil {
		panic(err)
	}

	b.Start(startHandler, nil)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	<-ch

	b.Stop()
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
