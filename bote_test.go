package bote_test

import (
	"context"
	"testing"
	"time"

	"github.com/maxbolgarin/bote"
	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
)

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

// Custom logger for testing
type testLogger struct {
	debugCalled bool
	infoCalled  bool
	warnCalled  bool
	errorCalled bool
}

func (l *testLogger) Debug(msg string, args ...any) { l.debugCalled = true }
func (l *testLogger) Info(msg string, args ...any)  { l.infoCalled = true }
func (l *testLogger) Warn(msg string, args ...any)  { l.warnCalled = true }
func (l *testLogger) Error(msg string, args ...any) { l.errorCalled = true }

// Custom update logger for testing
type testUpdateLogger struct {
	logCalled bool
}

func (l *testUpdateLogger) Log(t bote.UpdateType, args ...any) { l.logCalled = true }

// Custom user storage for testing
type testUserStorage struct{}

func (s *testUserStorage) Insert(ctx context.Context, userModel bote.UserModel) error { return nil }
func (s *testUserStorage) Find(ctx context.Context, id int64) (bote.UserModel, bool, error) {
	return bote.UserModel{}, false, nil
}
func (s *testUserStorage) Update(id int64, userModel *bote.UserModelDiff) {}

// Custom message provider for testing
type testMessageProvider struct{}

func (p *testMessageProvider) Messages(languageCode string) bote.Messages {
	return &testMessages{}
}

type testMessages struct{}

func (m *testMessages) GeneralError() string { return "error" }
func (m *testMessages) FatalError() string   { return "fatal" }
func (m *testMessages) PrepareMessage(msg string, u bote.User, newState bote.State, msgID int, isHistorical bool) string {
	return msg
}

func TestWithConfig(t *testing.T) {
	// Create a custom config
	cfg := bote.Config{
		LPTimeout:           30 * time.Second,
		ParseMode:           tele.ModeMarkdown,
		DefaultLanguageCode: "ru",
		NoPreview:           true,
		Debug:               true,
	}

	// Apply the config option
	var opts bote.Options
	optFunc := bote.WithConfig(cfg)
	optFunc(&opts)

	// Verify the config was set correctly
	assert.Equal(t, 30*time.Second, opts.Config.LPTimeout)
	assert.Equal(t, tele.ModeMarkdown, opts.Config.ParseMode)
	assert.Equal(t, "ru", opts.Config.DefaultLanguageCode)
	assert.True(t, opts.Config.NoPreview)
	assert.True(t, opts.Config.Debug)
}

func TestWithUserDB(t *testing.T) {
	// Create a custom UserDB
	db := &testUserStorage{}

	// Apply the UserDB option
	var opts bote.Options
	optFunc := bote.WithUserDB(db)
	optFunc(&opts)

	// Verify the UserDB was set correctly
	assert.Equal(t, db, opts.UserDB)
}

func TestWithMsgs(t *testing.T) {
	// Create a custom MessageProvider
	msgs := &testMessageProvider{}

	// Apply the MessageProvider option
	var opts bote.Options
	optFunc := bote.WithMsgs(msgs)
	optFunc(&opts)

	// Verify the MessageProvider was set correctly
	assert.Equal(t, msgs, opts.Msgs)
}

func TestWithLogger(t *testing.T) {
	// Create a custom Logger
	logger := &testLogger{}

	// Apply the Logger option
	var opts bote.Options
	optFunc := bote.WithLogger(logger)
	optFunc(&opts)

	// Verify the Logger was set correctly
	assert.Equal(t, logger, opts.Logger)
}

func TestWithUpdateLogger(t *testing.T) {
	// Create a custom UpdateLogger
	updateLogger := &testUpdateLogger{}

	// Apply the UpdateLogger option
	var opts bote.Options
	optFunc := bote.WithUpdateLogger(updateLogger)
	optFunc(&opts)

	// Verify the UpdateLogger was set correctly
	assert.Equal(t, updateLogger, opts.UpdateLogger)
}

func TestUpdateTypeString(t *testing.T) {
	// Test the String() method of UpdateType
	messageType := bote.MessageUpdate
	callbackType := bote.CallbackUpdate

	assert.Equal(t, "message", messageType.String())
	assert.Equal(t, "callback", callbackType.String())
}

// TestUserModel tests various user model functions
func TestUserModel(t *testing.T) {
	// Create a basic user model for testing
	now := time.Now()
	model := bote.UserModel{
		ID: 123456,
		Info: bote.UserInfo{
			FirstName:    "Test",
			LastName:     "User",
			Username:     "testuser",
			LanguageCode: "en",
		},
		LastSeenTime: now,
		CreatedTime:  now,
	}

	// Test basic properties
	assert.Equal(t, int64(123456), model.ID)
	assert.Equal(t, "Test", model.Info.FirstName)
	assert.Equal(t, "User", model.Info.LastName)
	assert.Equal(t, "testuser", model.Info.Username)
	assert.Equal(t, "en", model.Info.LanguageCode)
	assert.Equal(t, now, model.LastSeenTime)
	assert.Equal(t, now, model.CreatedTime)
	assert.False(t, model.IsDisabled)
}
