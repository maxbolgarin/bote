package bote

import (
	"strings"

	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// Context is an interface that provides to every handler.
type Context interface {
	// User returns current User context.
	User() User

	// Tele returns underlying telebot context.
	Tele() tele.Context

	// Data returns button data. If there are many items in data, they will be separated by '|'.
	Data() string

	// DataParsed returns all items of button data.
	DataParsed() []string

	// DataDouble returns two first items of button data.
	DataDouble() (string, string)

	// Text returns the message text if is available.
	Text() string

	// Set sets custom data for the current context.
	Set(key, value string)

	// Get returns custom data from the current context.
	Get(key string) string

	// Send sends new main and head messages to the user.
	// Old head message will be deleted. Old main message will becomve historical.
	// newState is a state of the user which will be set after sending message.
	// opts are additional options for sending messages.
	Send(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error)

	// SendMain sends new main message to the user.
	// Old head message will be deleted. Old main message will becomve historical.
	// newState is a state of the user which will be set after sending message.
	// opts are additional options for sending message.
	SendMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// SendNotification sends a notification message to the user.
	// opts are additional options for sending message.
	SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error

	// SendError sends an error message to the user.
	// opts are additional options for sending message.
	SendError(msg string, opts ...any) error

	// Edit edits main and head messages of the user.
	// newState is a state of the user which will be set after editing message.
	// opts are additional options for editing messages.
	Edit(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error)

	// EditMain edits main message of the user.
	// newState is a state of the user which will be set after editing message.
	// opts are additional options for editing message.
	EditMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditAny edits any message of the user.
	// newState is a state of the user which will be set after editing message.
	// msgID is the ID of the message to edit.
	// opts are additional options for editing message.
	EditAny(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHead edits head message of the user.
	// opts are additional options for editing message.
	EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHeadReplyMarkup edits reply markup of the head message.
	// opts are additional options for editing message.
	EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	// Delete deletes messages of the user by their IDs.
	Delete(msgIDs ...int) error

	// DeleteHistory deletes all messages of the user from the specified ID.
	DeleteHistory(lastMessageID int)
}

func (b *Bot) newContext(c tele.Context) *contextImpl {
	return &contextImpl{
		bt: b,
		ct: c,
	}
}

type contextImpl struct {
	bt *Bot
	ct tele.Context
}

func (c *contextImpl) User() User {
	upd := c.ct.Update()
	return c.bt.um.getUser(getSender(&upd).ID)
}

func (c *contextImpl) Tele() tele.Context {
	return c.ct
}

func (c *contextImpl) Data() string {
	if cb := c.ct.Callback(); cb != nil {
		return cb.Data
	}
	if msg := c.ct.Message(); msg != nil {
		return msg.Text
	}
	return ""
}

func (c *contextImpl) DataParsed() []string {
	return strings.Split(c.Data(), "|")
}

func (c *contextImpl) DataDouble() (string, string) {
	spl := strings.Split(c.Data(), "|")
	if len(spl) < 2 {
		return spl[0], ""
	}
	return spl[0], spl[1]
}

func (c *contextImpl) Text() string {
	if msg := c.ct.Message(); msg != nil {
		return msg.Text
	}
	return ""
}

func (c *contextImpl) Set(key, value string) {
	c.ct.Set(key, value)
}

func (c *contextImpl) Get(key string) string {
	return c.ct.Get(key).(string)
}

func (c *contextImpl) Send(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error) {
	if mainMsg == "" {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}
	user := c.User()

	mainMsg = c.bt.msgs.Messages(user.Language()).PrepareMainMessage(mainMsg, user)

	var headMsgID int
	if headMsg != "" {
		// Need copy to prevent from conflict in the next append because of using the same underlying array in opts
		headOpts := lang.If(len(opts) > 0, append(lang.Copy(opts), headKb), []any{headKb})
		headMsgID, err = c.bt.bot.send(user.ID(), headMsg, headOpts...)
		if err != nil {
			return c.handleError(err)
		}
	} else {
		c.bt.bot.log.Warn("empty head message, use SendMain instead", userFields(user)...)
	}

	mainMsgID, err := c.bt.bot.send(user.ID(), mainMsg, append(opts, mainKb)...)
	if err != nil {
		return c.handleError(err)
	}

	if headMsgID := user.Messages().HeadID; headMsgID != 0 {
		if err := c.bt.bot.delete(user.ID(), headMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", userFields(user)...)
		}
	}

	user.HandleSend(newState, mainMsgID, headMsgID)

	return nil
}

func (c *contextImpl) SendMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}
	user := c.User()

	msg = c.bt.msgs.Messages(user.Language()).PrepareMainMessage(msg, user)
	msgID, err := c.bt.bot.send(user.ID(), msg, append(opts, kb)...)
	if err != nil {
		return c.handleError(err)
	}

	if headMsgID := user.Messages().HeadID; headMsgID != 0 {
		if err := c.bt.bot.delete(user.ID(), headMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", userFields(user)...)
		}
	}

	user.HandleSend(newState, msgID, 0)

	return nil
}

func (c *contextImpl) SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" {
		c.bt.bot.log.Error("notification message cannot be empty", userFields(c.User())...)
		return nil
	}
	user := c.User()

	msgID, err := c.bt.bot.send(user.ID(), msg, append(opts, kb)...)
	if err != nil {
		return c.handleError(err)
	}
	user.SetNotificationMessage(msgID)

	return nil
}

func (c *contextImpl) SendError(msg string, opts ...any) error {
	user := c.User()

	msgID, err := c.bt.bot.send(user.ID(), msg, opts...)
	if err != nil {
		c.bt.bot.log.Error("failed to send error message", userFields(user)...)
		return nil
	}
	user.SetErrorMessage(msgID)

	return nil
}

func (c *contextImpl) Edit(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) error {
	if mainMsg == "" && mainKb == nil {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}
	if headMsg == "" && headKb == nil {
		c.bt.bot.log.Warn("empty head message, use EditMain instead", userFields(c.User())...)
	}

	user := c.User()
	msgIDs := user.Messages()

	headOpts := lang.If(len(opts) > 0, append(lang.Copy(opts), headKb), []any{headKb})
	if err := c.edit(msgIDs.HeadID, headMsg, headKb, headOpts...); err != nil {
		return c.handleError(err)
	}

	mainMsg = c.bt.msgs.Messages(user.Language()).PrepareMainMessage(mainMsg, user)
	if err := c.edit(msgIDs.MainID, mainMsg, mainKb, opts...); err != nil {
		return c.handleError(err)
	}

	user.SetState(newState)

	return nil
}

func (c *contextImpl) EditMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}

	user := c.User()
	msgIDs := user.Messages()

	msg = c.bt.msgs.Messages(user.Language()).PrepareMainMessage(msg, user)
	if err := c.edit(msgIDs.MainID, msg, kb, opts...); err != nil {
		return c.handleError(err)
	}

	user.SetState(newState)

	return nil
}

func (c *contextImpl) EditAny(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("message cannot be empty", userFields(c.User())...)
		return nil
	}

	user := c.User()

	if err := c.edit(msgID, msg, kb, opts...); err != nil {
		return c.handleError(err)
	}

	user.SetState(newState)

	return nil
}

func (c *contextImpl) EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("head message cannot be empty", userFields(c.User())...)
		return nil
	}

	user := c.User()
	msgIDs := user.Messages()

	if err := c.edit(msgIDs.HeadID, msg, kb, opts...); err != nil {
		return c.handleError(err)
	}

	return nil
}

func (c *contextImpl) EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error {
	if kb == nil {
		c.bt.bot.log.Error("head keyboard cannot be empty", userFields(c.User())...)
		return nil
	}

	user := c.User()
	msgIDs := user.Messages()

	if err := c.edit(msgIDs.HeadID, "", kb, opts...); err != nil {
		return c.handleError(err)
	}

	return nil
}

func (c *contextImpl) Delete(msgIDs ...int) error {
	if len(msgIDs) == 0 {
		return nil
	}
	if err := c.bt.bot.delete(c.User().ID(), msgIDs...); err != nil {
		return c.handleError(err)
	}
	return nil
}

func (c *contextImpl) DeleteHistory(lastMessageID int) {
	c.bt.bot.deleteHistory(c.User().ID(), lastMessageID)
}

func (c *contextImpl) handleError(err error) error {
	user := c.User()

	if strings.Contains(err.Error(), "bot was blocked by the user") {
		c.bt.bot.log.Info("bot is blocked, disable", userFields(user)...)
		user.Disable()
		c.bt.um.removeUserFromMemory(user.ID())
		return nil
	}

	if strings.Contains(err.Error(), "message to edit not found") {
		c.bt.bot.log.Warn("message not found", userFields(user)...)
		//return app.StartHandlerByUser(user, "")
		// user.ForgetHistoryDay(day)
		return nil
	}

	// TODO: handle other errors

	c.bt.bot.log.Error("handler", userFields(user, "error", err)...)

	msgID, _ := c.bt.bot.send(user.ID(), c.bt.msgs.Messages(user.Language()).GeneralError())
	user.SetErrorMessage(msgID)

	return nil
}

func (c *contextImpl) edit(msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msgID == 0 {
		c.bt.bot.log.Error("message id cannot be empty", userFields(c.User())...)
		return nil
	}

	user := c.User()
	if msg == "" && kb != nil {
		err := c.bt.bot.editReplyMarkup(user.ID(), msgID, kb)
		if err != nil {
			return err
		}
		return nil
	}

	return c.bt.bot.edit(user.ID(), msgID, msg, append(opts, kb)...)
}
