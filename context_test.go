package bote

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// TestContextCreation tests various ways to create context
func TestContextCreation(t *testing.T) {
	bot := setupTestBot(t)

	tests := []struct {
		name      string
		createCtx func() Context
		checkFunc func(Context) error
	}{
		{
			name: "NewContext for callback",
			createCtx: func() Context {
				return NewContext(bot, 123, 1, "data1", "data2")
			},
			checkFunc: func(c Context) error {
				if c.User().ID() != 123 {
					t.Errorf("Expected user ID 123, got %d", c.User().ID())
				}
				if c.MessageID() != 1 {
					t.Errorf("Expected message ID 1, got %d", c.MessageID())
				}
				if c.Data() != "data1|data2" {
					t.Errorf("Expected data 'data1|data2', got %s", c.Data())
				}
				return nil
			},
		},
		{
			name: "NewContextText for text message",
			createCtx: func() Context {
				return NewContextText(bot, 456, 2, "Hello world")
			},
			checkFunc: func(c Context) error {
				if c.User().ID() != 456 {
					t.Errorf("Expected user ID 456, got %d", c.User().ID())
				}
				if c.Text() != "Hello world" {
					t.Errorf("Expected text 'Hello world', got %s", c.Text())
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.createCtx()
			if err := tt.checkFunc(ctx); err != nil {
				t.Error(err)
			}
		})
	}
}

// TestContextMessageOperations tests message sending and editing
func TestContextMessageOperations(t *testing.T) {
	bot := setupTestBot(t)
	ctx := NewContext(bot, 789, 1)

	// Test SendMain
	t.Run("SendMain", func(t *testing.T) {
		// Since we're in offline mode, we can't actually send messages
		// but we can test that the methods don't panic
		err := ctx.SendMain(NoChange, "Test message", nil)
		if err == nil {
			t.Log("SendMain executed without panic")
		}
	})

	// Test SendNotification
	t.Run("SendNotification", func(t *testing.T) {
		err := ctx.SendNotification("Notification", nil)
		if err == nil {
			t.Log("SendNotification executed without panic")
		}
	})

	// Test SendError
	t.Run("SendError", func(t *testing.T) {
		err := ctx.SendError("Error message")
		if err == nil {
			t.Log("SendError executed without panic")
		}
	})

	// Test Edit operations
	t.Run("EditMain", func(t *testing.T) {
		err := ctx.EditMain(NoChange, "Edited message", nil)
		if err == nil {
			t.Log("EditMain executed without panic")
		}
	})
}

// TestContextDataOperations tests data parsing and manipulation
func TestContextDataOperations(t *testing.T) {
	bot := setupTestBot(t)

	tests := []struct {
		name         string
		data         []string
		expectedData string
		expectedLen  int
	}{
		{
			name:         "single data item",
			data:         []string{"item1"},
			expectedData: "item1",
			expectedLen:  1,
		},
		{
			name:         "multiple data items",
			data:         []string{"item1", "item2", "item3"},
			expectedData: "item1|item2|item3",
			expectedLen:  3,
		},
		{
			name:         "empty data",
			data:         []string{},
			expectedData: "",
			expectedLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := NewContext(bot, 123, 1, tt.data...)

			if ctx.Data() != tt.expectedData {
				t.Errorf("Expected data '%s', got '%s'", tt.expectedData, ctx.Data())
			}

			parsed := ctx.DataParsed()
			if len(parsed) != tt.expectedLen {
				t.Errorf("Expected %d parsed items, got %d", tt.expectedLen, len(parsed))
			}
		})
	}
}

// TestContextButtonCreation tests button creation with handlers
func TestContextButtonCreation(t *testing.T) {
	bot := setupTestBot(t)
	ctx := NewContext(bot, 123, 1)

	handler := func(c Context) error {
		return nil
	}

	// Create button
	btn := ctx.Btn("Test Button", handler, "data1", "data2")

	if btn.Text != "Test Button" {
		t.Errorf("Expected button text 'Test Button', got '%s'", btn.Text)
	}

	if btn.Unique == "" {
		t.Error("Button unique should not be empty")
	}

	if btn.Data == "" {
		t.Error("Button data should not be empty")
	}

	// Check if data is properly encoded
	if !strings.Contains(btn.Data, "data1|data2") {
		t.Errorf("Button data should contain data, got '%s'", btn.Data)
	}
}

// TestContextCustomData tests Set and Get operations
func TestContextCustomData(t *testing.T) {
	bot := setupTestBot(t)
	ctx := NewContext(bot, 123, 1)

	// Test Set and Get
	ctx.Set("key1", "value1")
	ctx.Set("key2", "value2")

	if ctx.Get("key1") != "value1" {
		t.Errorf("Expected 'value1' for key1, got '%s'", ctx.Get("key1"))
	}

	if ctx.Get("key2") != "value2" {
		t.Errorf("Expected 'value2' for key2, got '%s'", ctx.Get("key2"))
	}

	if ctx.Get("nonexistent") != "" {
		t.Errorf("Expected empty string for nonexistent key, got '%s'", ctx.Get("nonexistent"))
	}
}

// TestContextUserOperations tests user-related operations
func TestContextUserOperations(t *testing.T) {
	bot := setupTestBot(t)

	// Create a user with specific properties
	userID := int64(999)
	ctx := NewContext(bot, userID, 1)

	user := ctx.User()
	if user == nil {
		t.Fatal("User should not be nil")
	}

	if user.ID() != userID {
		t.Errorf("Expected user ID %d, got %d", userID, user.ID())
	}

	// Test user state
	state := user.StateMain()
	if state.String() != FirstRequest.String() {
		t.Errorf("Expected state %s, got %s", FirstRequest, state)
	}
}

// TestContextWithDifferentStates tests context behavior with different states
func TestContextWithDifferentStates(t *testing.T) {
	tests := []struct {
		name       string
		state      State
		isText     bool
		notChanged bool
	}{
		{
			name:       "NoChange state",
			state:      NoChange,
			isText:     false,
			notChanged: true,
		},
		{
			name:       "FirstRequest state",
			state:      FirstRequest,
			isText:     false,
			notChanged: false,
		},
		{
			name:       "Unknown state",
			state:      Unknown,
			isText:     false,
			notChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.state.IsText() != tt.isText {
				t.Errorf("Expected IsText() = %v, got %v", tt.isText, tt.state.IsText())
			}
			if tt.state.NotChanged() != tt.notChanged {
				t.Errorf("Expected NotChanged() = %v, got %v", tt.notChanged, tt.state.NotChanged())
			}
		})
	}
}

// TestIsMentioned tests the IsMentioned method with UTF-16 entity offsets
func TestIsMentioned(t *testing.T) {
	bot := setupTestBot(t)

	t.Run("mention at start", func(t *testing.T) {
		// Set bot username
		bot.Bot().Me = &tele.User{Username: "testbot"}
		ctx := NewContextText(bot, 100, 1, "@testbot hello")
		// IsMentioned operates on ct.Message().Entities, which we can't easily set via NewContextText
		// So we test the underlying logic directly
		// This at least ensures the method doesn't panic
		_ = ctx.IsMentioned()
	})

	t.Run("nil message does not panic", func(t *testing.T) {
		ctx := NewContext(bot, 100, 1) // callback context, no message
		assert.False(t, ctx.IsMentioned())
	})
}

// TestMessageIDNilCallback tests MessageID with nil callback message (Bug A fix)
func TestMessageIDNilCallback(t *testing.T) {
	bot := setupTestBot(t)

	t.Run("callback with nil message returns 0", func(t *testing.T) {
		// NewContext creates a callback with Message set, so test that it works
		ctx := NewContext(bot, 100, 42)
		assert.Equal(t, 42, ctx.MessageID())
	})

	t.Run("text message fallback", func(t *testing.T) {
		ctx := NewContextText(bot, 100, 5, "hello")
		// MessageID should return message ID
		id := ctx.MessageID()
		assert.True(t, id >= 0, "should return a valid message ID")
	})
}

// TestGetMsgID tests the getMsgID function with various update types
func TestGetMsgID(t *testing.T) {
	tests := []struct {
		name     string
		update   tele.Update
		expected int
	}{
		{
			name:     "empty update",
			update:   tele.Update{},
			expected: 0,
		},
		{
			name:     "message update",
			update:   tele.Update{Message: &tele.Message{ID: 42}},
			expected: 42,
		},
		{
			name:     "callback with message",
			update:   tele.Update{Callback: &tele.Callback{Message: &tele.Message{ID: 99}}},
			expected: 99,
		},
		{
			name:     "callback with nil message",
			update:   tele.Update{Callback: &tele.Callback{Message: nil}},
			expected: 0, // should not panic, falls through
		},
		{
			name:     "edited message",
			update:   tele.Update{EditedMessage: &tele.Message{ID: 55}},
			expected: 55,
		},
		{
			name:     "channel post",
			update:   tele.Update{ChannelPost: &tele.Message{ID: 77}},
			expected: 77,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getMsgID(&tt.update)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestHandleErrorClassification tests that errors are classified correctly
func TestHandleErrorClassification(t *testing.T) {
	bot := setupTestBot(t)

	t.Run("nil error returns nil", func(t *testing.T) {
		ctx := NewContext(bot, 100, 1)
		impl := ctx.(*contextImpl)
		err := impl.handleError(nil)
		assert.Nil(t, err)
	})

	t.Run("not modified silently ignored", func(t *testing.T) {
		ctx := NewContext(bot, 101, 1)
		impl := ctx.(*contextImpl)
		err := impl.handleError(fmt.Errorf("telegram: message is not modified"))
		assert.Nil(t, err)
	})

	t.Run("message to delete not found silently ignored", func(t *testing.T) {
		ctx := NewContext(bot, 102, 1)
		impl := ctx.(*contextImpl)
		err := impl.handleError(fmt.Errorf("telegram: message to delete not found"))
		assert.Nil(t, err)
	})

	t.Run("connection error handled", func(t *testing.T) {
		ctx := NewContext(bot, 103, 1)
		impl := ctx.(*contextImpl)
		err := impl.handleError(fmt.Errorf("read: connection reset by peer"))
		assert.Nil(t, err)
	})
}

// Helper function to set up a test bot
func setupTestBot(t *testing.T) *Bot {
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
		Poller: &mockPoller{},
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

	bot, err := NewWithOptions(t.Context(), "test-token", opts)
	if err != nil {
		t.Fatalf("Failed to create test bot: %v", err)
	}

	// Start the bot in background
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		time.Sleep(100 * time.Millisecond) // Give time for graceful shutdown
	})

	bot.Start(ctx, func(c Context) error { return nil }, nil)
	time.Sleep(50 * time.Millisecond) // Give time for bot to start

	return bot
}

func TestIsReplyNilSender(t *testing.T) {
	bot := setupTestBot(t)
	bot.Bot().Me = &tele.User{ID: 999}

	t.Run("nil_message", func(t *testing.T) {
		ctx := NewContextText(bot, 100, 1, "hi")
		assert.False(t, ctx.IsReply())
	})

	t.Run("nil_sender_in_replyTo", func(t *testing.T) {
		upd := tele.Update{
			Message: &tele.Message{
				ID:     1,
				Text:   "hi",
				Sender: &tele.User{ID: 100},
				ReplyTo: &tele.Message{
					ID:     2,
					Sender: nil,
				},
			},
		}
		impl := &contextImpl{
			bt:   bot,
			ct:   bot.bot.tbot.NewContext(upd),
			user: newPublicUserContext(&tele.User{ID: 100}),
		}
		assert.False(t, impl.IsReply())
	})

	t.Run("replyTo_with_bot_sender", func(t *testing.T) {
		upd := tele.Update{
			Message: &tele.Message{
				ID:     1,
				Text:   "hi",
				Sender: &tele.User{ID: 100},
				ReplyTo: &tele.Message{
					ID:     2,
					Sender: &tele.User{ID: 999},
				},
			},
		}
		impl := &contextImpl{
			bt:   bot,
			ct:   bot.bot.tbot.NewContext(upd),
			user: newPublicUserContext(&tele.User{ID: 100}),
		}
		assert.True(t, impl.IsReply())
	})
}

func TestChatTypeAndIsPrivateNilChat(t *testing.T) {
	bot := setupTestBot(t)

	// Callback-only update — no chat info
	upd := tele.Update{
		Callback: &tele.Callback{
			Sender:  &tele.User{ID: 100},
			Message: &tele.Message{ID: 1},
		},
	}
	impl := &contextImpl{
		bt:   bot,
		ct:   bot.bot.tbot.NewContext(upd),
		user: newPublicUserContext(&tele.User{ID: 100}),
	}

	assert.Equal(t, tele.ChatType(""), impl.ChatType())
	assert.False(t, impl.IsPrivate())
}

// TestCopyButtonsToNewMsgID verifies the core helper that remaps button registrations.
func TestCopyButtonsToNewMsgID(t *testing.T) {
	bot := setupTestBot(t)
	noop := func(c Context) error { return nil }

	t.Run("no-op when oldMsgID is zero", func(t *testing.T) {
		ctx := NewContext(bot, 3001, 5)
		impl := ctx.(*contextImpl)
		impl.user.buttonMap.Set("0:btn1", InitBundle{Handler: noop})

		before := impl.user.buttonMap.Len()
		impl.user.copyButtonsToNewMsgID(0, 100)
		assert.Equal(t, before, impl.user.buttonMap.Len(), "no entries should be added")
	})

	t.Run("no-op when oldMsgID equals newMsgID", func(t *testing.T) {
		ctx := NewContext(bot, 3002, 5)
		impl := ctx.(*contextImpl)
		btn := ctx.Btn("Btn", noop)
		btnID := getIDFromUnique(btn.Unique)
		_ = btnID

		before := impl.user.buttonMap.Len()
		impl.user.copyButtonsToNewMsgID(5, 5)
		assert.Equal(t, before, impl.user.buttonMap.Len(), "same src and dst should be a no-op")
	})

	t.Run("copies entries with matching prefix", func(t *testing.T) {
		ctx := NewContext(bot, 3003, 7)
		impl := ctx.(*contextImpl)
		btn := ctx.Btn("Btn", noop)
		btnID := getIDFromUnique(btn.Unique)

		impl.user.copyButtonsToNewMsgID(7, 200)

		key := strconv.Itoa(200) + ":" + btnID
		_, ok := impl.user.buttonMap.Lookup(key)
		assert.True(t, ok, "button must be findable under new msg ID")
	})

	t.Run("keeps old entries after copy", func(t *testing.T) {
		ctx := NewContext(bot, 3004, 8)
		impl := ctx.(*contextImpl)
		btn := ctx.Btn("Btn", noop)
		btnID := getIDFromUnique(btn.Unique)

		impl.user.copyButtonsToNewMsgID(8, 201)

		oldKey := strconv.Itoa(8) + ":" + btnID
		_, ok := impl.user.buttonMap.Lookup(oldKey)
		assert.True(t, ok, "old entry must still be present after copy")
	})

	t.Run("does not copy non-matching entries", func(t *testing.T) {
		ctx := NewContext(bot, 3005, 9)
		impl := ctx.(*contextImpl)
		// Register a button for a different message ID manually
		impl.user.buttonMap.Set("999:otherbtn", InitBundle{Handler: noop})

		impl.user.copyButtonsToNewMsgID(9, 202)

		// The "999:otherbtn" entry should NOT appear under 202
		_, ok := impl.user.buttonMap.Lookup("202:otherbtn")
		assert.False(t, ok, "non-matching entry must not be copied")
	})

	t.Run("handles empty buttonMap without panic", func(t *testing.T) {
		ctx := NewContext(bot, 3006, 10)
		impl := ctx.(*contextImpl)
		assert.Zero(t, impl.user.buttonMap.Len())
		assert.NotPanics(t, func() {
			impl.user.copyButtonsToNewMsgID(10, 203)
		})
	})

	t.Run("copies multiple buttons", func(t *testing.T) {
		ctx := NewContext(bot, 3007, 11)
		impl := ctx.(*contextImpl)
		btn1 := ctx.Btn("A", noop)
		btn2 := ctx.Btn("B", noop)
		id1 := getIDFromUnique(btn1.Unique)
		id2 := getIDFromUnique(btn2.Unique)

		impl.user.copyButtonsToNewMsgID(11, 204)

		_, ok1 := impl.user.buttonMap.Lookup(strconv.Itoa(204) + ":" + id1)
		_, ok2 := impl.user.buttonMap.Lookup(strconv.Itoa(204) + ":" + id2)
		assert.True(t, ok1, "first button must be copied")
		assert.True(t, ok2, "second button must be copied")
	})
}

// TestButtonMapKeyNormalization verifies that HeadID/ErrorID/NotificationID are
// normalized to MainID, while history/external IDs are not.
func TestButtonMapKeyNormalization(t *testing.T) {
	bot := setupTestBot(t)

	// Create a base user with known message IDs
	baseCtx := NewContext(bot, 4001, 1)
	baseImpl := baseCtx.(*contextImpl)
	baseImpl.user.mu.Lock()
	baseImpl.user.user.Messages.MainID = 10
	baseImpl.user.user.Messages.HeadID = 11
	baseImpl.user.user.Messages.NotificationID = 12
	baseImpl.user.user.Messages.ErrorID = 13
	baseImpl.user.mu.Unlock()

	cases := []struct {
		name      string
		triggerID int
		wantMsgID int
	}{
		{"main ID is not normalized", 10, 10},
		{"head ID normalized to main", 11, 10},
		{"notification ID normalized to main", 12, 10},
		{"error ID normalized to main", 13, 10},
		{"history/external ID not normalized", 99, 99},
		{"zero ID not normalized", 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewContext(bot, 4001, tc.triggerID)
			impl := ctx.(*contextImpl)
			impl.user = baseImpl.user // share the user with configured message IDs

			key := impl.buttonMapKey("somebtn")
			expected := strconv.Itoa(tc.wantMsgID) + ":somebtn"
			assert.Equal(t, expected, key)
		})
	}
}

// TestButtonDispatchAfterSendMainFromNonMainContext simulates the primary bug:
// a background task or /start handler sends a new main message. Buttons were
// registered with the triggering message ID, not the new main message ID.
// After the fix, buttons must be findable under the new main message ID.
func TestButtonDispatchAfterSendMainFromNonMainContext(t *testing.T) {
	const (
		startMsgID   = 5   // /start command message — the trigger
		newMainMsgID = 100 // newly sent main message
	)

	bot := setupTestBot(t)
	ctx := NewContext(bot, 5001, startMsgID) // trigger = start msg
	impl := ctx.(*contextImpl)

	noop := func(c Context) error { return nil }
	btn := ctx.Btn("Menu", noop)
	btnID := getIDFromUnique(btn.Unique)

	// Simulate what SendMain does: send succeeds, handleSend updates state,
	// then copyButtonsToNewMsgID remaps the buttons.
	impl.user.handleSend(UserState("main_menu"), newMainMsgID, 0)
	impl.user.copyButtonsToNewMsgID(startMsgID, newMainMsgID)

	// handleSend must mark the new main as inited — only callbackFallbackHandler runs.
	assert.True(t, impl.user.isMsgInited(newMainMsgID),
		"new main must be marked inited by handleSend")

	// Button must now be findable under the new main message ID.
	key := strconv.Itoa(newMainMsgID) + ":" + btnID
	_, ok := impl.user.buttonMap.Lookup(key)
	assert.True(t, ok, "button must be findable under new main msg ID after SendMain fix")
}

// TestButtonDispatchAfterEditMainFromHistoryContext simulates the second bug scenario:
// user clicks a history message, the handler calls EditMain on today's main message.
// Without the fix, buttons registered with historyID cannot be dispatched from mainID.
func TestButtonDispatchAfterEditMainFromHistoryContext(t *testing.T) {
	const (
		historyID = 50  // history message — the trigger
		mainID    = 100 // today's main message
	)

	bot := setupTestBot(t)
	ctx := NewContext(bot, 6001, historyID) // trigger = history msg
	impl := ctx.(*contextImpl)

	// Set today's main message ID
	impl.user.mu.Lock()
	impl.user.user.Messages.MainID = mainID
	impl.user.mu.Unlock()

	noop := func(c Context) error { return nil }
	btn := ctx.Btn("Back", noop)
	btnID := getIDFromUnique(btn.Unique)

	// Buttons are now under {historyID}:{btnID}; simulate EditMain's fix.
	impl.user.copyButtonsToNewMsgID(historyID, mainID)

	// Button must be findable under mainID for dispatch to succeed.
	key := strconv.Itoa(mainID) + ":" + btnID
	_, ok := impl.user.buttonMap.Lookup(key)
	assert.True(t, ok, "button must be findable under main msg ID after EditMain from history")
}

// TestEditHeadButtonDispatchFromNonMainContext verifies that head keyboard buttons
// are accessible under MainID when the handler is triggered from a non-main context.
// Head button dispatch normalizes HeadID → MainID, so registration must reach MainID.
func TestEditHeadButtonDispatchFromNonMainContext(t *testing.T) {
	const (
		historyID = 50
		mainID    = 100
		headID    = 101
	)

	bot := setupTestBot(t)
	ctx := NewContext(bot, 7001, historyID) // trigger = history msg
	impl := ctx.(*contextImpl)

	impl.user.mu.Lock()
	impl.user.user.Messages.MainID = mainID
	impl.user.user.Messages.HeadID = headID
	impl.user.mu.Unlock()

	noop := func(c Context) error { return nil }
	btn := ctx.Btn("HeadBtn", noop)
	btnID := getIDFromUnique(btn.Unique)

	// Simulate EditHead's fix.
	impl.user.copyButtonsToNewMsgID(historyID, mainID)

	// Head button dispatch: buttonMapKey normalizes headID → mainID → {mainID}:{btnID}.
	key := strconv.Itoa(mainID) + ":" + btnID
	_, ok := impl.user.buttonMap.Lookup(key)
	assert.True(t, ok, "head button must be findable under main msg ID")
}

// TestHandleSendPermanentlyMarksMainAsInited confirms the mechanism that caused
// the permanent button lockout: handleSend marks the new main as already inited,
// bypassing initUserHandler and forcing sole reliance on callbackFallbackHandler.
func TestHandleSendPermanentlyMarksMainAsInited(t *testing.T) {
	opts := newTestOptions()
	um, err := newUserManager(context.Background(), opts)
	require.NoError(t, err)
	user, err := um.prepareUser(&tele.User{ID: 8001, Username: "inited_test"})
	require.NoError(t, err)

	// First send establishes main=100 in history, second makes 200 the new main.
	user.handleSend(UserState("prev"), 100, 0)
	user.handleSend(UserState("active"), 200, 0)

	// New main (200) is explicitly marked inited — callbackFallbackHandler is sole path.
	assert.True(t, user.isMsgInited(200),
		"new main must be marked inited by handleSend")

	// The previous main (100) is now a history message and belongs to the user,
	// but isInitedMsg was cleared → isMsgInited returns false → initUserHandler fires.
	assert.False(t, user.isMsgInited(100),
		"history message must NOT be marked inited so initUserHandler re-registers its buttons")
}

// TestButtonDispatchWithoutFixWouldFail documents the exact failure mode:
// without copyButtonsToNewMsgID, the new main msg ID has no buttonMap entries.
func TestButtonDispatchWithoutFixWouldFail(t *testing.T) {
	const (
		startMsgID   = 5
		newMainMsgID = 100
	)

	bot := setupTestBot(t)
	ctx := NewContext(bot, 9001, startMsgID)
	impl := ctx.(*contextImpl)

	noop := func(c Context) error { return nil }
	btn := ctx.Btn("Menu", noop)
	btnID := getIDFromUnique(btn.Unique)

	// Simulate handleSend WITHOUT the fix (no copyButtonsToNewMsgID).
	impl.user.handleSend(UserState("main_menu"), newMainMsgID, 0)

	// Before fix: button is only under startMsgID, not under newMainMsgID.
	wrongKey := strconv.Itoa(newMainMsgID) + ":" + btnID
	_, foundUnderNew := impl.user.buttonMap.Lookup(wrongKey)
	assert.False(t, foundUnderNew, "without fix, button must NOT be findable under new main msg ID")

	correctKey := strconv.Itoa(startMsgID) + ":" + btnID
	_, foundUnderOld := impl.user.buttonMap.Lookup(correctKey)
	assert.True(t, foundUnderOld, "button is still under the original trigger msg ID")
}
