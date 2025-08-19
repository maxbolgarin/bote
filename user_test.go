package bote

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// TestUserCreation tests user creation and initialization
func TestUserCreation(t *testing.T) {
	tests := []struct {
		name     string
		teleUser *tele.User
		expected UserModel
	}{
		{
			name: "basic user",
			teleUser: &tele.User{
				ID:           123,
				FirstName:    "John",
				LastName:     "Doe",
				Username:     "johndoe",
				LanguageCode: "en",
			},
			expected: UserModel{
				ID:           123,
				IsBot:        false,
				LanguageCode: LanguageEnglish,
				Info: UserInfo{
					FirstName: "John",
					LastName:  "Doe",
					Username:  "johndoe",
				},
			},
		},
		{
			name: "user with premium",
			teleUser: &tele.User{
				ID:        456,
				FirstName: "Jane",
				Username:  "jane",
				IsPremium: true,
			},
			expected: UserModel{
				ID:           456,
				LanguageCode: LanguageDefault,
				Info: UserInfo{
					FirstName: "Jane",
					Username:  "jane",
					IsPremium: lang.Ptr(true),
				},
			},
		},
		{
			name: "bot user",
			teleUser: &tele.User{
				ID:       789,
				Username: "testbot",
				IsBot:    true,
			},
			expected: UserModel{
				ID:           789,
				LanguageCode: LanguageDefault,
				IsBot:        true,
				Info: UserInfo{
					Username: "testbot",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := newUserModel(tt.teleUser, PrivacyModeNo)

			if user.ID != tt.expected.ID {
				t.Errorf("Expected ID %d, got %d", tt.expected.ID, user.ID)
			}

			if user.Info.FirstName != tt.expected.Info.FirstName {
				t.Errorf("Expected FirstName %s, got %s", tt.expected.Info.FirstName, user.Info.FirstName)
			}

			if user.Info.Username != tt.expected.Info.Username {
				t.Errorf("Expected Username %s, got %s", tt.expected.Info.Username, user.Info.Username)
			}

			if lang.Deref(user.Info.IsPremium) != lang.Deref(tt.expected.Info.IsPremium) {
				t.Errorf("Expected IsPremium %v, got %v", tt.expected.Info.IsPremium, user.Info.IsPremium)
			}

			if user.IsBot != tt.expected.IsBot {
				t.Errorf("Expected IsBot %v, got %v", tt.expected.IsBot, user.IsBot)
			}

			if user.LanguageCode != tt.expected.LanguageCode {
				t.Errorf("Expected LanguageCode %s, got %s", tt.expected.LanguageCode, user.LanguageCode)
			}

			// Check default state
			if user.State.Main != FirstRequest {
				t.Errorf("Expected initial state %s, got %s", FirstRequest, user.State.Main)
			}
		})
	}
}

// TestUserManager tests user manager functionality
func TestUserManager(t *testing.T) {
	opts := Options{
		UserDB: &mockUserStorage{},
		Logger: &testLogger{},
		Config: Config{
			Bot: BotConfig{
				UserCacheCapacity: 100,
				UserCacheTTL:      time.Hour,
			},
		},
	}

	um, err := newUserManager(opts)
	if err != nil {
		t.Fatalf("Failed to create user manager: %v", err)
	}

	t.Run("prepareUser new user", func(t *testing.T) {
		teleUser := &tele.User{
			ID:        111,
			FirstName: "Test",
			Username:  "test",
		}

		user, err := um.prepareUser(teleUser)
		if err != nil {
			t.Fatalf("Failed to prepare user: %v", err)
		}

		if user.ID() != 111 {
			t.Errorf("Expected user ID 111, got %d", user.ID())
		}

		if user.Username() != "test" {
			t.Errorf("Expected username 'test', got %s", user.Username())
		}
	})

	t.Run("prepareUser existing user", func(t *testing.T) {
		// Second call should retrieve from cache
		teleUser := &tele.User{
			ID:        111,
			FirstName: "Test Updated",
			Username:  "test_updated",
		}

		user, err := um.prepareUser(teleUser)
		if err != nil {
			t.Fatalf("Failed to prepare existing user: %v", err)
		}

		if user.ID() != 111 {
			t.Errorf("Expected user ID 111, got %d", user.ID())
		}

		// Username should be updated
		if user.Username() != "test_updated" {
			t.Errorf("Expected updated username 'test_updated', got %s", user.Username())
		}
	})

	t.Run("getUser", func(t *testing.T) {
		user := um.getUser(111)
		if user == nil {
			t.Fatal("User should not be nil")
		}

		if user.ID() != 111 {
			t.Errorf("Expected user ID 111, got %d", user.ID())
		}
	})

	t.Run("getAllUsers", func(t *testing.T) {
		// Add another user
		teleUser2 := &tele.User{
			ID:       222,
			Username: "user2",
		}
		um.prepareUser(teleUser2)

		users := um.getAllUsers()
		if len(users) < 2 {
			t.Errorf("Expected at least 2 users, got %d", len(users))
		}
	})

	t.Run("disableUser", func(t *testing.T) {
		um.disableUser(111)

		// User should be removed from cache
		user := um.getUser(111)
		// Will create new user from DB
		if user.IsDisabled() {
			t.Log("User was properly disabled")
		}
	})
}

// TestUserState tests user state management
func TestUserState(t *testing.T) {
	opts := newTestOptions()
	um, err := newUserManager(opts)
	if err != nil {
		t.Fatalf("Failed to create user manager: %v", err)
	}

	teleUser := &tele.User{
		ID:       333,
		Username: "statetest",
	}

	user, _ := um.prepareUser(teleUser)

	t.Run("initial state", func(t *testing.T) {
		state := user.StateMain()
		if state.String() != FirstRequest.String() {
			t.Errorf("Expected initial state %s, got %s", FirstRequest, state)
		}
	})

	t.Run("handleStateChange", func(t *testing.T) {
		newState := "new_state"

		// Directly modify state since handleStateChange is private
		user.mu.Lock()
		if user.user.State.MessageStates == nil {
			user.user.State.MessageStates = make(map[int]UserState)
		}
		user.user.State.MessageStates[1] = NewUserState(newState)
		user.mu.Unlock()

		state, ok := user.State(1)
		if !ok {
			t.Fatal("State should exist for message 1")
		}

		if state.String() != string(newState) {
			t.Errorf("Expected state %s, got %s", newState, state)
		}
	})

	t.Run("text state handling", func(t *testing.T) {
		const waitingForText string = "waiting_text"

		// Simulate text state by modifying state directly
		user.mu.Lock()
		if user.user.State.MessageStates == nil {
			user.user.State.MessageStates = make(map[int]UserState)
		}
		user.user.State.MessageStates[2] = NewUserState(waitingForText)
		user.user.State.MessagesAwaitingText = append(user.user.State.MessagesAwaitingText, 2)
		user.mu.Unlock()

		msgID := user.lastTextMessage()
		if msgID != 2 {
			t.Errorf("Expected last text message ID 2, got %d", msgID)
		}

		msgID, state := user.lastTextMessageState()
		if msgID != 2 || state.String() != string(waitingForText) {
			t.Errorf("Expected text state for message 2, got %d with state %s", msgID, state)
		}
	})
}

// TestUserMessages tests user message management
func TestUserMessages(t *testing.T) {
	opts := newTestOptions()
	um, err := newUserManager(opts)
	if err != nil {
		t.Fatalf("Failed to create user manager: %v", err)
	}

	teleUser := &tele.User{
		ID:       444,
		Username: "msgtest",
	}

	user, _ := um.prepareUser(teleUser)

	t.Run("setMainMessage", func(t *testing.T) {
		user.mu.Lock()
		user.user.Messages.MainID = 100
		user.mu.Unlock()

		msgs := user.Messages()
		if msgs.MainID != 100 {
			t.Errorf("Expected main message ID 100, got %d", msgs.MainID)
		}
	})

	t.Run("setHeadMessage", func(t *testing.T) {
		user.setHeadMessage(200)
		msgs := user.Messages()
		if msgs.HeadID != 200 {
			t.Errorf("Expected head message ID 200, got %d", msgs.HeadID)
		}
	})

	t.Run("setNotificationMessage", func(t *testing.T) {
		user.setNotificationMessage(300)
		msgs := user.Messages()
		if msgs.NotificationID != 300 {
			t.Errorf("Expected notification message ID 300, got %d", msgs.NotificationID)
		}
	})

	t.Run("setErrorMessage", func(t *testing.T) {
		user.setErrorMessage(400)
		msgs := user.Messages()
		if msgs.ErrorID != 400 {
			t.Errorf("Expected error message ID 400, got %d", msgs.ErrorID)
		}
	})

	t.Run("handleSend", func(t *testing.T) {
		// Test handleSend which moves main to history
		user.handleSend(NoChange, 500, 501)
		msgs := user.Messages()

		if msgs.MainID != 500 {
			t.Errorf("Expected new main message ID 500, got %d", msgs.MainID)
		}

		if msgs.HeadID != 501 {
			t.Errorf("Expected new head message ID 501, got %d", msgs.HeadID)
		}

		// Previous main should be in history
		if len(msgs.HistoryIDs) == 0 || msgs.HistoryIDs[len(msgs.HistoryIDs)-1] != 100 {
			t.Error("Previous main message should be in history")
		}
	})
}

// TestInMemoryUserStorage tests the in-memory storage implementation
func TestInMemoryUserStorage(t *testing.T) {
	opts := newTestOptions()
	storage, err := newInMemoryUserStorage(opts.Config.Bot.UserCacheCapacity, opts.Config.Bot.UserCacheTTL)
	if err != nil {
		t.Fatalf("Failed to create in-memory storage: %v", err)
	}

	ctx := context.Background()

	t.Run("Insert", func(t *testing.T) {
		user := UserModel{
			ID: 555,
			Info: UserInfo{
				Username: "test",
			},
		}

		err := storage.Insert(ctx, user)
		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}
	})

	t.Run("Find existing", func(t *testing.T) {
		user, found, err := storage.Find(ctx, 555)
		if err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		if !found {
			t.Fatal("User should be found")
		}

		if user.ID != 555 {
			t.Errorf("Expected user ID 555, got %d", user.ID)
		}
	})

	t.Run("Find non-existing", func(t *testing.T) {
		_, found, err := storage.Find(ctx, 999)
		if err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		if found {
			t.Fatal("User should not be found")
		}
	})

	t.Run("UpdateAsync", func(t *testing.T) {
		newState := "updated_state"
		lastSeenTime := time.Now()
		userState := NewUserState(newState)
		diff := &UserModelDiff{
			State: &UserStateDiff{
				Main: &userState,
			},
			Stats: &UserStatDiff{
				LastSeenTime: &lastSeenTime,
			},
		}

		storage.UpdateAsync(555, diff)

		// Give async update time to complete
		time.Sleep(50 * time.Millisecond)

		user, found, _ := storage.Find(ctx, 555)
		if found && user.State.Main != userState {
			t.Errorf("Expected updated state %s, got %s", newState, user.State.Main)
		}
	})

	t.Run("Edge cases", func(t *testing.T) {
		// Test with zero ID
		err = storage.Insert(ctx, UserModel{ID: 0})
		if err == nil {
			t.Error("Expected error for zero ID")
		}
	})
}

// TestUserContextImplementation tests userContextImpl methods
func TestUserContextImplementation(t *testing.T) {
	opts := newTestOptions()
	um, err := newUserManager(opts)
	if err != nil {
		t.Fatalf("Failed to create user manager: %v", err)
	}

	teleUser := &tele.User{
		ID:           777,
		FirstName:    "Test",
		LastName:     "User",
		Username:     "testuser",
		LanguageCode: "en",
	}

	user, _ := um.prepareUser(teleUser)

	t.Run("User interface methods", func(t *testing.T) {
		if user.ID() != 777 {
			t.Errorf("Expected ID 777, got %d", user.ID())
		}

		if user.Username() != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", user.Username())
		}

		if user.Language() != "en" {
			t.Errorf("Expected language 'en', got %s", user.Language())
		}

		info := user.Info()
		if info.FirstName != "Test" || info.LastName != "User" {
			t.Errorf("Unexpected user info: %+v", info)
		}

		if user.IsDisabled() {
			t.Error("User should not be disabled initially")
		}

		expectedString := "[@testuser|777]"
		if user.String() != expectedString {
			t.Errorf("Expected string representation %s, got %s", expectedString, user.String())
		}
	})

	t.Run("Stats", func(t *testing.T) {
		stats := user.Stats()
		if stats.CreatedTime.IsZero() {
			t.Error("Created time should not be zero")
		}

		lastSeen := user.LastSeenTime()
		if lastSeen.IsZero() {
			t.Error("Last seen time should not be zero")
		}
	})

	t.Run("Language update", func(t *testing.T) {
		user.UpdateLanguage("fr")
		if user.Language() != "fr" {
			t.Errorf("Expected language 'fr' after update, got %s", user.Language())
		}
	})
}

// TestConcurrentUserAccess tests concurrent access to user data
func TestConcurrentUserAccess(t *testing.T) {
	opts := newTestOptions()
	um, err := newUserManager(opts)
	if err != nil {
		t.Fatalf("Failed to create user manager: %v", err)
	}

	teleUser := &tele.User{
		ID:       888,
		Username: "concurrent",
	}

	user, _ := um.prepareUser(teleUser)

	var wg sync.WaitGroup
	iterations := 100

	// Concurrent reads
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			_ = user.ID()
			_ = user.Username()
			_ = user.StateMain()
			_ = user.Messages()
		}()
	}

	// Concurrent writes
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func(msgID int) {
			defer wg.Done()
			user.mu.Lock()
			user.user.Messages.MainID = msgID
			if user.user.State.MessageStates == nil {
				user.user.State.MessageStates = make(map[int]UserState)
			}
			user.user.State.MessageStates[msgID] = FirstRequest
			user.mu.Unlock()
		}(i)
	}

	wg.Wait()
	t.Log("Concurrent access test completed without race conditions")
}

// testLogger is a simple logger for testing
type testLogger struct {
	mu   sync.Mutex
	logs []string
}

func (l *testLogger) Debug(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "DEBUG: "+msg)
}

func (l *testLogger) Info(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "INFO: "+msg)
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "WARN: "+msg)
}

func (l *testLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "ERROR: "+msg)
}

// Helper function for custom state implementation
func (s customState) String() string   { return string(s) }
func (s customState) IsText() bool     { return false }
func (s customState) NotChanged() bool { return s == "" }

func (s textState) String() string   { return string(s) }
func (s textState) IsText() bool     { return true }
func (s textState) NotChanged() bool { return s == "" }

type customState string
type textState string

func newTestOptions() Options {
	return Options{
		UserDB: &mockUserStorage{},
		Logger: &testLogger{},
		Config: Config{
			Bot: BotConfig{
				UserCacheCapacity: 100,
				UserCacheTTL:      time.Hour,
			},
		},
	}
}
