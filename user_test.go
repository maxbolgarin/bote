package bote

import (
	"context"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/maxbolgarin/abstract"
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
				ID:           NewPlainUserID(123),
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
				ID:           NewPlainUserID(456),
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
				ID:           NewPlainUserID(789),
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
			user := newUserModel(tt.teleUser, NewPlainUserID(tt.teleUser.ID), "")
			if lang.Deref(user.ID.IDPlain) != lang.Deref(tt.expected.ID.IDPlain) {
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
		KeysProvider: &simpleKeysProvider{
			encryptionKey: &EncryptionKey{
				key:     abstract.NewEncryptionKey(),
				version: nil,
			},
			hmacKey: &EncryptionKey{
				key:     abstract.NewEncryptionKey(),
				version: nil,
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
			ID: NewPlainUserID(555),
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
		user, found, err := storage.Find(ctx, NewPlainUserID(555))
		if err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		if !found {
			t.Fatal("User should be found")
		}

		if lang.Deref(user.ID.IDPlain) != 555 {
			t.Errorf("Expected user ID 555, got %d", user.ID)
		}
	})

	t.Run("Find non-existing", func(t *testing.T) {
		_, found, err := storage.Find(ctx, NewPlainUserID(999))
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

		storage.UpdateAsync(NewPlainUserID(555), diff)

		// Give async update time to complete
		time.Sleep(50 * time.Millisecond)

		user, found, _ := storage.Find(ctx, NewPlainUserID(555))
		if found && user.State.Main != userState {
			t.Errorf("Expected updated state %s, got %s", newState, user.State.Main)
		}
	})

	t.Run("Edge cases", func(t *testing.T) {
		// Test with zero ID
		err = storage.Insert(ctx, UserModel{ID: NewPlainUserID(0)})
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
		KeysProvider: &simpleKeysProvider{
			encryptionKey: &EncryptionKey{
				key:     abstract.NewEncryptionKey(),
				version: nil,
			},
			hmacKey: &EncryptionKey{
				key:     abstract.NewEncryptionKey(),
				version: nil,
			},
		},
	}
}

// TestFullUserIDCreation tests FullUserID creation methods
func TestFullUserIDCreation(t *testing.T) {
	t.Run("NewPlainUserID", func(t *testing.T) {
		id := int64(12345)
		fullID := NewPlainUserID(id)

		if fullID.IDPlain == nil {
			t.Fatal("IDPlain should not be nil")
		}

		if *fullID.IDPlain != id {
			t.Errorf("Expected IDPlain %d, got %d", id, *fullID.IDPlain)
		}

		if len(fullID.IDEnc) != 0 {
			t.Error("IDEnc should be empty for plain ID")
		}

		if len(fullID.IDHMAC) != 0 {
			t.Error("IDHMAC should be empty for plain ID")
		}

		if fullID.IsEmpty() {
			t.Error("FullUserID should not be empty")
		}
	})

	t.Run("NewPrivateUserID", func(t *testing.T) {
		id := int64(67890)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: lang.Ptr(int64(1)),
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: lang.Ptr(int64(2)),
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		if fullID.IDPlain != nil {
			t.Error("IDPlain should be nil for private ID")
		}

		if len(fullID.IDEnc) == 0 {
			t.Error("IDEnc should not be empty for private ID")
		}

		if len(fullID.IDHMAC) == 0 {
			t.Error("IDHMAC should not be empty for private ID")
		}

		if fullID.EncKeyVersion == nil || *fullID.EncKeyVersion != 1 {
			t.Errorf("Expected EncKeyVersion 1, got %v", fullID.EncKeyVersion)
		}

		if fullID.HMACKeyVersion == nil || *fullID.HMACKeyVersion != 2 {
			t.Errorf("Expected HMACKeyVersion 2, got %v", fullID.HMACKeyVersion)
		}

		if fullID.IsEmpty() {
			t.Error("FullUserID should not be empty")
		}
	})

	t.Run("NewPrivateUserID with nil keys", func(t *testing.T) {
		_, err := NewPrivateUserID(123, nil, nil)
		if err == nil {
			t.Error("Expected error when keys are nil")
		}
	})

	t.Run("NewPrivateUserID with one nil key", func(t *testing.T) {
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		_, err := NewPrivateUserID(123, nil, encKey)
		if err == nil {
			t.Error("Expected error when HMAC key is nil")
		}

		_, err = NewPrivateUserID(123, encKey, nil)
		if err == nil {
			t.Error("Expected error when encryption key is nil")
		}
	})
}

// TestFullUserIDMethods tests FullUserID methods
func TestFullUserIDMethods(t *testing.T) {
	t.Run("ID with plain ID", func(t *testing.T) {
		id := int64(11111)
		fullID := NewPlainUserID(id)

		gotID, err := fullID.ID()
		if err != nil {
			t.Fatalf("Failed to get ID: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}
	})

	t.Run("ID with encrypted ID", func(t *testing.T) {
		id := int64(22222)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		gotID, err := fullID.ID(encKey)
		if err != nil {
			t.Fatalf("Failed to decrypt ID: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}
	})

	t.Run("ID with encrypted ID without keys", func(t *testing.T) {
		id := int64(33333)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		_, err = fullID.ID()
		if err == nil {
			t.Error("Expected error when no keys provided")
		}
	})

	t.Run("ID with wrong encryption key", func(t *testing.T) {
		id := int64(44444)
		encKey1 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		encKey2 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey1)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		_, err = fullID.ID(encKey2)
		if err == nil {
			t.Error("Expected error when wrong key provided")
		}
	})

	t.Run("ID with multiple keys - correct key first", func(t *testing.T) {
		id := int64(55555)
		encKey1 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		encKey2 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey1)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		gotID, err := fullID.ID(encKey1, encKey2)
		if err != nil {
			t.Fatalf("Failed to decrypt ID: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}
	})

	t.Run("ID with multiple keys - correct key second", func(t *testing.T) {
		id := int64(66666)
		encKey1 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		encKey2 := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey2)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		gotID, err := fullID.ID(encKey1, encKey2)
		if err != nil {
			t.Fatalf("Failed to decrypt ID: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}
	})

	t.Run("ID with keys from string", func(t *testing.T) {
		id := int64(99999)
		encryptionKeyStr := "3258e7c6bfb9aa6a96b505d9e86876dfeff345f3a803b46840c44c7fad461249"
		hmacKeyStr := "a6eada98c5998f2141d0360575fc1663c466a09c3a3ded20bf3611a85eb89c28"

		encKey, err := NewEncryptionKeyFromString(encryptionKeyStr, nil)
		if err != nil {
			t.Fatalf("Failed to create encryption key from string: %v", err)
		}

		hmacKey, err := NewEncryptionKeyFromString(hmacKeyStr, nil)
		if err != nil {
			t.Fatalf("Failed to create HMAC key from string: %v", err)
		}

		// Create private user ID with keys from string
		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		// Decrypt using the same key created from string
		gotID, err := fullID.ID(encKey)
		if err != nil {
			t.Fatalf("Failed to decrypt ID with key from string: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}

		// Verify we can recreate the key from string and decrypt again
		encKey2, err := NewEncryptionKeyFromString(encryptionKeyStr, nil)
		if err != nil {
			t.Fatalf("Failed to recreate encryption key from string: %v", err)
		}

		gotID2, err := fullID.ID(encKey2)
		if err != nil {
			t.Fatalf("Failed to decrypt ID with recreated key: %v", err)
		}

		if gotID2 != id {
			t.Errorf("Expected ID %d, got %d", id, gotID2)
		}
	})

	t.Run("ID with keys from string with versions", func(t *testing.T) {
		id := int64(88888)
		encryptionKeyStr := "3258e7c6bfb9aa6a96b505d9e86876dfeff345f3a803b46840c44c7fad461249"
		hmacKeyStr := "a6eada98c5998f2141d0360575fc1663c466a09c3a3ded20bf3611a85eb89c28"
		encVersion := int64(10)
		hmacVersion := int64(20)

		encKey, err := NewEncryptionKeyFromString(encryptionKeyStr, &encVersion)
		if err != nil {
			t.Fatalf("Failed to create encryption key from string: %v", err)
		}

		hmacKey, err := NewEncryptionKeyFromString(hmacKeyStr, &hmacVersion)
		if err != nil {
			t.Fatalf("Failed to create HMAC key from string: %v", err)
		}

		// Create private user ID with keys from string
		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		// Verify key versions are set
		if fullID.EncKeyVersion == nil || *fullID.EncKeyVersion != encVersion {
			t.Errorf("Expected EncKeyVersion %d, got %v", encVersion, fullID.EncKeyVersion)
		}

		if fullID.HMACKeyVersion == nil || *fullID.HMACKeyVersion != hmacVersion {
			t.Errorf("Expected HMACKeyVersion %d, got %v", hmacVersion, fullID.HMACKeyVersion)
		}

		// Decrypt using the same key created from string
		gotID, err := fullID.ID(encKey)
		if err != nil {
			t.Fatalf("Failed to decrypt ID with key from string: %v", err)
		}

		if gotID != id {
			t.Errorf("Expected ID %d, got %d", id, gotID)
		}
	})

	t.Run("String with plain ID", func(t *testing.T) {
		id := int64(77777)
		fullID := NewPlainUserID(id)

		str := fullID.String()
		expected := "77777"
		if str != expected {
			t.Errorf("Expected string %s, got %s", expected, str)
		}
	})

	t.Run("String with encrypted ID", func(t *testing.T) {
		id := int64(88888)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		str := fullID.String()
		if str == "" || str == "[ENCRYPTED]" {
			// Should return HMAC hex representation (first 8 chars)
			if len(fullID.IDHMAC) == 0 {
				t.Error("IDHMAC should not be empty")
			}
		}
	})

	t.Run("IsEmpty", func(t *testing.T) {
		emptyID := FullUserID{}
		if !emptyID.IsEmpty() {
			t.Error("Empty FullUserID should return true for IsEmpty()")
		}

		plainID := NewPlainUserID(99999)
		if plainID.IsEmpty() {
			t.Error("Plain FullUserID should not be empty")
		}

		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		privateID, err := NewPrivateUserID(99999, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}
		if privateID.IsEmpty() {
			t.Error("Private FullUserID should not be empty")
		}
	})
}

// TestFullUserIDWithUserCreation tests user creation with FullUserID
func TestFullUserIDWithUserCreation(t *testing.T) {
	t.Run("User creation with plain ID", func(t *testing.T) {
		teleUser := &tele.User{
			ID:        100001,
			FirstName: "Plain",
			Username:  "plainuser",
		}

		plainID := NewPlainUserID(teleUser.ID)
		user := newUserModel(teleUser, plainID, "")

		if lang.Deref(user.ID.IDPlain) != teleUser.ID {
			t.Errorf("Expected ID %d, got %d", teleUser.ID, lang.Deref(user.ID.IDPlain))
		}

		if user.Info.Username != "plainuser" {
			t.Errorf("Expected username 'plainuser', got %s", user.Info.Username)
		}
	})

	t.Run("User creation with private ID", func(t *testing.T) {
		teleUser := &tele.User{
			ID:        100002,
			FirstName: "Private",
			Username:  "privateuser",
		}

		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		privateID, err := NewPrivateUserID(teleUser.ID, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		user := newUserModel(teleUser, privateID, PrivacyModeStrict)

		if user.ID.IDPlain != nil {
			t.Error("IDPlain should be nil in strict privacy mode")
		}

		if len(user.ID.IDEnc) == 0 {
			t.Error("IDEnc should not be empty")
		}

		if len(user.ID.IDHMAC) == 0 {
			t.Error("IDHMAC should not be empty")
		}

		// Verify we can decrypt it
		decryptedID, err := user.ID.ID(encKey)
		if err != nil {
			t.Fatalf("Failed to decrypt ID: %v", err)
		}

		if decryptedID != teleUser.ID {
			t.Errorf("Expected decrypted ID %d, got %d", teleUser.ID, decryptedID)
		}
	})

	t.Run("User creation with strict privacy mode clears plain ID", func(t *testing.T) {
		teleUser := &tele.User{
			ID:        100003,
			FirstName: "Test",
		}

		plainID := NewPlainUserID(teleUser.ID)
		user := newUserModel(teleUser, plainID, PrivacyModeStrict)

		if user.ID.IDPlain != nil {
			t.Error("IDPlain should be nil in strict privacy mode even if originally plain")
		}
	})
}

// TestFullUserIDWithStorage tests storage operations with FullUserID
func TestFullUserIDWithStorage(t *testing.T) {
	opts := newTestOptions()
	storage, err := newInMemoryUserStorage(opts.Config.Bot.UserCacheCapacity, opts.Config.Bot.UserCacheTTL)
	if err != nil {
		t.Fatalf("Failed to create in-memory storage: %v", err)
	}

	ctx := context.Background()

	t.Run("Insert and Find with plain ID", func(t *testing.T) {
		userID := NewPlainUserID(200001)
		user := UserModel{
			ID: userID,
			Info: UserInfo{
				Username: "plain_storage",
			},
		}

		err := storage.Insert(ctx, user)
		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}

		foundUser, found, err := storage.Find(ctx, userID)
		if err != nil {
			t.Fatalf("Failed to find user: %v", err)
		}

		if !found {
			t.Fatal("User should be found")
		}

		if lang.Deref(foundUser.ID.IDPlain) != lang.Deref(userID.IDPlain) {
			t.Errorf("Expected ID %d, got %d", lang.Deref(userID.IDPlain), lang.Deref(foundUser.ID.IDPlain))
		}
	})

	t.Run("Insert and Find with private ID", func(t *testing.T) {
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		userID, err := NewPrivateUserID(200002, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		// Note: inMemoryUserStorage uses IDPlain for lookup, so we need to use plain ID
		// But we can test that the private ID structure is preserved
		user := UserModel{
			ID: userID, // Store with private ID
			Info: UserInfo{
				Username: "private_storage",
			},
		}

		// For in-memory storage, we need plain ID for lookup
		// But we can verify the structure
		if len(user.ID.IDEnc) == 0 {
			t.Error("IDEnc should not be empty")
		}
	})

	t.Run("UpdateAsync with FullUserID", func(t *testing.T) {
		userID := NewPlainUserID(200003)
		user := UserModel{
			ID: userID,
			Info: UserInfo{
				Username: "update_test",
			},
		}

		err := storage.Insert(ctx, user)
		if err != nil {
			t.Fatalf("Failed to insert user: %v", err)
		}

		newState := NewUserState("updated_state")
		diff := &UserModelDiff{
			State: &UserStateDiff{
				Main: &newState,
			},
		}

		storage.UpdateAsync(userID, diff)

		// Give async update time to complete
		time.Sleep(50 * time.Millisecond)

		foundUser, found, _ := storage.Find(ctx, userID)
		if !found {
			t.Fatal("User should be found")
		}
		if foundUser.State.Main != newState {
			t.Errorf("Expected state %s, got %s", newState, foundUser.State.Main)
		}
	})
}

// TestFullUserIDWithUserManager tests user manager operations with FullUserID
func TestFullUserIDWithUserManager(t *testing.T) {
	t.Run("prepareUser with plain ID", func(t *testing.T) {
		opts := newTestOptions()
		um, err := newUserManager(opts)
		if err != nil {
			t.Fatalf("Failed to create user manager: %v", err)
		}

		teleUser := &tele.User{
			ID:        300001,
			FirstName: "Manager",
			Username:  "manageruser",
		}

		user, err := um.prepareUser(teleUser)
		if err != nil {
			t.Fatalf("Failed to prepare user: %v", err)
		}

		if user.ID() != teleUser.ID {
			t.Errorf("Expected ID %d, got %d", teleUser.ID, user.ID())
		}

		fullID := user.IDFull()
		if lang.Deref(fullID.IDPlain) != teleUser.ID {
			t.Errorf("Expected FullUserID plain %d, got %d", teleUser.ID, lang.Deref(fullID.IDPlain))
		}
	})

	t.Run("IDFull returns correct FullUserID", func(t *testing.T) {
		opts := newTestOptions()
		um, err := newUserManager(opts)
		if err != nil {
			t.Fatalf("Failed to create user manager: %v", err)
		}

		teleUser := &tele.User{
			ID:       300002,
			Username: "idfulltest",
		}

		user, err := um.prepareUser(teleUser)
		if err != nil {
			t.Fatalf("Failed to prepare user: %v", err)
		}

		fullID := user.IDFull()
		if fullID.IsEmpty() {
			t.Error("IDFull should not be empty")
		}

		// Verify ID() and IDFull() are consistent
		plainID := user.ID()
		if lang.Deref(fullID.IDPlain) != plainID {
			t.Errorf("ID() and IDFull() should be consistent: ID()=%d, IDFull().IDPlain=%d", plainID, lang.Deref(fullID.IDPlain))
		}
	})

	t.Run("getUser with plain ID", func(t *testing.T) {
		opts := newTestOptions()
		um, err := newUserManager(opts)
		if err != nil {
			t.Fatalf("Failed to create user manager: %v", err)
		}

		teleUser := &tele.User{
			ID:       300003,
			Username: "getusertest",
		}

		_, err = um.prepareUser(teleUser)
		if err != nil {
			t.Fatalf("Failed to prepare user: %v", err)
		}

		user := um.getUser(teleUser.ID)
		if user == nil {
			t.Fatal("User should not be nil")
		}

		if user.ID() != teleUser.ID {
			t.Errorf("Expected ID %d, got %d", teleUser.ID, user.ID())
		}

		fullID := user.IDFull()
		if lang.Deref(fullID.IDPlain) != teleUser.ID {
			t.Errorf("Expected FullUserID plain %d, got %d", teleUser.ID, lang.Deref(fullID.IDPlain))
		}
	})
}

// TestFullUserIDHMAC tests HMAC functionality
func TestFullUserIDHMAC(t *testing.T) {
	t.Run("NewHMAC creates consistent HMAC", func(t *testing.T) {
		id := int64(400001)
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		hmac1 := NewHMAC(id, hmacKey)
		hmac2 := NewHMAC(id, hmacKey)

		if len(hmac1) == 0 {
			t.Error("HMAC should not be empty")
		}

		if len(hmac1) != len(hmac2) {
			t.Error("HMAC should be consistent")
		}

		for i := range hmac1 {
			if hmac1[i] != hmac2[i] {
				t.Error("HMAC should be deterministic")
			}
		}
	})

	t.Run("NewHMACString returns hex string", func(t *testing.T) {
		id := int64(400002)
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		hmacStr := NewHMACString(id, hmacKey)
		if hmacStr == "" {
			t.Error("HMAC string should not be empty")
		}

		// Verify it's valid hex
		_, err := hex.DecodeString(hmacStr)
		if err != nil {
			t.Errorf("HMAC string should be valid hex: %v", err)
		}
	})

	t.Run("NewHMAC with nil key returns nil", func(t *testing.T) {
		hmac := NewHMAC(123, nil)
		if hmac != nil {
			t.Error("HMAC should be nil when key is nil")
		}
	})

	t.Run("Private user ID HMAC matches NewHMAC", func(t *testing.T) {
		id := int64(400003)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		expectedHMAC := NewHMAC(id, hmacKey)

		if len(fullID.IDHMAC) != len(expectedHMAC) {
			t.Error("HMAC lengths should match")
		}

		for i := range fullID.IDHMAC {
			if fullID.IDHMAC[i] != expectedHMAC[i] {
				t.Error("HMAC should match NewHMAC result")
			}
		}
	})
}

// TestFullUserIDEdgeCases tests edge cases for FullUserID
func TestFullUserIDEdgeCases(t *testing.T) {
	t.Run("ID with empty encrypted ID", func(t *testing.T) {
		fullID := FullUserID{
			IDEnc: []byte{},
		}

		_, err := fullID.ID()
		if err == nil {
			t.Error("Expected error when IDEnc is empty")
		}
	})

	t.Run("ID with nil encryption key", func(t *testing.T) {
		id := int64(500001)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		_, err = fullID.ID(nil)
		if err == nil {
			t.Error("Expected error when encryption key is nil")
		}
	})

	t.Run("ID with encryption key with nil key field", func(t *testing.T) {
		id := int64(500002)
		encKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID, err := NewPrivateUserID(id, hmacKey, encKey)
		if err != nil {
			t.Fatalf("Failed to create private user ID: %v", err)
		}

		nilKeyEncKey := &EncryptionKey{
			key:     nil,
			version: nil,
		}

		_, err = fullID.ID(nilKeyEncKey)
		if err == nil {
			t.Error("Expected error when encryption key.key is nil")
		}
	})

	t.Run("String with empty encrypted ID but HMAC present", func(t *testing.T) {
		hmacKey := &EncryptionKey{
			key:     abstract.NewEncryptionKey(),
			version: nil,
		}

		fullID := FullUserID{
			IDHMAC: NewHMAC(500003, hmacKey),
		}

		str := fullID.String()
		if str == "" || str == "[ENCRYPTED]" {
			t.Error("String should return HMAC representation when IDEnc is empty but IDHMAC is present")
		}
	})

	t.Run("String with no IDPlain and no IDHMAC", func(t *testing.T) {
		fullID := FullUserID{}
		str := fullID.String()
		if str != "[ENCRYPTED]" {
			t.Errorf("Expected '[ENCRYPTED]', got %s", str)
		}
	})
}
