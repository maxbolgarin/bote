package bote

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

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
	m.users[lang.Deref(user.ID.IDPlain)] = user
	return nil
}

func (m *mockUserStorage) Find(ctx context.Context, id FullUserID) (UserModel, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, found := m.users[lang.Deref(id.IDPlain)]
	return user, found, nil
}

func (m *mockUserStorage) UpdateAsync(id FullUserID, diff *UserModelDiff) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if user, found := m.users[lang.Deref(id.IDPlain)]; found {
		// Apply diff (simplified)
		if diff.State != nil && diff.State.Main != nil {
			user.State.Main = *diff.State.Main
		}
		m.users[lang.Deref(id.IDPlain)] = user
	}
}

func (m *mockUserStorage) Delete(_ context.Context, id FullUserID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.users, lang.Deref(id.IDPlain))
	return nil
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

func boolPtr(v bool) *bool { return &v }

func TestUserIDWrapperRecipient(t *testing.T) {
	tests := []struct {
		name     string
		id       int64
		expected string
	}{
		{"zero", 0, "0"},
		{"positive", 12345, "12345"},
		{"negative_chat_id", -1001234567890, "-1001234567890"},
		{"large_user_id", 9999999999999, "9999999999999"},
		{"max_int32_plus_one", 2147483648, "2147483648"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := userIDWrapper(tt.id)
			if got := w.Recipient(); got != tt.expected {
				t.Errorf("Recipient() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCbackRxWithFormFeedPrefix(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		expectMatch  bool
		expectUnique string
		expectData   string
	}{
		{"plain_unique_no_data", "btn12345678", true, "btn12345678", ""},
		{"unique_with_data", "btn12345678|payload", true, "btn12345678", "payload"},
		{"form_feed_prefix_with_data", "\fbtn12345678|payload", true, "btn12345678", "payload"},
		{"form_feed_prefix_no_data", "\fbtn12345678", true, "btn12345678", ""},
		{"empty_string", "", false, "", ""},
		{"data_with_pipes", "btn1234|a|b|c", true, "btn1234", "a|b|c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := cbackRx.FindAllStringSubmatch(tt.data, -1)
			if !tt.expectMatch {
				if match != nil {
					t.Errorf("expected no match for %q", tt.data)
				}
				return
			}
			if match == nil {
				t.Fatalf("expected match for %q, got nil", tt.data)
			}
			if got := match[0][1]; got != tt.expectUnique {
				t.Errorf("unique = %q, want %q", got, tt.expectUnique)
			}
			if got := match[0][3]; got != tt.expectData {
				t.Errorf("data = %q, want %q", got, tt.expectData)
			}
		})
	}
}
