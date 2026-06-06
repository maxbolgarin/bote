package bote

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// dispatchCallback builds a callback update as it would arrive from a non-private chat
// (channel / group / untracked admin) and runs it through callbackFallbackHandler with a
// public user stub — exactly the path a service bot exercises.
//
// It replicates the REAL Telegram wire format: the data field is "\f<unique>|<payload>" and the
// Unique field is empty (Telegram does not pre-split it for the generic OnCallback handler). This
// is what a button built with NewButton actually produces when tapped.
func dispatchCallback(bot *Bot, btn tele.Btn, chatType tele.ChatType) error {
	upd := tele.Update{
		Callback: &tele.Callback{
			Sender: &tele.User{ID: 555},
			Message: &tele.Message{
				ID:   4242,
				Chat: &tele.Chat{ID: -100123, Type: chatType},
			},
			Data: "\f" + btn.Unique + "|" + btn.Data,
		},
	}
	impl := &contextImpl{
		bt:   bot,
		ct:   bot.bot.tbot.NewContext(upd),
		user: newPublicUserContext(&tele.User{ID: 555}),
	}
	return bot.callbackFallbackHandler(impl)
}

// TestRegisterButtonDispatchesToServiceRouter verifies that a tap on a button built with
// NewButton routes to the handler registered with RegisterButton, with the live payload.
func TestRegisterButtonDispatchesToServiceRouter(t *testing.T) {
	bot := setupTestBot(t)

	var gotData string
	called := 0
	bot.RegisterButton("approve", func(ctx Context) error {
		called++
		gotData = ctx.Data()
		return nil
	})

	btn := bot.NewButton("approve", "42")
	err := dispatchCallback(bot, btn, tele.ChatChannel)

	require.NoError(t, err)
	assert.Equal(t, 1, called, "registered handler should be called once")
	assert.Equal(t, "42", gotData, "handler should see the live per-message payload")
}

// TestServiceRouterMultiplePayloadItems verifies multi-item payloads survive the round-trip.
func TestServiceRouterMultiplePayloadItems(t *testing.T) {
	bot := setupTestBot(t)

	var parsed []string
	bot.RegisterButton("cancel", func(ctx Context) error {
		parsed = ctx.DataParsed()
		return nil
	})

	btn := bot.NewButton("cancel", "post", "99")
	require.NoError(t, dispatchCallback(bot, btn, tele.ChatSuperGroup))
	assert.Equal(t, []string{"post", "99"}, parsed)
}

// TestServiceRouterUnregisteredIsNoop verifies an unregistered button does not panic on a
// public user (whose buttonMap is nil) and simply does nothing.
func TestServiceRouterUnregisteredIsNoop(t *testing.T) {
	bot := setupTestBot(t)

	btn := bot.NewButton("never_registered", "1")
	assert.NotPanics(t, func() {
		err := dispatchCallback(bot, btn, tele.ChatChannel)
		assert.NoError(t, err)
	})
}

// TestNewButtonRoundTripsButtonID verifies the button id encoded in a NewButton's Unique maps
// back to the same id RegisterButton keys on.
func TestNewButtonRoundTripsButtonID(t *testing.T) {
	bot := setupTestBot(t)

	btn := bot.NewButton("publish", "7")
	assert.Equal(t, "publish", getNameFromUnique(btn.Unique), "name should decode from unique")
	assert.Equal(t, "7", btn.Data)

	regID, _ := getBtnIDAndUnique("publish")
	assert.Equal(t, regID, getIDFromUnique(btn.Unique), "router key must match the tapped button id")
}

// TestNewButtonTruncatesOverlongPayload verifies the payload is clamped under the callback-data
// limit at a rune boundary (no panic, no split runes).
func TestNewButtonTruncatesOverlongPayload(t *testing.T) {
	bot := setupTestBot(t)

	long := strings.Repeat("é", 80) // 2 bytes per rune → well over the limit
	btn := bot.NewButton("x", long)
	assert.LessOrEqual(t, len(btn.Unique)+len(btn.Data)+2, MaxDataLengthBytes)
	assert.True(t, len(btn.Data) > 0)
}

// TestKeyboardAddURL verifies URL buttons end up in the inline markup with no callback unique.
func TestKeyboardAddURL(t *testing.T) {
	kb := NewKeyboard()
	kb.AddURL("View source", "https://doi.org/10.1/abc")
	kb.AddURLRow("Open app", "https://app.example.io")

	markup := kb.CreateInlineMarkup()
	var urls []string
	for _, row := range markup.InlineKeyboard {
		for _, b := range row {
			if b.URL != "" {
				urls = append(urls, b.URL)
			}
		}
	}
	assert.ElementsMatch(t, []string{"https://doi.org/10.1/abc", "https://app.example.io"}, urls)
}
