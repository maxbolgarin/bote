package bote

import (
	"net/http"
	"strings"
	"time"

	"github.com/maxbolgarin/contem"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	"github.com/maxbolgarin/logze"
	tele "gopkg.in/telebot.v3"
)

type BaseBot struct {
	Bot *tele.Bot
	log logze.Logger

	defaultOptions []any
	middlewares    []MiddlewareFunc
}

func NewBase(ctx contem.Context, cfg Config) (*BaseBot, error) {
	cfg, err := cfg.prepare()
	if err != nil {
		return nil, errm.Wrap(err, "prepare")
	}

	b := &BaseBot{
		log:            cfg.Logger,
		defaultOptions: []any{cfg.ParseMode},
		middlewares:    make([]MiddlewareFunc, 0),
	}

	if cfg.NoPreview {
		b.defaultOptions = append(b.defaultOptions, tele.NoPreview)
	}

	bot, err := tele.NewBot(tele.Settings{
		Token:   cfg.Token,
		Poller:  tele.NewMiddlewarePoller(&tele.LongPoller{Timeout: cfg.LPTimeout}, b.middleware),
		Client:  &http.Client{Timeout: 2 * cfg.LPTimeout},
		Verbose: cfg.Debug,
		OnError: func(err error, ctx tele.Context) {
			var userID int64
			if ctx != nil && ctx.Chat() != nil {
				userID = ctx.Chat().ID
			}
			b.logError(err, userID, "Bot.OnError")
		},
	})
	if err != nil {
		return nil, errm.Wrap(err, "new telebot")
	}
	b.Bot = bot

	b.log.Info("bot is starting")
	runGoroutine(b.Bot.Start)

	ctx.AddFunc(b.Bot.Stop)

	return b, nil
}

// AddMiddleware adds middleware to the bot.
func (b *BaseBot) AddMiddleware(f MiddlewareFunc) {
	b.middlewares = append(b.middlewares, f)
}

// AddMessageMiddleware adds middleware to the bot that only works with messages.
func (b *BaseBot) AddMessageMiddleware(f MiddlewareMessageFunc) {
	b.middlewares = append(b.middlewares, func(upd *tele.Update) bool {
		if upd.Message == nil {
			return true
		}
		return f(upd.Message)
	})
}

// AddCallbackMiddleware adds middleware to the bot that only works with callbacks.
func (b *BaseBot) AddCallbackMiddleware(f MiddlewareCallbackFunc) {
	b.middlewares = append(b.middlewares, func(upd *tele.Update) bool {
		if upd.Callback == nil {
			return true
		}
		return f(upd.Callback)
	})
}

// Handle adds handler to the provided endpoint (e.g. button).
func (b *BaseBot) Handle(endpoint any, handler tele.HandlerFunc) {
	b.Bot.Handle(endpoint, handler)
}

// Send sends a message and logs an error if something goes wrong.
func (b *BaseBot) Send(userID int64, msg string, options ...any) int {
	msgID, err := b.send(userID, msg, options...)
	if err != nil {
		b.logError(err, userID, "Bot.Send", options...)
		return 0
	}
	return msgID
}

// SendWithError sends a message and returns an error if something goes wrong.
func (b *BaseBot) SendWithError(userID int64, msg string, options ...any) (int, error) {
	return b.send(userID, msg, options...)
}

// Edit edits the message by ID and logs an error if something goes wrong.
func (b *BaseBot) Edit(userID int64, msgID int, what any, options ...any) {
	err := b.edit(userID, msgID, what, options...)
	if err != nil {
		b.logError(err, userID, "Bot.Edit", options...)
	}
}

// EditWithError edits the message by ID and returns an error if something goes wrong.
func (b *BaseBot) EditWithError(userID int64, msgID int, what any, options ...any) error {
	return b.edit(userID, msgID, what, options...)
}

// EditReplyMarkup edits the message's reply markup by ID and logs an error if something goes wrong.
func (b *BaseBot) EditReplyMarkup(userID int64, msgID int, markup *tele.ReplyMarkup) {
	err := b.editReplyMarkup(userID, msgID, markup)
	if err != nil {
		b.logError(err, userID, "Bot.EditReplyMarkup", markup)
	}
}

// EditReplyMarkup edits the message's reply markup by ID and returns an error if something goes wrong.
func (b *BaseBot) EditReplyMarkupWithError(userID int64, msgID int, markup *tele.ReplyMarkup) error {
	return b.editReplyMarkup(userID, msgID, markup)
}

// Delete deletes messages and logs errors if something goes wrong.
func (b *BaseBot) Delete(userID int64, msgIDs ...int) {
	for _, msgID := range msgIDs {
		err := b.delete(userID, msgID)
		if err != nil {
			b.logError(err, userID, "Bot.Delete")
		}
	}
}

// Delete deletes messages and returns an error set if something goes wrong.
func (b *BaseBot) DeleteWithError(userID int64, msgIDs ...int) error {
	errSet := errm.NewSet()
	for _, msgID := range msgIDs {
		err := b.delete(userID, msgID)
		if err != nil {
			errSet.Add(err)
		}
	}
	return errSet.Err()
}

// DeleteMany try to delete all historical messages.
func (b *BaseBot) DeleteHistory(userID int64, lastMessageID int) {
	var counter int
	for msgID := lastMessageID - 1; msgID > 1; msgID-- {
		err := b.delete(userID, msgID)
		if err != nil {
			counter += 1
		} else {
			counter = 0
		}
		if counter == 5 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (b *BaseBot) send(userID int64, msg string, options ...any) (int, error) {
	if userID == 0 {
		return 0, EmptyUserIDErr
	}

	m, err := b.Bot.Send(UserID(userID), msg, append(options, b.defaultOptions...)...)
	if err != nil {
		return 0, err
	}

	return m.ID, nil
}

func (b *BaseBot) edit(userID int64, msgID int, what any, options ...any) error {
	if userID == 0 {
		return EmptyUserIDErr
	}
	if msgID == 0 {
		return EmptyMsgIDErr
	}

	_, err := b.Bot.Edit(getEditable(userID, msgID), what, append(options, b.defaultOptions...)...)
	if err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			b.log.Warn("message is not modified", "msg_id", msgID, "user_id", userID, "method", "Bot.Edit")
			return nil
		}
		return err
	}

	return nil
}

func (b *BaseBot) editReplyMarkup(userID int64, msgID int, markup *tele.ReplyMarkup) error {
	if userID == 0 {
		return EmptyUserIDErr
	}
	if msgID == 0 {
		return EmptyMsgIDErr
	}

	_, err := b.Bot.EditReplyMarkup(getEditable(userID, msgID), markup)
	if err != nil {
		return err
	}

	return nil
}

func (b *BaseBot) delete(userID int64, msgID int) error {
	if userID == 0 {
		return EmptyUserIDErr
	}
	if msgID == 0 {
		return EmptyMsgIDErr
	}

	if err := b.Bot.Delete(getEditable(userID, msgID)); err != nil {
		if !strings.Contains(err.Error(), "message to delete not found") {
			b.log.Warn("message to delete not found", "msg_id", msgID, "user_id", userID, "method", "Bot.Delete")
			return nil
		}
		return err
	}

	return nil
}

func (b *BaseBot) logError(err error, userID int64, method string, options ...any) {
	if err == nil {
		return
	}
	if len(options) == 0 {
		b.log.Err(err, "user_id", userID, "method", method)
		return
	}
	b.log.Err(err, "user_id", userID, "method", method, "opts", options)
}

func (b *BaseBot) middleware(upd *tele.Update) bool {
	if upd.MyChatMember != nil {
		if lang.Deref(upd.MyChatMember.NewChatMember).Role == "kicked" {
			b.log.Warn("bot is blocked",
				"user_id", lang.Deref(upd.MyChatMember.Sender).ID,
				"username", lang.Deref(upd.MyChatMember.Sender).Username,
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)

			return false
		}
		if lang.Deref(upd.MyChatMember.OldChatMember).Role == "kicked" {
			b.log.Info("bot is unblocked",
				"user_id", lang.Deref(upd.MyChatMember.Sender).ID,
				"username", lang.Deref(upd.MyChatMember.Sender).Username,
				"old_role", lang.Deref(upd.MyChatMember.OldChatMember).Role,
				"new_role", lang.Deref(upd.MyChatMember.NewChatMember).Role)

			return false
		}
	}

	for _, m := range b.middlewares {
		if !m(upd) {
			return false
		}
	}

	return true
}
