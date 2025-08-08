package bote

import (
	"context"
	"strings"
	"testing"
	"time"
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
	}

	bot, err := NewWithOptions("test-token", opts)
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
