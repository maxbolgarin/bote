package bote

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// TestBotNew verifies that New creates a bot without error using offline mode.
func TestBotNew(t *testing.T) {
	ctx := context.Background()
	bot, err := New(ctx, "test-token", WithOffline(&mockPoller{}))
	require.NoError(t, err)
	assert.NotNil(t, bot)
}

// TestBotGetAllUsersFromCache verifies cache is non-nil when empty
// and that a user appears after being registered via NewContext.
func TestBotGetAllUsersFromCache(t *testing.T) {
	bot := setupTestBot(t)

	users := bot.GetAllUsersFromCache()
	assert.NotNil(t, users)

	// Create a context for user 1001 to register them in cache.
	_ = NewContext(bot, 1001, 1)

	users = bot.GetAllUsersFromCache()
	assert.NotNil(t, users)

	found := false
	for _, u := range users {
		if u.ID() == 1001 {
			found = true
			break
		}
	}
	assert.True(t, found, "user 1001 should be in cache after NewContext")
}

// TestBotAddMiddleware verifies AddMiddleware does not panic.
func TestBotAddMiddleware(t *testing.T) {
	bot := setupTestBot(t)

	assert.NotPanics(t, func() {
		bot.AddMiddleware(func(upd *tele.Update) bool { return true })
	})
}

// TestBotSetTextHandler verifies SetTextHandler does not panic.
func TestBotSetTextHandler(t *testing.T) {
	bot := setupTestBot(t)

	assert.NotPanics(t, func() {
		bot.SetTextHandler(func(ctx Context) error { return nil })
	})
}

// TestBotSetMessageProvider verifies SetMessageProvider does not panic.
func TestBotSetMessageProvider(t *testing.T) {
	bot := setupTestBot(t)

	assert.NotPanics(t, func() {
		bot.SetMessageProvider(testProvider{})
	})
}

// TestBotSendInChatValidation verifies early-return behaviour for invalid inputs.
func TestBotSendInChatValidation(t *testing.T) {
	bot := setupTestBot(t)

	// chatID == 0 should return (0, nil) without calling Telegram.
	msgID, err := bot.SendInChat(0, 0, "hello", nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, msgID)

	// empty message should return (0, nil).
	msgID, err = bot.SendInChat(12345, 0, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, msgID)
}

// TestBotEditInChatValidation verifies early-return behaviour for invalid inputs.
func TestBotEditInChatValidation(t *testing.T) {
	bot := setupTestBot(t)

	// chatID == 0 should return nil.
	err := bot.EditInChat(0, 1, "hello", nil)
	assert.NoError(t, err)

	// msgID == 0 should return nil.
	err = bot.EditInChat(12345, 0, "hello", nil)
	assert.NoError(t, err)

	// empty message should return nil.
	err = bot.EditInChat(12345, 1, "", nil)
	assert.NoError(t, err)
}

// TestBotDeleteInChatValidation verifies early-return behaviour for invalid inputs.
func TestBotDeleteInChatValidation(t *testing.T) {
	bot := setupTestBot(t)

	// chatID == 0 should return nil.
	err := bot.DeleteInChat(0, 1)
	assert.NoError(t, err)

	// msgID == 0 should return nil.
	err = bot.DeleteInChat(12345, 0)
	assert.NoError(t, err)
}

// TestBotCreateUserFromModel verifies a user is created with the correct ID.
func TestBotCreateUserFromModel(t *testing.T) {
	bot := setupTestBot(t)

	model := UserModel{}
	model.ID = NewPlainUserID(9999)

	user := bot.CreateUserFromModel(model, false)
	require.NotNil(t, user)
	assert.Equal(t, int64(9999), user.ID())
}

// TestBotGetUserID verifies GetUserID returns the plain telegram ID.
func TestBotGetUserID(t *testing.T) {
	bot := setupTestBot(t)

	id, err := bot.GetUserID(NewPlainUserID(12345))
	require.NoError(t, err)
	assert.Equal(t, int64(12345), id)
}
