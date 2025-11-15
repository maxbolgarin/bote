package bote

import (
	"context"
	"sync"
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
