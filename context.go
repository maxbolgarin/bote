package bote

import (
	"strconv"
	"strings"

	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// Context is an interface that provides to every handler.
type Context interface {
	// Tele returns underlying telebot context.
	Tele() tele.Context

	// User returns current User context if chat type is private.
	// WARNING: If chat type is not private, returns public read only user.
	User() User

	// IsPrivate returns true if the current chat type is private.
	IsPrivate() bool

	// ChatID returns the ID of the current chat.
	ChatID() int64

	// ChatType returns the type of the current chat.
	ChatType() tele.ChatType

	// IsMentioned returns true if the bot is mentioned in the current message.
	IsMentioned() bool

	// IsReply returns true if the current message is a reply to the bot's message.
	IsReply() bool

	// MessageID returns an ID of the message.
	// If handler was called from a callback button, message is the one with keyboard.
	// If handler was called from a text message and user's state IsText == true,
	//  message is the one with an active text handler with this state (not sent message!).
	// In other case it is an ID of the message sent to the chat.
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

	// Btn creates button and registers handler for it. You can provide data for the button.
	// Data items will be separated by '|' in a single data string.
	// Button unique value is generated from hexing button name with 10 random bytes at the end.
	Btn(name string, callback HandlerFunc, dataList ...string) tele.Btn

	// Send sends new main and head messages to the user.
	// Old head message will be deleted. Old main message will becomve historical.
	// newState is a state of the user which will be set after sending message.
	// opts are additional options for sending messages.
	// WARNING: It works only in private chats.
	Send(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error)

	// SendMain sends new main message to the user.
	// Old head message will be deleted. Old main message will becomve historical.
	// newState is a state of the user which will be set after sending message.
	// opts are additional options for sending message.
	// WARNING: It works only in private chats.
	SendMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// SendNotification sends a notification message to the user.
	// opts are additional options for sending message.
	// WARNING: It works only in private chats.
	SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error

	// SendError sends an error message to the user.
	// opts are additional options for sending message.
	// WARNING: It works only in private chats.
	SendError(msg string, opts ...any) error

	// SendFile sends a file to the user.
	// name is the name of the file to send.
	// file is the file to send.
	// opts are additional options for sending the file.
	// WARNING: It works only in private chats.
	SendFile(name string, file []byte, opts ...any) error

	// SendInChat sends a message to a specific chat ID and thread ID.
	// chatID is the target chat ID, threadID is the target thread ID (0 for no thread).
	// msg is the message to send.
	// kb is the keyboard to send.
	// opts are additional options for sending the message.
	SendInChat(chatID int64, threadID int, msg string, kb *tele.ReplyMarkup, opts ...any) (int, error)

	// Edit edits main and head messages of the user.
	// newState is a state of the user which will be set after editing message.
	// opts are additional options for editing messages.
	// WARNING: It works only in private chats.
	Edit(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) (err error)

	// EditMain edits main message of the user.
	// newState is a state of the user which will be set after editing message.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditMainReplyMarkup edits reply markup of the main message.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditMainReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	// EditHistory edits message of the user by provided ID.
	// newState is a state of the user which will be set after editing message.
	// msgID is the ID of the message to edit.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditHistory(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHistoryReplyMarkup edits reply markup of the history message.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditHistoryReplyMarkup(msgID int, kb *tele.ReplyMarkup, opts ...any) error

	// EditHead edits head message of the user.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error

	// EditHeadReplyMarkup edits reply markup of the head message.
	// opts are additional options for editing message.
	// WARNING: It works only in private chats.
	EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	// EditInChat edits message in a specific chat ID and thread ID.
	// chatID is the target chat ID, threadID is the target thread ID (0 for no thread).
	// msg is the message to edit.
	// kb is the keyboard to edit.
	// opts are additional options for editing message.
	EditInChat(chatID int64, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error

	// DeleteHead deletes head message of the user.
	// WARNING: It works only in private chats.
	DeleteHead() error

	// DeleteHistory deletes provided history message.
	// msgID is the ID of the history message to delete.
	// WARNING: It works only in private chats.
	DeleteHistory(msgID int) error

	// DeleteNotification deletes notification message of the user.
	// WARNING: It works only in private chats.
	DeleteNotification() error

	// DeleteError deletes error message of the user.
	// WARNING: It works only in private chats.
	DeleteError() error

	// DeleteAll deletes all messages of the user from the specified ID.
	// WARNING: It works only in private chats.
	DeleteAll(from int)

	// Delete deletes message by provided chat ID and message ID.
	DeleteInChat(chatID int64, msgID int) error

	// DeleteUser deletes user from the memory and deletes all messages of the user.
	// Returns true if all messages were deleted successfully, false otherwise.
	// It doesn't delete user from the persistent database, so you should make it manually.
	// WARNING: It works only in private chats.
	DeleteUser() bool
}

func (b *Bot) newContext(c tele.Context) *contextImpl {
	upd := c.Update()
	result := &contextImpl{bt: b, ct: c}

	sender := getSender(&upd)
	if sender == nil {
		return result
	}

	if chat := c.Chat(); chat != nil && chat.Type == tele.ChatPrivate {
		result.user = b.um.getUser(sender.ID)
	}
	if result.user == nil {
		result.user = newPublicUserContext(sender)
	}

	return result
}

func (b *Bot) newContextFromUpdate(upd tele.Update) *contextImpl {
	return b.newContext(b.bot.tbot.NewContext(upd))
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
	data := strings.Split(c.Data(), "|")
	if len(data) == 0 || data[0] == "" {
		return nil
	}
	return data
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

func (c *contextImpl) IsMentioned() bool {
	msg := c.ct.Message()
	if msg == nil {
		return false
	}

	// Check if the message has entities (mentions)
	if msg.Entities == nil {
		return false
	}

	// Get bot username from the bot instance
	bot := c.bt.Bot()
	if bot == nil {
		return false
	}

	botUsername := bot.Me.Username
	if botUsername == "" {
		return false
	}

	// Check for mention entities
	for _, entity := range msg.Entities {
		if entity.Type == "mention" {
			// Extract the mentioned username from the text
			start := entity.Offset
			end := start + entity.Length
			if end <= len(msg.Text) {
				mentionedUsername := msg.Text[start:end]
				// Remove @ symbol if present
				if len(mentionedUsername) > 0 && mentionedUsername[0] == '@' {
					mentionedUsername = mentionedUsername[1:]
				}
				if mentionedUsername == botUsername {
					return true
				}
			}
		}
	}

	return false
}

func (c *contextImpl) ChatID() int64 {
	chat := c.ct.Chat()
	if chat == nil {
		return 0
	}
	return chat.ID
}

func (c *contextImpl) ChatType() tele.ChatType {
	chat := c.ct.Chat()
	if chat == nil {
		return tele.ChatPrivate
	}
	return chat.Type
}

func (c *contextImpl) IsReply() bool {
	msg := c.ct.Message()
	if msg == nil {
		return false
	}

	replyTo := msg.ReplyTo
	if replyTo == nil {
		return false
	}

	return replyTo.Sender.ID == c.bt.Bot().Me.ID
}

func (c *contextImpl) IsPrivate() bool {
	return c.ChatType() == tele.ChatPrivate
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
	if !c.validateUserSendInput(mainMsg, "Send") {
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

	if oldHeadMsgID := c.user.Messages().HeadID; oldHeadMsgID != 0 {
		if err = c.bt.bot.delete(c.user.ID(), oldHeadMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", c.bt.userFields(c.user)...)
		}
		// TODO: add cleanup task
	}

	c.user.handleSend(newState, mainMsgID, headMsgID)

	return nil
}

func (c *contextImpl) SendMain(newState State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserSendInput(msg, "SendMain") {
		return nil
	}

	msg = c.bt.msgs.Messages(c.user.Language()).PrepareMessage(msg, c.user, newState, 0, false)
	msgID, err := c.bt.bot.send(c.user.ID(), msg, append(opts, kb)...)
	if err != nil {
		return c.prepareError(err, msgID)
	}

	if oldHeadMsgID := c.user.Messages().HeadID; oldHeadMsgID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), oldHeadMsgID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous head message", c.bt.userFields(c.user)...)
		}
	}

	c.user.handleSend(newState, msgID, 0)

	return nil
}

func (c *contextImpl) SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserSendInput(msg, "SendNotification") {
		return nil
	}

	if c.user.Messages().NotificationID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().NotificationID); err != nil {
			c.bt.bot.log.Warn("cannot delete previous notification message", c.bt.userFields(c.user)...)
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
	if !c.validateUserSendInput(msg, "SendError") {
		return nil
	}

	closeBtn := c.bt.msgs.Messages(c.user.Language()).CloseBtn()
	if closeBtn != "" {
		opts = append(opts, SingleRow(c.Btn(closeBtn, func(c Context) error {
			return c.DeleteError()
		})))
	}
	msgID, err := c.bt.bot.send(c.user.ID(), msg, append(opts, tele.Silent)...)
	if err != nil {
		c.bt.bot.log.Error("failed to send error message", c.bt.userFields(c.user)...)
		return nil
	}
	c.user.setErrorMessage(msgID)

	return nil
}

func (c *contextImpl) SendFile(name string, file []byte, opts ...any) error {
	if !c.validateUserSendInput(name, "SendFile") {
		return nil
	}

	if len(file) == 0 {
		c.bt.bot.log.Error("file cannot be empty", c.bt.userFields(c.User())...)
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}

	msgID, err := c.bt.bot.sendFile(c.user.ID(), file, name, opts...)
	if err != nil {
		return c.prepareError(err, msgID)
	}

	return nil
}

func (c *contextImpl) SendInChat(chatID int64, threadID int, msg string, kb *tele.ReplyMarkup, opts ...any) (int, error) {
	if chatID == 0 {
		c.bt.bot.log.Error("chat ID cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return 0, nil
	}
	if msg == "" {
		c.bt.bot.log.Error("message cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return 0, nil
	}

	if threadID > 0 {
		opts = append(opts, tele.MessageThreadID(threadID))
	}

	msgID, err := c.bt.bot.send(chatID, msg, append(opts, kb)...)
	if err != nil {
		return 0, c.prepareError(err, msgID)
	}

	return msgID, nil
}

func (c *contextImpl) Edit(newState State, mainMsg, headMsg string, mainKb, headKb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserEditInput(mainMsg, mainKb, "Edit") {
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
	if !c.validateUserEditInput(msg, kb, "EditMain") {
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
	if !c.validateUserEditInput("", kb, "EditMainReplyMarkup") {
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.MainID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.MainID)
	}

	return nil
}

func (c *contextImpl) EditHistory(newState State, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserEditInput(msg, kb, "EditHistory") {
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
	if !c.validateUserEditInput("", kb, "EditHistoryReplyMarkup") {
		return nil
	}

	if err := c.edit(msgID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgID)
	}

	return nil
}

func (c *contextImpl) EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserEditInput(msg, kb, "EditHead") {
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.HeadID, msg, kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.HeadID)
	}

	return nil
}

func (c *contextImpl) EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error {
	if !c.validateUserEditInput("", kb, "EditHeadReplyMarkup") {
		return nil
	}

	msgIDs := c.user.Messages()

	if err := c.edit(msgIDs.HeadID, "", kb, opts...); err != nil {
		return c.prepareEditError(err, msgIDs.HeadID)
	}

	return nil
}

func (c *contextImpl) EditInChat(chatID int64, msgID int, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	if chatID == 0 {
		c.bt.bot.log.Error("chat ID cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
	if msg == "" {
		c.bt.bot.log.Error("message cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}

	if err := c.bt.bot.edit(chatID, msgID, msg, append(opts, kb)...); err != nil {
		return c.prepareError(err, msgID)
	}

	return nil
}

func (c *contextImpl) DeleteHistory(msgID int) error {
	if !c.validateUserInput("DeleteHistory") {
		return nil
	}
	if msgID == 0 {
		c.bt.bot.log.Error("message ID cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
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

func (c *contextImpl) DeleteHead() error {
	if !c.validateUserInput("DeleteHead") {
		return nil
	}
	if c.user.Messages().HeadID == 0 {
		return nil
	}
	if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().HeadID); err != nil {
		return c.prepareError(err, c.user.Messages().HeadID)
	}
	c.user.setHeadMessage(0)
	return nil
}

func (c *contextImpl) DeleteNotification() error {
	if !c.validateUserInput("DeleteNotification") {
		return nil
	}
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
	if !c.validateUserInput("DeleteError") {
		return nil
	}
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
	if !c.validateUserInput("DeleteAll") {
		return
	}
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

func (c *contextImpl) DeleteInChat(chatID int64, msgID int) error {
	if chatID == 0 {
		c.bt.bot.log.Error("chat ID cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}
	if msgID == 0 {
		c.bt.bot.log.Error("message ID cannot be empty")
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return nil
	}

	if err := c.bt.bot.delete(chatID, msgID); err != nil {
		return c.prepareError(err, msgID)
	}
	if chatID != c.user.ID() {
		return nil
	}
	userMsgs := c.user.Messages()
	switch msgID {
	case userMsgs.MainID:
		c.user.setMainMessage(0)
	case userMsgs.HeadID:
		c.user.setHeadMessage(0)
	case userMsgs.NotificationID:
		c.user.setNotificationMessage(0)
	case userMsgs.ErrorID:
		c.user.setErrorMessage(0)
	default:
		for _, historyID := range userMsgs.HistoryIDs {
			if historyID == msgID {
				c.user.forgetHistoryMessage(historyID)
				break
			}
		}
	}
	return nil
}

func (c *contextImpl) DeleteUser() bool {
	if !c.validateUserInput("DeleteUser") {
		return false
	}
	isOK := true
	for _, id := range c.user.Messages().HistoryIDs {
		if err := c.bt.bot.delete(c.user.ID(), id); err != nil {
			c.bt.bot.log.Warn("cannot delete history message", c.bt.userFields(c.user, "msg_id", id)...)
			isOK = false
		}
	}
	if c.user.Messages().NotificationID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().NotificationID); err != nil {
			c.bt.bot.log.Warn("cannot delete notification message", c.bt.userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().ErrorID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().ErrorID); err != nil {
			c.bt.bot.log.Warn("cannot delete error message", c.bt.userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().MainID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().MainID); err != nil {
			c.bt.bot.log.Warn("cannot delete main message", c.bt.userFields(c.user)...)
			isOK = false
		}
	}
	if c.user.Messages().HeadID != 0 {
		if err := c.bt.bot.delete(c.user.ID(), c.user.Messages().HeadID); err != nil {
			c.bt.bot.log.Warn("cannot delete head message", c.bt.userFields(c.user)...)
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

func (c *contextImpl) validateUserInput(methodName string) bool {
	if c.user.isPublic {
		c.bt.bot.log.Error("cannot use user methods (", methodName, ") in public chats", "chat_id", c.ChatID())
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return false
	}
	return true
}

func (c *contextImpl) validateUserSendInput(msg string, methodName string) bool {
	if !c.validateUserInput(methodName) {
		return false
	}
	if msg == "" {
		c.bt.bot.log.Error("message cannot be empty", c.bt.userFields(c.User())...)
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return false
	}
	return true
}

func (c *contextImpl) validateUserEditInput(msg string, kb *tele.ReplyMarkup, methodName string) bool {
	if !c.validateUserInput(methodName) {
		return false
	}
	if msg == "" && kb == nil {
		c.bt.bot.log.Error("message cannot be empty", c.bt.userFields(c.User())...)
		c.bt.bot.metr.incError(MetricsErrorBadUsage, MetricsErrorSeveritHigh)
		return false
	}
	return true
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

	c.bt.bot.log.Info("bot is blocked by user, disable", c.bt.userFields(c.user)...)
	c.bt.um.disableUser(c.user.ID())
	c.bt.bot.metr.incError(MetricsErrorBotBlocked, MetricsErrorSeverityLow)

	return true
}

// error when you want to edit message with the same text and buttons
func (c *contextImpl) handleNotModifiedError(errorMsg string) bool {
	if !strings.Contains(errorMsg, "is not modified") {
		return false
	}
	return true
}

// error when you want to edit message that is not found
func (c *contextImpl) handleMessageNotFoundError(errorMsg string) bool {
	if !strings.Contains(errorMsg, "message to edit not found") {
		return false
	}
	c.bt.bot.metr.incError(MetricsErrorInvalidUserState, MetricsErrorSeverityLow)

	msgIDRaw := c.Get("msg_id")
	if msgIDRaw == "" {
		c.bt.bot.log.Warn("message to edit not found", c.bt.userFields(c.user)...)
		return true
	}

	msgID, err := strconv.Atoi(msgIDRaw)
	if err != nil {
		return true
	}

	// Try to remove message from history
	if c.user.forgetHistoryMessage(msgID) {
		c.bt.bot.log.Warn("history message not found", c.bt.userFields(c.user, "msg_id", msgID)...)
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
		c.bt.bot.log.Warn("main/head message not found", c.bt.userFields(c.user, "msg_id", msgID)...)
		c.bt.sendError(c.user.ID(), c.bt.msgs.Messages(c.user.Language()).GeneralError())

	case msgs.NotificationID:
		c.bt.bot.log.Warn("notification message not found", c.bt.userFields(c.user, "msg_id", msgID)...)
		c.user.setNotificationMessage(0)

	case msgs.ErrorID:
		c.bt.bot.log.Warn("error message not found", c.bt.userFields(c.user, "msg_id", msgID)...)
		c.user.setErrorMessage(0)
	}
}

func (c *contextImpl) handleConnectionError(errorMsg string, err error) bool {
	if !strings.Contains(errorMsg, "reset by peer") {
		return false
	}
	c.bt.bot.log.Warn("connection error", c.bt.userFields(c.user, "error", err.Error())...)
	c.bt.bot.metr.incError(MetricsErrorConnectionError, MetricsErrorSeverityLow)
	return true
}

func (c *contextImpl) handleGenericError(err error) {
	c.bt.bot.log.Error("handler", c.bt.userFields(c.user, "error", err.Error())...)
	c.bt.bot.metr.incError(MetricsErrorHandler, MetricsErrorSeveritHigh)

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
		c.bt.bot.log.Error("message id cannot be empty", c.bt.userFields(c.user)...)
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
