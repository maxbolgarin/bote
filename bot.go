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

// Bot is a main struct of this package. It contains all necessary components for working with Telegram bot.
type Bot struct {
	bot  *baseBot
	um   *userManagerImpl
	msgs MessageProvider
	rlog UpdateLogger

	defaultLanguageCode string

	middlewares    *abstract.SafeSlice[MiddlewareFunc]
	deleteMessages bool
}

// Start starts the bot with optional options.
// All that you need is to pass your bot token to start it with default options.
// Don't forget to call Stop() when you're done.
func Start(ctx context.Context, token string, optsFuncs ...func(*Options)) (*Bot, error) {
	var opts Options
	for _, f := range optsFuncs {
		f(&opts)
	}
	return StartWithOptions(ctx, token, opts)
}

// StartWithOptions starts the bot with required options.
// Don't forget to call Stop() when you're done.
func StartWithOptions(ctx context.Context, token string, opts Options) (*Bot, error) {
	if token == "" {
		return nil, errm.New("token cannot be empty")
	}
	opts, err := prepareOpts(opts)
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

	bote := &Bot{
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

// Stop gracefully shuts the poller down.
func (b *Bot) Stop() {
	b.bot.bot.Stop()
}

// SetMessageProvider sets message provider.
func (b *Bot) SetMessageProvider(msgs MessageProvider) {
	b.msgs = msgs
}

// Bot returns the underlying *tele.Bot.
func (b *Bot) Bot() *tele.Bot {
	return b.bot.bot
}

// GetUser returns user by its ID.
func (b *Bot) GetUser(userID int64) User {
	return b.um.getUser(userID)
}

// GetAllUsers returns all users.
func (b *Bot) GetAllUsers() []User {
	return b.um.getAllUsers()
}

// AddMiddleware adds middleware functions that will be called on each update.
func (b *Bot) AddMiddleware(f ...MiddlewareFunc) {
	b.middlewares.Append(f...)
}

// Handle sets handler for any endpoint. Endpoint can be string or callback button.
func (b *Bot) Handle(endpoint any, f HandlerFunc) {
	b.bot.handle(endpoint, func(c tele.Context) (err error) {
		defer lang.RecoverWithErrAndStack(b.bot.log, &err)

		ctx := b.newContext(c)
		if err = f(ctx); err != nil {
			return ctx.handleError(err)
		}
		if c.Callback() != nil {
			c.Respond(&tele.CallbackResponse{})
		}
		return nil
	})
}

// HandleText sets handler for text messages.
// You should provide a single handler for all text messages, that will call another handlers based on the state.
func (b *Bot) HandleText(f HandlerFunc) {
	b.bot.handle(tele.OnText, func(c tele.Context) (err error) {
		defer lang.RecoverWithErrAndStack(b.bot.log, &err)

		ctx := b.newContext(c)
		if !ctx.user.hasTextMessages() {
			return nil
		}
		ctx.textMsgID = ctx.user.lastTextMessage()
		return ctx.handleError(f(ctx))
	})
}

// HandleStart sets handler for start command.
func (b *Bot) HandleStart(h HandlerFunc, commands ...string) {
	if len(commands) > 0 {
		for _, c := range commands {
			b.Handle(c, h)
		}
		return
	}
	b.Handle("/start", h)
}

func (b *Bot) masterMiddleware(upd *tele.Update) bool {
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

func (b *Bot) cleanMiddleware(upd *tele.Update, userRaw User) bool {
	user := userRaw.(*userContextImpl)

	msgIDs := user.Messages()
	if msgIDs.ErrorID > 0 {
		b.bot.delete(user.ID(), msgIDs.ErrorID)
		user.setErrorMessage(0)
	}
	if upd.Message != nil && b.deleteMessages {
		b.bot.delete(user.ID(), upd.Message.ID)
	}

	// TODO: sanitize

	return true
}

var cbackRx = regexp.MustCompile(`^\f([-\w]+)(\|(.+))?$`)

func (b *Bot) logMiddleware(upd *tele.Update, userRaw User) bool {
	user := userRaw.(*userContextImpl)

	fields := make([]any, 0, 7)
	fields = append(fields,
		"user_id", user.ID(),
		"username", user.Username(),
	)

	switch {
	case upd.Message != nil:
		fields = append(fields, "state", user.StateMain(), "msg_id", upd.Message.ID, "text", maxLen(upd.Message.Text, MaxTextLenInLogs))
		if user.hasTextMessages() {
			msgID := user.lastTextMessage()
			ts, _ := user.State(msgID)
			fields = append(fields, "text_state", ts, "text_state_msg_id", msgID)
		}
		b.rlog.Log(MessageUpdate, fields...)

	case upd.Callback != nil:
		var (
			payload = upd.Callback.Data
			button  string
		)
		if upd.Callback.Message != nil {
			st, _ := user.State(upd.Callback.Message.ID)
			fields = append(fields, "state", st, "msg_id", upd.Callback.Message.ID)
		} else {
			fields = append(fields, "state", user.StateMain())
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
	f = append(f, "user_id", user.ID(), "username", user.Username(), "state", user.StateMain())
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

func (b *baseBot) deleteHistory(userID int64, lastMessageID int) map[int]struct{} {
	deleted := map[int]struct{}{}
	var counter int
	for msgID := lastMessageID - 1; msgID > 1; msgID-- {
		err := b.delete(userID, msgID)
		if err != nil {
			counter += 1
		} else {
			counter = 0
			deleted[msgID] = struct{}{}
		}
		if counter == 5 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return deleted
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
