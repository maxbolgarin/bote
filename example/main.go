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
		DefaultLanguageCode: "ru",
		NoPreview:           true,

		// --- Webhook Configuration Example ---
		// To use webhooks, uncomment and configure the following.
		// The WebhookURL must be accessible from the internet and Telegram servers.
		// ListenAddress is where the bot will listen for incoming updates from Telegram.
		//
		// If you're using a reverse proxy (e.g., Nginx) to handle HTTPS termination,
		// you might not need to set TLSKeyFile and TLSCertFile here.
		// The proxy would handle HTTPS and forward plain HTTP to your bot's ListenAddress.

		// Example 1: Webhook without direct TLS handling by the bot (e.g., behind a reverse proxy)
		// WebhookURL:    "https://your.domain.com/webhook-path", // Replace with your actual public URL
		// ListenAddress: "0.0.0.0:8080",                       // Bot listens on port 8080 locally

		// Example 2: Webhook with direct TLS handling by the bot
		// WebhookURL:    "https://your.domain.com:8443/webhook-path", // Port in URL must match ListenAddress
		// ListenAddress: "0.0.0.0:8443",
		// TLSKeyFile:    "/path/to/your/private.key", // Replace with actual path
		// TLSCertFile:   "/path/to/your/public.crt",  // Replace with actual path
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
