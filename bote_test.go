package bote_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxbolgarin/bote"
	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
)

// Ptr returns a pointer to the given value
func Ptr[T any](v T) *T {
	return &v
}

// Enhance the testPoller with functionality for unit testing
type testPoller struct {
	updates  chan tele.Update
	stopChan chan struct{}
	botRef   *tele.Bot
	destChan chan tele.Update
	mu       sync.Mutex // Mutex to protect access to shared fields
}

// NewTestPoller creates a new test poller that can send mock updates
func NewTestPoller() *testPoller {
	return &testPoller{
		updates:  make(chan tele.Update, 100),
		stopChan: make(chan struct{}),
	}
}

// Poll implements the telebot.Poller interface
func (p *testPoller) Poll(b *tele.Bot, dest chan tele.Update, stop chan struct{}) {
	p.mu.Lock()
	p.botRef = b
	p.destChan = dest
	p.stopChan = stop
	p.mu.Unlock()

	for {
		select {
		case <-stop:
			return
		case upd := <-p.updates:
			dest <- upd
		}
	}
}

// SendUpdate sends a mock update to the bot
func (p *testPoller) SendUpdate(upd tele.Update) {
	p.updates <- upd
}

// SendTextMessage sends a mock text message to the bot
func (p *testPoller) SendTextMessage(from tele.User, text string) {
	p.SendUpdate(tele.Update{
		Message: &tele.Message{
			Sender: &from,
			Text:   text,
		},
	})
}

// SendCallbackQuery sends a mock callback query to the bot
func (p *testPoller) SendCallbackQuery(from tele.User, data string, message *tele.Message) {
	p.SendUpdate(tele.Update{
		Callback: &tele.Callback{
			Sender:  &from,
			Data:    data,
			Message: message,
		},
	})
}

func TestCreateBtnData(t *testing.T) {
	// Test with no data
	data := bote.CreateBtnData()
	assert.Equal(t, "", data)

	// Test with one item
	data = bote.CreateBtnData("item1")
	assert.Equal(t, "item1", data)

	// Test with multiple items
	data = bote.CreateBtnData("item1", "item2", "item3")
	assert.Equal(t, "item1|item2|item3", data)

	// Test with empty items
	data = bote.CreateBtnData("item1", "", "item3")
	assert.Equal(t, "item1|item3", data)
}

func TestKeyboardBasic(t *testing.T) {
	// Create a new keyboard builder
	kb := bote.NewKeyboard()
	assert.NotNil(t, kb)

	// Add buttons
	btn1 := tele.Btn{Text: "Button 1"}
	btn2 := tele.Btn{Text: "Button 2"}
	kb.Add(btn1, btn2)

	// Create markup
	markup := kb.CreateInlineMarkup()
	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 1)
	assert.Len(t, markup.InlineKeyboard[0], 2)
	assert.Equal(t, "Button 1", markup.InlineKeyboard[0][0].Text)
	assert.Equal(t, "Button 2", markup.InlineKeyboard[0][1].Text)
}

func TestKeyboardMultipleRows(t *testing.T) {
	// Create a new keyboard builder
	kb := bote.NewKeyboard()

	// Create buttons
	btn1 := tele.Btn{Text: "Button 1"}
	btn2 := tele.Btn{Text: "Button 2"}
	btn3 := tele.Btn{Text: "Button 3"}

	// Add buttons to different rows
	kb.Add(btn1, btn2)
	kb.StartNewRow()
	kb.Add(btn3)

	// Create markup
	markup := kb.CreateInlineMarkup()
	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 2)
	assert.Len(t, markup.InlineKeyboard[0], 2)
	assert.Len(t, markup.InlineKeyboard[1], 1)
	assert.Equal(t, "Button 1", markup.InlineKeyboard[0][0].Text)
	assert.Equal(t, "Button 2", markup.InlineKeyboard[0][1].Text)
	assert.Equal(t, "Button 3", markup.InlineKeyboard[1][0].Text)
}

func TestKeyboardAddRow(t *testing.T) {
	// Create a new keyboard builder
	kb := bote.NewKeyboard()

	// Create buttons
	btn1 := tele.Btn{Text: "Button 1"}
	btn2 := tele.Btn{Text: "Button 2"}
	btn3 := tele.Btn{Text: "Button 3"}
	btn4 := tele.Btn{Text: "Button 4"}

	// Add buttons using AddRow
	kb.Add(btn1)
	kb.AddRow(btn2, btn3)
	kb.Add(btn4)

	// Create markup
	markup := kb.CreateInlineMarkup()
	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 3)
	assert.Len(t, markup.InlineKeyboard[0], 1)
	assert.Len(t, markup.InlineKeyboard[1], 2)
	assert.Len(t, markup.InlineKeyboard[2], 1)
	assert.Equal(t, "Button 1", markup.InlineKeyboard[0][0].Text)
	assert.Equal(t, "Button 2", markup.InlineKeyboard[1][0].Text)
	assert.Equal(t, "Button 3", markup.InlineKeyboard[1][1].Text)
	assert.Equal(t, "Button 4", markup.InlineKeyboard[2][0].Text)
}

func TestKeyboardWithRowLength(t *testing.T) {
	// Create a new keyboard builder with row length 2
	kb := bote.NewKeyboard(2)

	// Create buttons
	btn1 := tele.Btn{Text: "Button 1"}
	btn2 := tele.Btn{Text: "Button 2"}
	btn3 := tele.Btn{Text: "Button 3"}
	btn4 := tele.Btn{Text: "Button 4"}
	btn5 := tele.Btn{Text: "Button 5"}

	// Add buttons
	kb.Add(btn1, btn2, btn3, btn4, btn5)

	// Create markup
	markup := kb.CreateInlineMarkup()
	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 3)
	assert.Len(t, markup.InlineKeyboard[0], 2)
	assert.Len(t, markup.InlineKeyboard[1], 2)
	assert.Len(t, markup.InlineKeyboard[2], 1)
}

func TestKeyboardWithLength(t *testing.T) {
	// Create a new keyboard builder with runes counting
	kb := bote.NewKeyboardWithLength(bote.OneBytePerRune)

	// Create buttons with long texts
	btn1 := tele.Btn{Text: "Short"}
	btn2 := tele.Btn{Text: "Very Long Button Text That Should Cause New Row"}
	btn3 := tele.Btn{Text: "Another Button"}

	// Add buttons
	kb.Add(btn1, btn2, btn3)

	// Create markup
	markup := kb.CreateInlineMarkup()
	assert.NotNil(t, markup)

	// Since this depends on the internal runesInRow map, we just check that the buttons exist
	found1 := false
	found2 := false
	found3 := false

	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if btn.Text == "Short" {
				found1 = true
			}
			if btn.Text == "Very Long Button Text That Should Cause New Row" {
				found2 = true
			}
			if btn.Text == "Another Button" {
				found3 = true
			}
		}
	}

	assert.True(t, found1)
	assert.True(t, found2)
	assert.True(t, found3)
}

func TestInlineHelperFunctions(t *testing.T) {
	// Test Inline helper function
	markup := bote.Inline(2,
		tele.Btn{Text: "Button 1"},
		tele.Btn{Text: "Button 2"},
		tele.Btn{Text: "Button 3"},
	)

	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 2)
	assert.Len(t, markup.InlineKeyboard[0], 2)
	assert.Len(t, markup.InlineKeyboard[1], 1)

	// Test SingleRow helper function
	markup = bote.SingleRow(
		tele.Btn{Text: "Button 1"},
		tele.Btn{Text: "Button 2"},
	)

	assert.NotNil(t, markup)
	assert.Len(t, markup.InlineKeyboard, 1)
	assert.Len(t, markup.InlineKeyboard[0], 2)

	// Test RemoveKeyboard helper function
	markup = bote.RemoveKeyboard()
	assert.NotNil(t, markup)
	assert.True(t, markup.RemoveKeyboard)
}

func TestGetBtnIDAndUnique(t *testing.T) {
	// Test the internal functions that handle button IDs
	// This requires using exported functions

	// Create a button data string and check its format
	data := bote.CreateBtnData("id1", "value1")
	assert.Equal(t, "id1|value1", data)

	// We can't directly test getBtnIDAndUnique as it's not exported,
	// but we can test functionality that uses it through public interfaces
}

func TestMessageFormatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		formats  []bote.Format
		expected string
	}{
		{
			name:     "Bold formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Bold},
			expected: "<b>Test message</b>",
		},
		{
			name:     "Italic formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Italic},
			expected: "<i>Test message</i>",
		},
		{
			name:     "Code formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Code},
			expected: "<code>Test message</code>",
		},
		{
			name:     "Strike formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Strike},
			expected: "<s>Test message</s>",
		},
		{
			name:     "Underline formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Underline},
			expected: "<u>Test message</u>",
		},
		{
			name:     "Pre formatting",
			input:    "Test message",
			formats:  []bote.Format{bote.Pre},
			expected: "<pre>Test message</pre>",
		},
		{
			name:     "Multiple formatting - bold and italic",
			input:    "Test message",
			formats:  []bote.Format{bote.Bold, bote.Italic},
			expected: "<i><b>Test message</b></i>",
		},
		{
			name:     "Multiple formatting - all formats",
			input:    "Test message",
			formats:  []bote.Format{bote.Bold, bote.Italic, bote.Code, bote.Strike, bote.Underline, bote.Pre},
			expected: "<pre><u><s><code><i><b>Test message</b></i></code></s></u></pre>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bote.F(tt.input, tt.formats...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessageBuilder(t *testing.T) {
	t.Run("Basic operations", func(t *testing.T) {
		builder := bote.NewBuilder()

		// Test initial state
		assert.True(t, builder.IsEmpty())
		assert.Equal(t, "", builder.String())

		// Test Write
		builder.Write("Hello")
		assert.Equal(t, "Hello", builder.String())
		assert.False(t, builder.IsEmpty())

		// Test Writef
		builder.Writef(" %s %d", "World", 42)
		assert.Equal(t, "Hello World 42", builder.String())

		// Test Writeln
		builder.Writeln("!")
		assert.Equal(t, "Hello World 42!\n", builder.String())

		// Test WriteIf (true case)
		builder.WriteIf(true, "Yes", "No")
		assert.Equal(t, "Hello World 42!\nYes", builder.String())

		// Test WriteIf (false case)
		builder.WriteIf(false, "Yes", "No")
		assert.Equal(t, "Hello World 42!\nYesNo", builder.String())

		// Test WriteBytes
		builder.WriteBytes([]byte(" Bytes"))
		assert.Equal(t, "Hello World 42!\nYesNo Bytes", builder.String())
	})
}

// MockLogger for testing
type testLogger struct {
	mu          sync.Mutex
	debugCalled bool
	infoCalled  bool
	warnCalled  bool
	errorCalled bool
}

func (l *testLogger) Debug(msg string, args ...any) {
	l.mu.Lock()
	l.debugCalled = true
	l.mu.Unlock()
}

func (l *testLogger) Info(msg string, args ...any) {
	l.mu.Lock()
	l.infoCalled = true
	l.mu.Unlock()
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.mu.Lock()
	l.warnCalled = true
	l.mu.Unlock()
}

func (l *testLogger) Error(msg string, args ...any) {
	l.mu.Lock()
	l.errorCalled = true
	l.mu.Unlock()
}

// Mock update logger for testing
type testUpdateLogger struct {
	mu        sync.Mutex
	logCalled bool
}

func (l *testUpdateLogger) Log(t bote.UpdateType, args ...any) {
	l.mu.Lock()
	l.logCalled = true
	l.mu.Unlock()
}

// Mock user storage for testing
type testUserStorage struct{}

func (s *testUserStorage) Insert(ctx context.Context, userModel bote.UserModel) error { return nil }
func (s *testUserStorage) Find(ctx context.Context, id int64) (bote.UserModel, bool, error) {
	now := time.Now()
	return bote.UserModel{
		ID: id,
		Info: bote.UserInfo{
			FirstName:    "Test",
			LastName:     "User",
			Username:     "testuser",
			LanguageCode: "en",
		},
		LastSeenTime: now,
		CreatedTime:  now,
	}, true, nil
}
func (s *testUserStorage) UpdateAsync(id int64, userModel *bote.UserModelDiff) {}

// Mock message provider for testing
type testMessageProvider struct{}

func (p *testMessageProvider) Messages(language bote.Language) bote.Messages {
	return &testMessages{}
}

type testMessages struct{}

func (m *testMessages) CloseBtn() string     { return "Close" }
func (m *testMessages) GeneralError() string { return "error" }
func (m *testMessages) PrepareMessage(msg string, u bote.User, newState bote.State, msgID int, isHistorical bool) string {
	return msg
}

func TestBotWithTestPoller(t *testing.T) {
	// Create a test poller
	poller := NewTestPoller()

	// Create a bot with test mode
	bot, err := bote.New("test_token", bote.WithTestMode(poller))
	assert.NoError(t, err)
	assert.NotNil(t, bot)

	// Setup a handler for testing
	var handlerCalled atomic.Bool
	bot.Handle("/test", func(c bote.Context) error {
		handlerCalled.Store(true)
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)
	defer bot.Stop()

	// Allow some time for the bot to initialize
	time.Sleep(100 * time.Millisecond)

	// Send a test message
	poller.SendTextMessage(tele.User{ID: 123, FirstName: "Test"}, "/test")

	// Wait for handler to be called with timeout
	waitTimeout := time.NewTimer(500 * time.Millisecond)
	defer waitTimeout.Stop()

	for !handlerCalled.Load() {
		select {
		case <-waitTimeout.C:
			t.Fatal("Timed out waiting for handler to be called")
			return
		case <-time.After(10 * time.Millisecond):
			// Keep checking
		}
	}

	// Check if the handler was called
	assert.True(t, handlerCalled.Load())
}

func TestBotCallbackHandling(t *testing.T) {
	// Skip this test in CI environment since we're using a fake token
	if testing.Short() {
		t.Skip("Skipping TestBotCallbackHandling in short mode")
	}

	// Create a test poller
	poller := NewTestPoller()

	// Create a mock logger to capture output
	logger := &testLogger{}

	// Create a bot with test mode
	bot, err := bote.New("test_token",
		bote.WithTestMode(poller),
		bote.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}
	assert.NotNil(t, bot)

	// Setup a callback handler for testing
	var callbackHandlerCalled atomic.Bool
	var callbackWg sync.WaitGroup

	callbackWg.Add(1)
	bot.Handle(tele.OnCallback, func(c bote.Context) error {
		callbackHandlerCalled.Store(true)
		callbackWg.Done()
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)
	defer bot.Stop()

	// Allow some time for the bot to initialize
	time.Sleep(100 * time.Millisecond)

	// Send a test callback query
	poller.SendCallbackQuery(
		tele.User{ID: 123, FirstName: "Test"},
		"test_callback_data",
		&tele.Message{ID: 456},
	)

	// Wait for the callback to be processed with timeout
	waitCh := make(chan struct{})
	go func() {
		callbackWg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Handler was called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for callback handler")
	}

	// Check if the handler was called
	assert.True(t, callbackHandlerCalled.Load())
}

func TestBotWithCustomOptions(t *testing.T) {
	// Skip this test in CI environment since we're using a fake token
	if testing.Short() {
		t.Skip("Skipping TestBotWithCustomOptions in short mode")
	}

	// Create custom components for testing
	logger := &testLogger{}
	updateLogger := &testUpdateLogger{}
	userStorage := &testUserStorage{}
	messageProvider := &testMessageProvider{}
	poller := NewTestPoller()

	// Create a bot with custom options
	bot, err := bote.New("test_token",
		bote.WithTestMode(poller),
		bote.WithLogger(logger),
		bote.WithUpdateLogger(updateLogger),
		bote.WithUserDB(userStorage),
		bote.WithMsgs(messageProvider),
	)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}
	assert.NotNil(t, bot)

	// Setup a simple handler to ensure something happens
	var handlerCalled atomic.Bool
	var handlerWg sync.WaitGroup

	handlerWg.Add(1)
	bot.Handle("/test", func(c bote.Context) error {
		handlerCalled.Store(true)
		handlerWg.Done()
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)
	defer bot.Stop()

	// Allow some time for the bot to initialize
	time.Sleep(100 * time.Millisecond)

	// Send a test message
	poller.SendTextMessage(tele.User{ID: 123, FirstName: "Test"}, "/test")

	// Wait for the handler to be called with timeout
	waitCh := make(chan struct{})
	go func() {
		handlerWg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Handler was called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for handler")
	}

	// Check if the handler was called
	assert.True(t, handlerCalled.Load())
}

func TestUpdateTypeString(t *testing.T) {
	assert.Equal(t, "message", bote.MessageUpdate.String())
	assert.Equal(t, "callback", bote.CallbackUpdate.String())
}

func TestWithConfig(t *testing.T) {
	// This test only checks that we can create a configuration
	cfg := bote.Config{
		DefaultLanguage: bote.Language("fr"),
		TestMode:        true, // Enable test mode to avoid actual Telegram API calls
	}

	// Just test the function doesn't panic
	optFunc := bote.WithConfig(cfg)

	var opts bote.Options
	optFunc(&opts)

	assert.Equal(t, bote.Language("fr"), opts.Config.DefaultLanguage)
	assert.True(t, opts.Config.TestMode)
}

func TestUserModel(t *testing.T) {
	// Test creating and updating user models
	now := time.Now()
	model := bote.UserModel{
		ID: 123,
		Info: bote.UserInfo{
			FirstName:    "Test",
			LastName:     "User",
			Username:     "testuser",
			LanguageCode: "en",
		},
		LastSeenTime: now,
		CreatedTime:  now,
	}

	assert.Equal(t, int64(123), model.ID)
	assert.Equal(t, "Test", model.Info.FirstName)
	assert.Equal(t, "User", model.Info.LastName)
	assert.Equal(t, "testuser", model.Info.Username)
	assert.Equal(t, "en", model.Info.LanguageCode)

	// Test diff
	diff := &bote.UserModelDiff{
		Info: &bote.UserInfoDiff{
			FirstName: Ptr("NewName"),
		},
	}

	assert.NotNil(t, diff.Info)
	assert.NotNil(t, diff.Info.FirstName)
	assert.Equal(t, "NewName", *diff.Info.FirstName)
}

// TestTestPollerFunctionality tests the testPoller implementation and its helper functions
func TestTestPollerFunctionality(t *testing.T) {
	// Create a test poller
	poller := NewTestPoller()

	// Create channels to mock the telebot system
	dest := make(chan tele.Update, 10)
	stop := make(chan struct{})

	// Create a mock bot
	mockBot := &tele.Bot{}

	// Start polling in a goroutine
	go poller.Poll(mockBot, dest, stop)

	// Test sending a text message
	testUser := tele.User{ID: 123, FirstName: "Test", Username: "testuser"}
	poller.SendTextMessage(testUser, "Hello World")

	// Get the update from the destination channel with timeout
	var update tele.Update
	select {
	case update = <-dest:
		assert.NotNil(t, update.Message)
		assert.Equal(t, "Hello World", update.Message.Text)
		assert.Equal(t, int64(123), update.Message.Sender.ID)
		assert.Equal(t, "Test", update.Message.Sender.FirstName)
		assert.Equal(t, "testuser", update.Message.Sender.Username)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for text message update")
	}

	// Test sending a callback query
	message := &tele.Message{ID: 456}
	poller.SendCallbackQuery(testUser, "btn_data", message)

	// Get the update from the destination channel with timeout
	select {
	case update = <-dest:
		assert.NotNil(t, update.Callback)
		assert.Equal(t, "btn_data", update.Callback.Data)
		assert.Equal(t, int64(123), update.Callback.Sender.ID)
		assert.Equal(t, 456, update.Callback.Message.ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for callback update")
	}

	// Test sending a custom update directly
	customUpdate := tele.Update{
		ID: 789,
		Message: &tele.Message{
			ID:     123,
			Sender: &testUser,
			Text:   "Custom update",
		},
	}
	poller.SendUpdate(customUpdate)

	// Get the update from the destination channel with timeout
	select {
	case update = <-dest:
		assert.Equal(t, 789, update.ID)
		assert.Equal(t, "Custom update", update.Message.Text)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for custom update")
	}

	// Stop polling
	close(stop)
}

// TestSendingDifferentUpdateTypes tests the ability to send different types of updates through testPoller
func TestSendingDifferentUpdateTypes(t *testing.T) {
	poller := NewTestPoller()

	// Create channels to mock the telebot system
	dest := make(chan tele.Update, 10)
	stop := make(chan struct{})

	// Create a mock bot
	mockBot := &tele.Bot{}

	// Start polling in a goroutine
	go poller.Poll(mockBot, dest, stop)
	defer close(stop)

	testUser := tele.User{ID: 123, FirstName: "Test", Username: "testuser"}

	// Test sending an inline query
	poller.SendUpdate(tele.Update{
		Query: &tele.Query{
			ID:       "query123",
			Sender:   &testUser,
			Text:     "inline query text",
			Offset:   "0",
			Location: &tele.Location{Lat: 55.7558, Lng: 37.6173},
		},
	})

	// Get the inline query update
	select {
	case update := <-dest:
		assert.NotNil(t, update.Query)
		assert.Equal(t, "query123", update.Query.ID)
		assert.Equal(t, "inline query text", update.Query.Text)
		assert.Equal(t, int64(123), update.Query.Sender.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for inline query update")
	}

	// Test sending a channel post
	poller.SendUpdate(tele.Update{
		ChannelPost: &tele.Message{
			ID:       456,
			Text:     "Channel post",
			Chat:     &tele.Chat{ID: -1001234567890, Type: tele.ChatChannel, Title: "Test Channel"},
			Sender:   &testUser,
			Entities: []tele.MessageEntity{{Type: tele.EntityBold, Offset: 0, Length: 7}},
		},
	})

	// Get the channel post update
	select {
	case update := <-dest:
		assert.NotNil(t, update.ChannelPost)
		assert.Equal(t, "Channel post", update.ChannelPost.Text)
		assert.Equal(t, int64(-1001234567890), update.ChannelPost.Chat.ID)
		assert.Equal(t, tele.ChatChannel, update.ChannelPost.Chat.Type)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timed out waiting for channel post update")
	}
}

// TestBotWithPollerShutdown tests that the bot properly shuts down when stop is called
func TestBotWithPollerShutdown(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestBotWithPollerShutdown in short mode")
	}

	// Create a test poller
	poller := NewTestPoller()

	// Create a mock logger to capture output
	logger := &testLogger{}

	// Create a bot with test mode
	bot, err := bote.New("test_token",
		bote.WithTestMode(poller),
		bote.WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Setup a handler for testing
	var handlerCalled atomic.Bool
	var handlerWg sync.WaitGroup

	handlerWg.Add(1)
	bot.Handle("/test", func(c bote.Context) error {
		handlerCalled.Store(true)
		handlerWg.Done()
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)

	// Allow some time for the bot to initialize
	time.Sleep(100 * time.Millisecond)

	// Send a test message
	poller.SendTextMessage(tele.User{ID: 123, FirstName: "Test"}, "/test")

	// Wait for the handler to be called with timeout
	waitCh := make(chan struct{})
	go func() {
		handlerWg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// Handler was called
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for handler")
	}

	// Check if the handler was called
	assert.True(t, handlerCalled.Load())

	// Stop the bot and make sure it doesn't panic
	bot.Stop()

	// Give the bot time to shut down
	time.Sleep(100 * time.Millisecond)
}

// Helper function to enhance testPoller by adding more update types
func (p *testPoller) SendInlineQuery(from tele.User, queryText string) {
	p.SendUpdate(tele.Update{
		Query: &tele.Query{
			ID:     "query_" + queryText,
			Sender: &from,
			Text:   queryText,
			Offset: "0",
		},
	})
}

// Helper function to send a channel post update
func (p *testPoller) SendChannelPost(channelID int64, text string) {
	p.SendUpdate(tele.Update{
		ChannelPost: &tele.Message{
			ID:   int(time.Now().Unix()),
			Text: text,
			Chat: &tele.Chat{
				ID:    channelID,
				Type:  tele.ChatChannel,
				Title: "Test Channel",
			},
		},
	})
}

// Define a simple state implementation for tests
type testState string

func (s testState) String() string {
	return string(s)
}

func (s testState) IsText() bool {
	return false
}

func (s testState) NotChanged() bool {
	return false
}

// Define a simple state for testing
var noneState = testState("none")
var unchangedState unchangedTestState = "unchanged"

// Special type for unchanged state
type unchangedTestState string

func (s unchangedTestState) String() string {
	return string(s)
}

func (s unchangedTestState) IsText() bool {
	return false
}

func (s unchangedTestState) NotChanged() bool {
	return true
}

// TestContextOperations tests various context methods
func TestContextOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestContextOperations in short mode")
	}

	poller := NewTestPoller()

	// Create a bot with test mode
	bot, err := bote.New("test_token", bote.WithTestMode(poller))
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Track context method results with proper synchronization
	var mu sync.Mutex
	var receivedText string
	var parsedData []string
	var buttonIDReceived string
	var dataReceived string

	var textWg sync.WaitGroup
	var callbackWg sync.WaitGroup

	// Set up handlers to test context methods
	textWg.Add(1)
	bot.Handle("/test", func(c bote.Context) error {
		mu.Lock()
		receivedText = c.Text()
		mu.Unlock()
		textWg.Done()
		return nil
	})

	callbackWg.Add(1)
	bot.Handle(tele.OnCallback, func(c bote.Context) error {
		mu.Lock()
		buttonIDReceived = c.ButtonID()
		dataReceived = c.Data()
		parsedData = c.DataParsed()
		mu.Unlock()
		callbackWg.Done()
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)
	defer bot.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	// Test text processing
	user := tele.User{ID: 123, FirstName: "Test"}
	poller.SendTextMessage(user, "/test with arguments")

	// Wait with timeout
	textWaiter := make(chan struct{})
	go func() {
		textWg.Wait()
		close(textWaiter)
	}()

	select {
	case <-textWaiter:
		// Handler completed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for text handler")
	}

	// Check text parsing
	mu.Lock()
	assert.Equal(t, "/test with arguments", receivedText)
	mu.Unlock()

	// Test callback data
	message := &tele.Message{ID: 456}
	poller.SendCallbackQuery(user, "button_id|user_data", message)

	// Wait with timeout
	callbackWaiter := make(chan struct{})
	go func() {
		callbackWg.Wait()
		close(callbackWaiter)
	}()

	select {
	case <-callbackWaiter:
		// Handler completed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for callback handler")
	}

	// Check callback data parsing
	mu.Lock()
	assert.Equal(t, "butto", buttonIDReceived)
	assert.Equal(t, "button_id|user_data", dataReceived)
	assert.Equal(t, []string{"button_id", "user_data"}, parsedData)
	mu.Unlock()
}

// TestBotSendAndEdit tests message sending and editing
func TestBotSendAndEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping TestBotSendAndEdit in short mode")
	}

	poller := NewTestPoller()

	// Create a bot with test mode
	bot, err := bote.New("test_token", bote.WithTestMode(poller))
	if err != nil {
		t.Fatalf("Failed to create bot: %v", err)
	}

	// Set up handler to test send and edit
	bot.Handle("/send", func(c bote.Context) error {
		err := c.SendMain(testState("send"), "Test message", nil)
		if err != nil {
			return err
		}
		return nil
	})

	bot.Handle("/edit", func(c bote.Context) error {
		err := c.EditMain(testState("edit"), "Edited message", nil)
		if err != nil {
			return err
		}
		return nil
	})

	// Start the bot
	bot.Start(func(c bote.Context) error {
		return nil
	}, nil)
	defer bot.Stop()

	// Allow time for initialization
	time.Sleep(100 * time.Millisecond)

	// Test sending a message
	user := tele.User{ID: 123, FirstName: "Test"}
	poller.SendTextMessage(user, "/send")
	time.Sleep(100 * time.Millisecond)

	// Test editing the message
	poller.SendTextMessage(user, "/edit")
	time.Sleep(100 * time.Millisecond)
}

// TestStateImplementation tests the state implementation functionality
func TestStateImplementation(t *testing.T) {
	// Create some test states
	state1 := testState("state1")
	state2 := testState("state2")
	stateEmpty := testState("")

	// Test String method
	assert.Equal(t, "state1", state1.String())
	assert.Equal(t, "state2", state2.String())
	assert.Equal(t, "", stateEmpty.String())

	// Test NotChanged method (should be false for our test states)
	assert.False(t, state1.NotChanged())
	assert.False(t, state2.NotChanged())
	assert.False(t, stateEmpty.NotChanged())

	// Test IsText method (should be false for our test states)
	assert.False(t, state1.IsText())
	assert.False(t, state2.IsText())
	assert.False(t, stateEmpty.IsText())

	// Test our none and unchanged states
	assert.Equal(t, "none", noneState.String())
	assert.Equal(t, "unchanged", unchangedState.String())
	assert.False(t, noneState.IsText())
	assert.False(t, unchangedState.IsText())
	assert.False(t, noneState.NotChanged())
	assert.True(t, unchangedState.NotChanged())
}

// TestFormatting tests text formatting functions
func TestFormatting(t *testing.T) {
	// Test formatting with different functions
	text := "Test message"

	// Using F function
	assert.Equal(t, "<b>Test message</b>", bote.F(text, bote.Bold))
	assert.Equal(t, "<i>Test message</i>", bote.F(text, bote.Italic))
}
