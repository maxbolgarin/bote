package bote

import (
	"context"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"
)

// mockPoller implements tele.Poller; it sends a minimal update then stops.
type mockPoller struct{ sent bool }

func (m *mockPoller) Poll(bot *tele.Bot, updates chan tele.Update, stop chan struct{}) {
	if m.sent {
		return
	}
	m.sent = true
	// emit one update then wait for stop signal from outer code
	select {
	case updates <- tele.Update{Message: &tele.Message{Text: "/start", Sender: &tele.User{ID: 1}}}:
	case <-time.After(100 * time.Millisecond):
	}
	select {
	case <-stop:
		return
	case <-time.After(100 * time.Millisecond):
		return
	}
}

func TestBotLifecycleWithMockPoller(t *testing.T) {
	opts := Options{}
	opts.Config.Mode = PollingModeCustom
	opts.Poller = &mockPoller{}
	opts.Config.LongPolling.Timeout = 50 * time.Millisecond
	opts.Config.Log.Enable = boolPtr(false)
	opts.Config.Log.LogUpdates = boolPtr(false)
	opts.Config.Bot.DeleteMessages = boolPtr(false)
	opts.Offline = true

	b, err := NewWithOptions("dummy-token", opts)
	if err != nil {
		t.Fatalf("NewWithOptions error: %v", err)
	}

	// Simple start handler that does nothing
	start := func(c Context) error { return nil }

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	st := b.Start(ctx, start, nil)

	select {
	case <-st:
		// stopped as expected
	case <-time.After(2 * time.Second):
		t.Fatal("bot did not stop in time")
	}
}

func boolPtr(v bool) *bool { return &v }

