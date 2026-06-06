package bote

import (
	tele "gopkg.in/telebot.v4"
)

// This file contains helpers for "service" bots: bots that post to channels and handle
// inline-button taps from channels, groups, or admin chats where there is no per-user
// conversation (and therefore no tracked [User] / per-message buttonMap).
//
// Such bots register one handler per button *action* once via [Bot.RegisterButton] and then
// build any number of buttons for that action via [Bot.NewButton], carrying a per-message
// payload in the button data. When a button is tapped, the tap is routed to the registered
// handler by the button's stable ID (derived from its name), regardless of who tapped it or
// in which chat — see callbackFallbackHandler. The live payload is available in the handler
// via [Context.Data] / [Context.DataParsed].
//
// Example:
//
//	bot.RegisterButton("approve", func(ctx bote.Context) error {
//	    postID := ctx.Data() // the payload passed to NewButton when the message was sent
//	    // ...act on postID...
//	    return nil
//	})
//
//	kb := bote.SingleRow(
//	    bot.NewButton("approve", postID),
//	    bot.NewButton("reject", postID),
//	)
//	msgID, _ := bot.SendInChat(adminChatID, 0, draft, kb)

// RegisterButton registers a stateless handler for an inline button action identified by name.
// It should be called once per action (e.g. at startup). Every button created with
// [Bot.NewButton] using the same name routes to this handler when tapped, no matter the chat
// type or whether the tapper is a tracked user. The per-message payload is delivered to the
// handler through [Context.Data] / [Context.DataParsed].
//
// RegisterButton complements the per-user keyboard flow ([Context.Btn]); it does not replace
// it. A tracked user's buttonMap is still consulted first, so service buttons and per-user
// buttons can coexist as long as their names differ.
func (b *Bot) RegisterButton(name string, handler HandlerFunc) {
	id, _ := getBtnIDAndUnique(name)
	b.callbackRouter.Set(id, handler)
}

// NewButton builds an inline button for an action previously registered with
// [Bot.RegisterButton]. The optional data items become the button's callback payload
// (joined by '|', readable via [Context.Data] / [Context.DataParsed] in the handler).
//
// Unlike [Context.Btn], NewButton needs no [Context] or tracked user, so it is suitable for
// channel/admin messages. Build a fresh button per message with the payload identifying that
// message's subject (e.g. a database row id). Overlong payloads are truncated at a rune
// boundary to stay within Telegram's 64-byte callback-data limit.
func (b *Bot) NewButton(name string, data ...string) tele.Btn {
	_, unique := getBtnIDAndUnique(name)
	payload := CreateBtnData(data...)
	maxDataLength := MaxDataLengthBytes - len(unique) - 2
	if len(payload) > maxDataLength {
		b.bot.log.Warn("button data length exceeds limit, it will be truncated", "length", len(payload))
		runes := []rune(payload)
		for i := len(runes); i > 0; i-- {
			if truncated := string(runes[:i]); len(truncated) <= maxDataLength {
				payload = truncated
				break
			}
		}
	}
	return tele.Btn{
		Text:   name,
		Unique: unique,
		Data:   payload,
	}
}

// AddURL adds an inline URL button to the current row. URL buttons open a link and produce no
// callback, so they need no handler — handy for "View source" / deep-link CTAs in channel posts.
func (k *Keyboard) AddURL(text, url string) *Keyboard {
	return k.Add(tele.Btn{Text: text, URL: url})
}

// AddURLRow adds an inline URL button as a new row.
func (k *Keyboard) AddURLRow(text, url string) *Keyboard {
	return k.AddRow(tele.Btn{Text: text, URL: url})
}
