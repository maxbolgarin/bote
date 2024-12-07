package main

import (
	"log/slog"
	"os"

	"github.com/maxbolgarin/bote"
	"github.com/maxbolgarin/contem"
	"github.com/maxbolgarin/errm"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)
	contem.Start(run, slog.Default())
}

var isDebug = false

func run(ctx contem.Context) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return errm.New("TELEGRAM_BOT_TOKEN is not set")
	}

	b, err := bote.Start(ctx, token, bote.Options{Config: bote.Config{Debug: isDebug}})
	if err != nil {
		return err
	}

	b.Handle("/start", func(ctx bote.Context) error {
		slog.Info("start")
		return nil
	})

	return nil
}
