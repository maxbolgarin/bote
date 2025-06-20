package bote

import (
	"strconv"
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

	// MessageID returns an ID of the active message.
	// If handler was called from a callback button, message is the one with keyboard.
	// If handler was called from a text message, message is the one with an active text handler (not sent message!).
	MessageID() int

	// ButtonID returns an ID of the pressed callback button.
	ButtonID() string

	// Data returns callback button data. If there are many items in data, they will be separated by '|'.
	Data() string

	// DataParsed returns all items of button data.
	DataParsed() []string

	// Text returns a text sended by the user.
	Text() string

	// TextWithMessage returns a text sended by the user and the ID of this message (deleted by default).
	TextWithMessage() (string, int)

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

	// EditMainReplyMarkup edits reply markup of the main message.
	// opts are additional options for editing message.
	EditMainReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	// EditHistory edits message of the user by provided ID.
	// newState is a state of the user which will be set after editing message.
	// msgID is the ID of the message to edit.
	// opts are additional options for editing message.
	EditHistory(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHistoryReplyMarkup edits reply markup of the history message.
	// opts are additional options for editing message.
	EditHistoryReplyMarkup(msgID int, kb *tele.ReplyMarkup, opts ...any) error

	// EditHead edits head message of the user.
	// opts are additional options for editing message.
	EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHeadReplyMarkup edits reply markup of the head message.
	// opts are additional options for editing message.
	EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	// DeleteHistory deletes provided history message.
	DeleteHistory(msgIDs int) error

	// DeleteNotification deletes notification message of the user.
	DeleteNotification() error

	// DeleteError deletes error message of the user.
	DeleteError() error

	// DeleteAll deletes all messages of the user from the specified ID.
	DeleteAll(from int)

	// Btn creates button and registers handler for it. You can provide data for the button.
	// Data items will be separated by '|' in a single data string.
	// Button unique value is generated from hexing button name with 10 random bytes at the end.
	Btn(name string, callback HandlerFunc, dataList ...string) tele.Btn

	// DeleteUser deletes user from the memory and deletes all messages of the user.
	// Returns true if all messages were deleted successfully, false otherwise.
	// It doesn't delete user from the persistent database, so you should make it manually.
	DeleteUser() bool
}

func (b *Bot) newContext(c tele.Context) *contextImpl {
	upd := c.Update()
	return &contextImpl{
		bt:   b,
		ct:   c,
		user: b.um.getUser(getSender(&upd).ID),
	}
}

func (b *Bot) newContextFromUpdate(upd tele.Update) *contextImpl {
	return &contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: b.um.getUser(getSender(&upd).ID),
	}
}

// NewContext creates a new context for the given user simulating that callback button was pressed.
// It creates a minimal update to handle all possible methods in [Context] without panics.
// It can be useful if you want to start a handler without user action (by some external event).
// Warning! IT WON'T WORK WITH TEXT HANDLERS. Use [NewContextText] instead.
func NewContext(b *Bot, userID int64, callbackMsgID int, data ...string) Context {
	upd := tele.Update{
		Callback: &tele.Callback{
			Message: &tele.Message{ID: callbackMsgID, Sender: &tele.User{ID: userID}},
			Data:    CreateBtnData(data...),
		},
	}
	return &contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: b.um.getUser(userID),
	}
}

// NewContextText creates a new context for the given user simulating that text message was received.
// It creates a minimal update to handle all possible methods in [Context] without panics.
// It can be useful if you want to start a handler without user action (by some external event).
// Warning! IT WON'T WORK WITH CALLBACK HANDLERS. Use [NewContext] instead.
func NewContextText(b *Bot, userID int64, textMsgID int, text string) Context {
	upd := tele.Update{
		Message: &tele.Message{
			ID:   textMsgID,
			Text: text,
			Sender: &tele.User{
				ID: userID,
			},
		},
	}
	return &contextImpl{
		bt:   b,
		ct:   b.bot.tbot.NewContext(upd),
		user: b.um.getUser(userID),
	}
}

type contextImpl struct {
	bt   *Bot
	ct   tele.Context
	user *userContextImpl

	textMsgID int
}

func (c *contextImpl) User() User {
	return c.user
}

func (c *contextImpl) Tele() tele.Context {
	return c.ct
}

func (c *contextImpl) MessageID() int {
	if c.textMsgID != 0 {
		return c.textMsgID
	}
	if cb := c.ct.Callback(); cb != nil {
		return cb.Message.ID
	}
	return lang.Deref(c.ct.Message()).ID
}

func (c *contextImpl) ButtonID() string {
	if cb := c.ct.Callback(); cb != nil {
		if cb.Unique != "" {
			return getIDFromUnique(cb.Unique)
		}
		return getIDFromUnparsedData(cb.Data)
	}
	return ""
}

func (c *contextImpl) Data() string {
	if cb := c.ct.Callback(); cb != nil {
		return cb.Data
	}
	return ""
}

func (c *contextImpl) DataParsed() []string {
	return strings.Split(c.Data(), "|")
}

func (c *contextImpl) Text() string {
	msg, _ := c.TextWithMessage()
	return msg
}

func (c *contextImpl) TextWithMessage() (string, int) {
	msg := c.ct.Message()
	if msg == nil {
		return "", 0
	}
	// Do not return text of a bot message
	if c.user.Messages().HasMsgID(msg.ID) {
		return "", 0
	}
	return msg.Text, msg.ID
}

func (c *contextImpl) Set(key, value string) {
	c.ct.Set(key, value)
}

func (c *contextImpl) Get(key string) string {
	out, ok := c.ct.Get(key).(string)
	if !ok {
		return ""
	}
	return out
}

func (c *contextImpl) Send(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error) {
	if mainMsg == "" {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}

	if headMsg == "" {
		return c.SendMain(newState, mainMsg, mainKb, opts...)
	}

	mainMsg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(mainMsg, c.user, newState, 0, false)

	// Need copy to prevent from conflict in the next append because of using the same underlying array in opts
	headOpts := lang.If(len(opts) > 0, append(lang.Copy(opts), headKb), []any{headKb})
	headMsgID, err := c.bt.bot.send(c.user.ID(), headMsg, headOpts...)
	if err != nil {
		return c.prepareError(err, headMsgID)
	}

	mainMsgID, err := c.bt.bot.send(c.user.ID(), mainMsg, append(opts, mainKb)...)
	if err != nil {
		return c.prepareError(err, mainMsgID)
	}

	if headMsgID = c.user.Messages().HeadID; headMsgID != 0 {
		if err = c.bt.bot.delete(c.user.ID(), headMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", userFields(c.user)...)
		}
		// TODO: add cleanup task
	}

	c.user.handleSend(newState, mainMsgID, headMsgID)

	return nil
}

func (c *contextImpl) SendMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}

	msg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(msg, c.user, newState, 0, false)
	msgID, err := c.bt.bot.send(c.user.ID(), msg, append(opts, kb)...)
	if err != nil {
		return c.prepareError(err, msgID)
	}

	if headMsgID := c.user.Messages().HeadID; headMsgID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), headMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", userFields(c.user)...)
		}
	}

	c.user.handleSend(newState, msgID, 0)

	return nil
}

func (c *contextImpl) SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" {
		c.bt.bot.log.Error("notification message cannot be empty", userFields(c.User())...)
		return nil
	}
	if c.user.Messages().NotificationID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().NotificationID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous notification message", userFields(c.user)...)
		}
	}

	msgID, err := c.bt.bot.send(c.user.ID(), msg, append(opts, kb)...)
	if err != nil {
		return c.prepareError(err, msgID)
	}
	c.user.setNotificationMessage(msgID)

	return nil
}

func (c *contextImpl) SendError(msg string, opts ...any) error {
	closeBtn := c.bt.msgs.Messages(c.user.Language()).CloseBtn()
	if closeBtn != "" {
		opts = append(opts, SingleRow(c.Btn(closeBtn, func(c Context) error {
			return c.DeleteError()
		})))
	}
	msgID, err := c.bt.bot.send(c.user.ID(), msg, append(opts, tele.Silent)...)
	if err != nil {
		c.bt.bot.log.Error("failed to send error message", userFields(c.user)...)
		return nil
	}
	c.user.setErrorMessage(msgID)

	return nil
}

func (c *contextImpl) Edit(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) error {
	if mainMsg == "" && mainKb == nil {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}
	if headMsg == "" && headKb == nil {
		return c.EditMain(newState, mainMsg, mainKb, opts...)
	}

	msgIDs := c.user.Messages()

	headOpts := lang.If(len(opts) > 0, append(lang.Copy(opts), headKb), []any{headKb})
	if err := c.edit(msgIDs.HeadID, headMsg, headKb, headOpts...); err != nil {
		return c.prepareEditError(err, msgIDs.HeadID)
	}

	mainMsg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(mainMsg, c.user, newState, msgIDs.MainID, false)
	if err := c.edit(msgIDs.MainID, mainMsg, mainKb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.MainID)
	}

	c.user.setState(newState)

	return nil
}

func (c *contextImpl) EditMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("main message cannot be empty", userFields(c.User())...)
		return nil
	}

	msgIDs := c.user.Messages()

	msg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(msg, c.user, newState, msgIDs.MainID, false)
	if err := c.edit(msgIDs.MainID, msg, kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.MainID)
	}

	c.user.setState(newState)

	return nil
}

func (c *contextImpl) EditMainReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error {
	if kb == nil {
		c.bt.bot.log.Error("main keyboard cannot be empty", userFields(c.User())...)
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.MainID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.MainID)
	}

	return nil
}

func (c *contextImpl) EditHistory(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("message cannot be empty", userFields(c.User())...)
		return nil
	}

	msg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(msg, c.user, newState, msgID, true)
	if err := c.edit(msgID, msg, kb, opts...); err != nil {
		return c.prepareEditError(err, msgID)
	}

	c.user.setState(newState, msgID)

	return nil
}

func (c *contextImpl) EditHistoryReplyMarkup(msgID int, kb *tele.ReplyMarkup, opts ...any) error {
	if kb == nil {
		c.bt.bot.log.Error("history keyboard cannot be empty", userFields(c.User())...)
		return nil
	}

	if err := c.edit(msgID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgID)
	}

	return nil
}

func (c *contextImpl) EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("head message cannot be empty", userFields(c.User())...)
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.HeadID, msg, kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.HeadID)
	}

	return nil
}

func (c *contextImpl) EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error {
	if kb == nil {
		c.bt.bot.log.Error("head keyboard cannot be empty", userFields(c.User())...)
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.HeadID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.HeadID)
	}

	return nil
}

func (c *contextImpl) DeleteHistory(msgID int) error {
	for _, id := range c.User().Messages().HistoryIDs {
		if id == msgID {
			if err := c.bt.bot.delete(c.user.ID(), msgID); err != nil {
				return c.prepareError(err, msgID)
			}
			c.user.forgetHistoryMessage(msgID)
		}
	}
	return nil
}

func (c *contextImpl) DeleteNotification() error {
	if c.user.Messages().NotificationID == 0 {
		return nil
	}
	if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().NotificationID); err != nil {
		return c.prepareError(err, c.user.Messages().NotificationID)
	}
	c.user.setNotificationMessage(0)
	return nil
}

func (c *contextImpl) DeleteError() error {
	if c.user.Messages().ErrorID == 0 {
		return nil
	}
	if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().ErrorID); err != nil {
		return c.prepareError(err, c.user.Messages().ErrorID)
	}
	c.user.setErrorMessage(0)
	return nil
}

func (c *contextImpl) DeleteAll(from int) {
	deleted := c.bt.bot.deleteHistory(c.user.ID(), from)
	msgs := c.user.Messages()
	if _, ok := deleted[msgs.MainID]; ok {
		msgs.MainID = 0
	}
	if _, ok := deleted[msgs.HeadID]; ok {
		msgs.HeadID = 0
	}
	if _, ok := deleted[msgs.NotificationID]; ok {
		msgs.NotificationID = 0
	}
	if _, ok := deleted[msgs.ErrorID]; ok {
		msgs.ErrorID = 0
	}

	historyIDsToDelete := make([]int, 0, len(msgs.HistoryIDs))
	for _, id := range msgs.HistoryIDs {
		if _, ok := deleted[id]; ok {
			historyIDsToDelete = append(historyIDsToDelete, id)
		}
	}
	c.user.forgetHistoryMessage(historyIDsToDelete...)

	c.user.setMessages(
		append(
			append(
				make([]int, 0, len(msgs.HistoryIDs)+4),
				msgs.MainID, msgs.HeadID, msgs.NotificationID, msgs.ErrorID),
			msgs.HistoryIDs...)...)
}

func (c *contextImpl) DeleteUser() bool {
	isOK := true
	for _, id := range c.user.Messages().HistoryIDs {
		if err := c.bt.bot.delete(c.user.ID(), id); err != nil {
			c.bt.bot.log.Warn("cannot delete history message", userFields(c.user, "msg_id", id)...)
			isOK = false
		}
	}
	if c.user.Messages().NotificationID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().NotificationID); err != nil {
			c.bt.bot.log.Warn("cannot delete notification message", userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().ErrorID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().ErrorID); err != nil {
			c.bt.bot.log.Warn("cannot delete error message", userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().MainID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().MainID); err != nil {
			c.bt.bot.log.Warn("cannot delete main message", userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().HeadID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().HeadID); err != nil {
			c.bt.bot.log.Warn("cannot delete head message", userFields(c.user)...)
			isOK = false
		}
	}
	c.bt.um.deleteUser(c.user.ID())
	return isOK
}

func (c *contextImpl) prepareError(err error, msgID int) error {
	c.Set("msg_id", strconv.Itoa(msgID))
	return err
}

func (c *contextImpl) prepareEditError(err error, msgID int) error {
	err = c.prepareError(err, msgID)
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "message is not modified") {
		return nil
	}
	return err
}

func (c *contextImpl) handleError(err error) error {
	if err == nil {
		return nil
	}

	errorMsg := err.Error()

	// Handle specific error types
	if c.handleBotBlockedError(errorMsg) {
		return nil
	}

	if c.handleNotModifiedError(errorMsg) {
		return nil
	}

	if c.handleMessageNotFoundError(errorMsg) {
		return nil
	}

	if c.handleConnectionError(errorMsg, err) {
		return nil
	}

	// Handle generic errors
	c.handleGenericError(err)
	return nil
}

func (c *contextImpl) handleBotBlockedError(errorMsg string) bool {
	if !strings.Contains(errorMsg, "bot was blocked by the user") {
		return false
	}

	c.bt.bot.log.Info("bot is blocked by user, disable", userFields(c.user)...)
	c.bt.um.disableUser(c.user.ID())
	return true
}

// error when you want to edit message with the same text and buttons
func (*contextImpl) handleNotModifiedError(errorMsg string) bool {
	return strings.Contains(errorMsg, "is not modified")
}

// error when you want to edit message that is not found
func (c *contextImpl) handleMessageNotFoundError(errorMsg string) bool {
	if !strings.Contains(errorMsg, "message to edit not found") {
		return false
	}

	msgIDRaw := c.Get("msg_id")
	if msgIDRaw == "" {
		c.bt.bot.log.Warn("message to edit not found", userFields(c.user)...)
		return true
	}

	msgID, err := strconv.Atoi(msgIDRaw)
	if err != nil {
		return true
	}

	// Try to remove message from history
	if c.user.forgetHistoryMessage(msgID) {
		c.bt.bot.log.Warn("history message not found", userFields(c.user, "msg_id", msgID)...)
		return true
	}

	// Handle special message types
	c.handleSpecialMessageNotFound(msgID)
	return true
}

func (c *contextImpl) handleSpecialMessageNotFound(msgID int) {
	msgs := c.user.Messages()

	switch msgID {
	case msgs.MainID, msgs.HeadID:
		c.bt.bot.log.Warn("main/head message not found", userFields(c.user, "msg_id", msgID)...)
		c.bt.sendError(c.user.ID(), c.bt.msgs.Messages(c.user.Language()).GeneralError())

	case msgs.NotificationID:
		c.bt.bot.log.Warn("notification message not found", userFields(c.user, "msg_id", msgID)...)
		c.user.setNotificationMessage(0)

	case msgs.ErrorID:
		c.bt.bot.log.Warn("error message not found", userFields(c.user, "msg_id", msgID)...)
		c.user.setErrorMessage(0)
	}
}

func (c *contextImpl) handleConnectionError(errorMsg string, err error) bool {
	if !strings.Contains(errorMsg, "reset by peer") {
		return false
	}

	c.bt.bot.log.Warn("connection error", userFields(c.user, "error", err)...)
	return true
}

func (c *contextImpl) handleGenericError(err error) {
	c.bt.bot.log.Error("handler", userFields(c.user, "error", err)...)

	// Create error message with optional close button
	closeBtn := c.bt.msgs.Messages(c.user.Language()).CloseBtn()
	opts := []any{tele.Silent}

	if closeBtn != "" {
		opts = append(opts, SingleRow(c.Btn(closeBtn, func(c Context) error {
			return c.DeleteError()
		})))
	}

	c.bt.sendError(c.user.ID(), c.bt.msgs.Messages(c.user.Language()).GeneralError(), opts...)
}

func (c *contextImpl) edit(msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if msgID == 0 {
		c.bt.bot.log.Error("message id cannot be empty", userFields(c.user)...)
		return nil
	}

	if msg == "" && kb != nil {
		err := c.bt.bot.editReplyMarkup(c.user.ID(), msgID, kb)
		if err != nil {
			return err
		}
		return nil
	}

	return c.bt.bot.edit(c.user.ID(), msgID, msg, append(opts, kb)...)
}
