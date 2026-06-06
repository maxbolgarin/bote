// Command service is a minimal "service bot" example: it has no per-user conversation. It posts a
// draft to an admin chat with Approve / Reject buttons, and on approve republishes the draft to a
// public channel with a "View source" link button. It demonstrates the stateless callback router
// (RegisterButton / NewButton), the channel send/edit helpers, and URL buttons.
//
// The bot must be an admin with post rights in BOTH the admin chat and the public channel.
//
// Usage:
//
//	TELEGRAM_BOT_TOKEN=... ADMIN_CHAT_ID=-100... CHANNEL_ID=-100... go run ./examples/service
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"

	"github.com/maxbolgarin/bote"
)

// In a real bot drafts live in a database; here a tiny in-memory store keyed by draft id.
var (
	mu     sync.Mutex
	drafts = map[string]string{
		"1": "<b>Did you know?</b> Bote can run channel/admin bots with no per-user state. ✨",
	}
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatalln("TELEGRAM_BOT_TOKEN is not set")
	}
	adminChatID, err := strconv.ParseInt(os.Getenv("ADMIN_CHAT_ID"), 10, 64)
	if err != nil {
		log.Fatalln("ADMIN_CHAT_ID must be a numeric chat id (e.g. -100...)")
	}
	channelID, err := strconv.ParseInt(os.Getenv("CHANNEL_ID"), 10, 64)
	if err != nil {
		log.Fatalln("CHANNEL_ID must be a numeric channel id (e.g. -100...)")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	b, err := bote.New(ctx, token)
	if err != nil {
		log.Fatalln(err)
	}

	// Register the two actions ONCE. The draft id arrives as the button payload (ctx.Data()).
	b.RegisterButton("approve", func(ctx bote.Context) error {
		id := ctx.Data()
		mu.Lock()
		text := drafts[id]
		mu.Unlock()
		if text == "" {
			return nil
		}

		// Publish to the public channel with a URL ("View source") button.
		kb := bote.NewKeyboard()
		kb.AddURL("View source", "https://example.com/posts/"+id)
		if _, err := ctx.SendInChat(channelID, 0, text, kb.CreateInlineMarkup()); err != nil {
			return err
		}
		// Strip the buttons on the admin message to mark it handled.
		return ctx.EditInChat(ctx.ChatID(), ctx.MessageID(), "✅ approved and published", bote.EmptyKeyboard)
	})

	b.RegisterButton("reject", func(ctx bote.Context) error {
		return ctx.EditInChat(ctx.ChatID(), ctx.MessageID(), "✖ rejected", bote.EmptyKeyboard)
	})

	// Push the draft to the admin chat for review. Build the buttons with NewButton, carrying the
	// draft id as payload — no Context needed.
	mu.Lock()
	draft := drafts["1"]
	mu.Unlock()
	kb := bote.NewKeyboard()
	kb.AddRow(b.NewButton("approve", "1"), b.NewButton("reject", "1"))
	if _, err := b.SendInChat(adminChatID, 0, draft, kb.CreateInlineMarkup()); err != nil {
		log.Fatalln(err)
	}
	log.Println("draft sent to admin chat; waiting for approval…")

	// A service bot has no /start flow; a no-op start handler is fine.
	stopCh := b.Start(ctx, func(bote.Context) error { return nil }, nil)
	select {
	case <-stopCh:
	case <-ctx.Done():
	}
}
