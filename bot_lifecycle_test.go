package bote

import (
	"context"
	"sync"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"
)

// TestBotCreation tests the bot initialization with various configurations
func TestBotCreation(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		opts    func(*Options)
		wantErr bool
	}{
		{
			name:  "valid bot with default options",
			token: "test-token",
			opts: func(o *Options) {
				o.Offline = true
				o.Config.Mode = PollingModeCustom
				o.Poller = &mockPoller{}
			},
			wantErr: false,
		},
		{
			name:    "empty token",
			token:   "",
			opts:    func(o *Options) {},
			wantErr: true,
		},
		{
			name:  "with custom user storage",
			token: "test-token",
			opts: func(o *Options) {
				o.Offline = true
				o.Config.Mode = PollingModeCustom
				o.Poller = &mockPoller{}
				o.UserDB = &mockUserStorage{}
			},
			wantErr: false,
		},
		{
			name:  "with webhook mode offline",
			token: "test-token",
			opts: func(o *Options) {
				o.Offline = true
				o.Config.Mode = PollingModeWebhook
				o.Config.Webhook.URL = "https://example.com/webhook"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts Options
			if tt.opts != nil {
				tt.opts(&opts)
			}

			bot, err := NewWithOptions(t.Context(), tt.token, opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewWithOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if bot != nil && !tt.wantErr {
				// Verify bot was created properly
				if bot.bot == nil || bot.bot.tbot == nil {
					t.Error("bot internal structures not initialized")
				}
				if bot.um == nil {
					t.Error("user manager not initialized")
				}
			}
		})
	}
}

// TestBotStartStop tests the bot lifecycle with start and stop
func TestBotStartStop(t *testing.T) {
	opts := Options{
		Offline: true,
		Config: Config{
			Mode: PollingModeCustom,
			Log: LogConfig{
				Enable:     boolPtr(false),
				LogUpdates: boolPtr(false),
			},
		},
		Poller: &controllablePoller{
			updates: make(chan tele.Update, 10),
		},
	}

	bot, err := NewWithOptions(t.Context(), "test-token", opts)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startHandler := func(c Context) error {
		return c.SendMain(NoChange, "Welcome!", nil)
	}

	// Start the bot
	stopCh := bot.Start(ctx, startHandler, nil)

	// Give the bot time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the bot
	cancel()

	// Wait for graceful shutdown with timeout
	select {
	case <-stopCh:
		// Successfully stopped
	case <-time.After(2 * time.Second):
		t.Fatal("Bot did not stop in time")
	}
}

// TestBotWithMiddleware tests middleware functionality
func TestBotWithMiddleware(t *testing.T) {
	poller := &controllablePoller{
		updates: make(chan tele.Update, 10),
	}

	opts := Options{
		Offline: true,
		Config: Config{
			Mode: PollingModeCustom,
			Log: LogConfig{
				Enable:     boolPtr(false),
				LogUpdates: boolPtr(false),
			},
		},
		Poller: poller,
	}

	bot, err := NewWithOptions(t.Context(), "test-token", opts)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	middlewareCalled := false
	var middlewareUser User
	var middlewareMu sync.Mutex

	// Add middleware
	bot.AddUserMiddleware(func(upd *tele.Update, user User) bool {
		middlewareMu.Lock()
		middlewareCalled = true
		middlewareUser = user
		middlewareMu.Unlock()
		return true // Continue processing
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startHandler := func(c Context) error {
		return nil
	}

	// Add a handler to process the update
	bot.SetTextHandler(func(c Context) error {
		// This will trigger after middleware
		return nil
	})

	stopCh := bot.Start(ctx, startHandler, nil)

	// Wait for bot to be ready
	time.Sleep(300 * time.Millisecond)

	// Send a test update with a regular text message (not a command)
	poller.sendUpdate(tele.Update{
		ID: 1,
		Message: &tele.Message{
			ID:     1,
			Text:   "test message",
			Sender: &tele.User{ID: 123, Username: "testuser"},
			Chat:   &tele.Chat{ID: 123, Type: "private"},
		},
	})

	// Wait for processing with retry logic
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		middlewareMu.Lock()
		called := middlewareCalled
		middlewareMu.Unlock()
		if called {
			break
		}
	}

	middlewareMu.Lock()
	defer middlewareMu.Unlock()

	if !middlewareCalled {
		t.Error("Middleware was not called")
	}

	if middlewareUser == nil || middlewareUser.ID() != 123 {
		t.Error("Middleware received incorrect user")
	}

	cancel()
	<-stopCh
}

// TestBotHandlers tests various handler registrations
func TestBotHandlers(t *testing.T) {
	poller := &controllablePoller{
		updates: make(chan tele.Update, 10),
	}

	opts := Options{
		Offline: true,
		Config: Config{
			Mode: PollingModeCustom,
			Log: LogConfig{
				Enable:     boolPtr(false),
				LogUpdates: boolPtr(false),
			},
			Bot: BotConfig{
				DeleteMessages: boolPtr(false),
			},
		},
		Poller: poller,
	}

	bot, err := NewWithOptions(t.Context(), "test-token", opts)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	textHandlerCalled := false
	callbackHandlerCalled := false
	var handlerMu sync.Mutex

	// Set text handler
	bot.SetTextHandler(func(c Context) error {
		handlerMu.Lock()
		textHandlerCalled = true
		handlerMu.Unlock()
		return nil
	})

	// Handle callback
	bot.Handle(tele.OnCallback, func(c Context) error {
		handlerMu.Lock()
		callbackHandlerCalled = true
		handlerMu.Unlock()
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopCh := bot.Start(ctx, func(c Context) error { return nil }, nil)

	// Wait for bot to be ready
	time.Sleep(300 * time.Millisecond)

	// Send text message
	poller.sendUpdate(tele.Update{
		Message: &tele.Message{
			Text:   "Hello",
			Sender: &tele.User{ID: 123},
			Chat:   &tele.Chat{ID: 123},
		},
	})

	// Wait for text handler with retry logic
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		handlerMu.Lock()
		called := textHandlerCalled
		handlerMu.Unlock()
		if called {
			break
		}
	}

	// Send callback
	poller.sendUpdate(tele.Update{
		Callback: &tele.Callback{
			Data:   "test",
			Sender: &tele.User{ID: 123},
			Message: &tele.Message{
				ID:     1,
				Sender: &tele.User{ID: 123},
				Chat:   &tele.Chat{ID: 123},
			},
		},
	})

	// Wait for callback handler with retry logic
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		handlerMu.Lock()
		called := callbackHandlerCalled
		handlerMu.Unlock()
		if called {
			break
		}
	}

	handlerMu.Lock()
	defer handlerMu.Unlock()

	if !textHandlerCalled {
		t.Error("Text handler was not called")
	}

	if !callbackHandlerCalled {
		t.Error("Callback handler was not called")
	}

	cancel()
	<-stopCh
}

// controllablePoller is a custom poller for testing
type controllablePoller struct {
	updates chan tele.Update
	stopped bool
	mu      sync.Mutex
}

func (p *controllablePoller) Poll(bot *tele.Bot, updates chan tele.Update, stop chan struct{}) {
	for {
		select {
		case upd := <-p.updates:
			select {
			case updates <- upd:
			case <-stop:
				p.mu.Lock()
				p.stopped = true
				p.mu.Unlock()
				return
			}
		case <-stop:
			p.mu.Lock()
			p.stopped = true
			p.mu.Unlock()
			return
		}
	}
}

func (p *controllablePoller) sendUpdate(upd tele.Update) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.stopped {
		select {
		case p.updates <- upd:
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// mockUserStorage implements UsersStorage for testing
type mockUserStorage struct {
	users map[int64]UserModel
	mu    sync.RWMutex
}

func (m *mockUserStorage) Insert(ctx context.Context, user UserModel) error {
	if m.users == nil {
		m.users = make(map[int64]UserModel)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	return nil
}

func (m *mockUserStorage) Find(ctx context.Context, id int64) (UserModel, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, found := m.users[id]
	return user, found, nil
}

func (m *mockUserStorage) UpdateAsync(id int64, diff *UserModelDiff) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if user, found := m.users[id]; found {
		// Apply diff (simplified)
		if diff.State != nil && diff.State.Main != nil {
			user.State.Main = *diff.State.Main
		}
		m.users[id] = user
	}
}

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

	b, err := NewWithOptions(t.Context(), "dummy-token", opts)
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
