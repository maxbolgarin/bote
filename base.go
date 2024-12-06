package bote

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/maxbolgarin/contem"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

var (
	errEmptyUserID = errm.New("empty user id")
	errEmptyMsgID  = errm.New("empty msg id")
)

type baseBot struct {
	bot *tele.Bot
	log Logger

	defaultOptions []any
	middlewares    []func(upd *tele.Update) bool
}

func startBot(ctx contem.Context, token string, cfg Config, log Logger) (*baseBot, error) {
	b := &baseBot{
		log:            log,
		defaultOptions: []any{cfg.ParseMode},
		middlewares:    make([]func(upd *tele.Update) bool, 0),
	}

	if cfg.NoPreview {
		b.defaultOptions = append(b.defaultOptions, tele.NoPreview)
	}

	bot, err := tele.NewBot(tele.Settings{
		Token:   token,
		Poller:  tele.NewMiddlewarePoller(&tele.LongPoller{Timeout: cfg.LPTimeout}, b.middleware),
		Client:  &http.Client{Timeout: 2 * cfg.LPTimeout},
		Verbose: cfg.Debug,
		OnError: func(err error, ctx tele.Context) {
			var userID int64
			if ctx != nil && ctx.Chat() != nil {
				userID = ctx.Chat().ID
			}
			b.log.Error("Bot.OnError", "error", err, "user_id", userID)
		},
	})
	if err != nil {
		return nil, errm.Wrap(err, "new telebot")
	}
	b.bot = bot

	b.log.Info("bot is starting")

	lang.Go(b.log, b.bot.Start)

	ctx.AddFunc(b.bot.Stop)

	return b, nil
}

func (b *baseBot) addMiddleware(f func(upd *tele.Update) bool) {
	b.middlewares = append(b.middlewares, f)
}

func (b *baseBot) handle(endpoint any, handler tele.HandlerFunc) {
	b.bot.Handle(endpoint, handler)
}

func (b *baseBot) send(userID int64, msg string, options ...any) (int, error) {
	if userID == 0 {
		return 0, errEmptyUserID
	}

	m, err := b.bot.Send(userIDWrapper(userID), msg, append(options, b.defaultOptions...)...)
	if err != nil {
		return 0, err
	}

	return m.ID, nil
}

func (b *baseBot) edit(userID int64, msgID int, what any, options ...any) error {
	if userID == 0 {
		return errEmptyUserID
	}
	if msgID == 0 {
		return errEmptyMsgID
	}

	_, err := b.bot.Edit(getEditable(userID, msgID), what, append(options, b.defaultOptions...)...)
	if err != nil {
		if strings.Contains(err.Error(), "message is not modified") {
			b.log.Warn("message is not modified", "msg_id", msgID, "user_id", userID, "method", "Bot.Edit")
			return nil
		}
		return err
	}

	return nil
}

func (b *baseBot) editReplyMarkup(userID int64, msgID int, markup *tele.ReplyMarkup) error {
	if userID == 0 {
		return errEmptyUserID
	}
	if msgID == 0 {
		return errEmptyMsgID
	}

	_, err := b.bot.EditReplyMarkup(getEditable(userID, msgID), markup)
	if err != nil {
		return err
	}

	return nil
}

func (b *baseBot) delete(userID int64, msgIDs ...int) error {
	if userID == 0 {
		return errEmptyUserID
	}

	errSet := errm.NewList()

	for _, msgID := range msgIDs {
		if msgID == 0 {
			return errEmptyMsgID
		}
		if err := b.bot.Delete(getEditable(userID, msgID)); err != nil {
			errSet.Add(err)
		}
	}

	return errSet.Err()
}

func (b *baseBot) deleteHistory(userID int64, lastMessageID int) {
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

func (b *baseBot) middleware(upd *tele.Update) bool {
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

type userIDWrapper int64

func (u userIDWrapper) Recipient() string {
	return strconv.Itoa(int(u))
}

func getEditable(senderID int64, messageID int) tele.Editable {
	return &tele.Message{ID: messageID, Chat: &tele.Chat{ID: senderID}}
}
