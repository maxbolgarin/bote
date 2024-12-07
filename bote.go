package bote

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// Bote is a main struct of this package. It contains all necessary components for working with Telegram bot.
type Bote struct {
	bot  *baseBot
	um   *userManagerImpl
	msgs MessageProvider
	rlog UpdateLogger

	defaultLanguageCode string

	middlewares    *abstract.SafeSlice[MiddlewareFunc]
	deleteMessages bool
}

// Start starts the bot. All that you need is to pass your bot token and options.
// Don't forget to call Stop() when you're done.
func Start(ctx context.Context, token string, optsRaw ...Options) (*Bote, error) {
	if token == "" {
		return nil, errm.New("token cannot be empty")
	}
	opts, err := prepareOpts(optsRaw...)
	if err != nil {
		return nil, errm.Wrap(err, "prepare opts")
	}

	um, err := newUserManager(ctx, opts.UserDB, opts.Logger)
	if err != nil {
		return nil, errm.Wrap(err, "new user manager")
	}

	b, err := startBot(token, opts.Config, opts.Logger)
	if err != nil {
		return nil, errm.Wrap(err, "start bot")
	}

	bote := &Bote{
		bot:  b,
		um:   um,
		msgs: opts.Msgs,
		rlog: opts.UpdateLogger,

		defaultLanguageCode: opts.Config.DefaultLanguageCode,
		middlewares:         abstract.NewSafeSlice[MiddlewareFunc](),
		deleteMessages:      *opts.Config.DeleteMessages,
	}

	b.addMiddleware(bote.masterMiddleware)

	bote.AddMiddleware(bote.cleanMiddleware)

	if *opts.Config.LogUpdates {
		bote.AddMiddleware(bote.logMiddleware)
	}

	return bote, nil
}

func (b *Bote) Stop() {
	b.bot.bot.Stop()
}

func (b *Bote) Bot() *tele.Bot {
	return b.bot.bot
}

func (b *Bote) GetUser(userID int64) User {
	return b.um.getUser(userID)
}

func (b *Bote) GetAllUsers() []User {
	return b.um.getAllUsers()
}

func (b *Bote) AddMiddleware(f ...MiddlewareFunc) {
	b.middlewares.Append(f...)
}

func (b *Bote) SetTextHandler(h HandlerFunc) {
	b.Handle(tele.OnText, h)
}

func (b *Bote) SetStartHandler(h HandlerFunc, commands ...string) {
	if len(commands) > 0 {
		for _, c := range commands {
			b.Handle(c, h)
		}
		return
	}
	b.Handle("/start", h)
}

func (b *Bote) Handle(endpoint any, f HandlerFunc) {
	b.bot.handle(endpoint, func(c tele.Context) (err error) {
		defer lang.RecoverWithErrAndStack(b.bot.log, &err)

		ctx := b.newContext(c)
		err = f(ctx)
		if err != nil {
			return ctx.handleError(err)
		}
		if c.Callback() != nil {
			c.Respond(&tele.CallbackResponse{})
		}
		return nil
	})
}

func (b *Bote) masterMiddleware(upd *tele.Update) bool {
	defer lang.Recover(b.bot.log)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	sender := getSender(upd)

	user, err := b.um.prepareUser(ctx, sender)
	if err != nil {
		b.bot.log.Error("cannot prepare user", "error", err, "user_id", sender.ID, "username", sender.Username)
		b.bot.send(sender.ID, b.msgs.Messages(b.defaultLanguageCode).GeneralError())
		return false
	}

	return b.middlewares.Range(func(mf MiddlewareFunc) bool {
		return mf(upd, user)
	})
}

func (b *Bote) cleanMiddleware(upd *tele.Update, user User) bool {
	msgIDs := user.Messages()
	if msgIDs.ErrorID > 0 {
		b.bot.delete(user.ID(), msgIDs.ErrorID)
		user.SetErrorMessage(0)
	}
	if upd.Message != nil && b.deleteMessages {
		b.bot.delete(user.ID(), upd.Message.ID)
	}

	// TODO: sanitize

	return true
}

var cbackRx = regexp.MustCompile(`^\f([-\w]+)(\|(.+))?$`)

func (b *Bote) logMiddleware(upd *tele.Update, user User) bool {
	fields := make([]any, 0, 7)
	fields = append(fields,
		"user_id", user.ID(),
		"username", user.Username(),
	)

	switch {
	case upd.Message != nil:
		fields = append(fields, "state", user.State(), "msg_id", upd.Message.ID, "text", maxLen(upd.Message.Text, MaxTextLenInLogs))
		if user.HasTextStates() {
			ts, msgID := user.LastTextState()
			fields = append(fields, "text_state", ts, "text_state_msg_id", msgID)
		}
		b.rlog.Log(MessageUpdate, fields...)

	case upd.Callback != nil:
		var (
			payload = upd.Callback.Data
			button  string
		)
		if upd.Callback.Message != nil {
			fields = append(fields, "state", user.State(upd.Callback.Message.ID), "msg_id", upd.Callback.Message.ID)
		} else {
			fields = append(fields, "state", user.State())
		}

		if match := cbackRx.FindAllStringSubmatch(payload, -1); match != nil {
			button, payload = match[0][1], match[0][3]
			button = parseBtnUnique(button)
			fields = lang.AppendIfAll(fields, "button", any(button))
		}
		fields = lang.AppendIfAll(fields, "payload", any(payload))

		b.rlog.Log(CallbackUpdate, fields...)
		return true
	}

	return true
}

func userFields(user User, fields ...any) []any {
	f := make([]any, 0, len(fields)+6)
	f = append(f, "user_id", user.ID(), "username", user.Username(), "state", user.State())
	return append(f, fields...)
}

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

func startBot(token string, cfg Config, log Logger) (*baseBot, error) {
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
			b.log.Error("error callback", "error", err, "user_id", userID)
		},
	})
	if err != nil {
		return nil, errm.Wrap(err, "new telebot")
	}
	b.bot = bot

	b.log.Info("bot is starting")

	lang.Go(b.log, b.bot.Start)

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
			b.log.Warn("message is not modified", "msg_id", msgID, "user_id", userID)
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

type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...any) {}
func (noopLogger) Info(msg string, fields ...any)  {}
func (noopLogger) Warn(msg string, fields ...any)  {}
func (noopLogger) Error(msg string, fields ...any) {}

type updateLogger struct {
	l Logger
}

func (r *updateLogger) Log(t UpdateType, fields ...any) {
	r.l.Debug(t.String(), fields...)
}

func getEditable(senderID int64, messageID int) tele.Editable {
	return &tele.Message{ID: messageID, Chat: &tele.Chat{ID: senderID}}
}

func maxLen(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func getSender(upd *tele.Update) *tele.User {
	switch {
	case upd.Callback != nil:
		return upd.Callback.Sender
	case upd.Message != nil:
		return upd.Message.Sender
	case upd.Query != nil:
		return upd.Query.Sender
	case upd.MessageReaction != nil:
		return upd.MessageReaction.User
	case upd.InlineResult != nil:
		return upd.InlineResult.Sender
	case upd.MyChatMember != nil:
		return upd.MyChatMember.Sender
	case upd.EditedMessage != nil:
		return upd.EditedMessage.Sender
	case upd.ShippingQuery != nil:
		return upd.ShippingQuery.Sender
	case upd.ChannelPost != nil:
		return upd.ChannelPost.Sender
	case upd.EditedChannelPost != nil:
		return upd.EditedChannelPost.Sender
	case upd.PreCheckoutQuery != nil:
		return upd.PreCheckoutQuery.Sender
	case upd.PollAnswer != nil:
		return upd.PollAnswer.Sender
	case upd.ChatJoinRequest != nil:
		return upd.ChatJoinRequest.Sender
	case upd.BusinessMessage != nil:
		return upd.BusinessMessage.Sender
	case upd.BusinessConnection != nil:
		return upd.BusinessConnection.Sender
	case upd.EditedBusinessMessage != nil:
		return upd.EditedBusinessMessage.Sender
	default:
		return nil
	}
}
