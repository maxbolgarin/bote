package main

import (
	"log/slog"

	"github.com/maxbolgarin/contem"
)

type App struct {
}

func main() {
	contem.Start(run, slog.Default())
}

func run(ctx contem.Context) error {

	// b, err := bote.Start[App](ctx, cfg)
	// if err != nil {
	// 	panic(err)
	// }

	return nil
}
