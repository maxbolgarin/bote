package bote

import (
	"context"
	"log/slog"
	"os"
	"regexp"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/contem"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

type Bote struct {
	bot  *baseBot
	um   *userManagerImpl
	msgs MessageProvider
	rlog UpdateLogger

	defaultLanguageCode string

	middlewares    *abstract.SafeSlice[MiddlewareFunc]
	deleteMessages bool
}

func Start(ctx contem.Context, token string, optsRaw ...Options) (*Bote, error) {
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

	b, err := startBot(ctx, token, opts.Config, opts.Logger)
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
			return b.handleError(ctx, err)
		}
		if c.Callback() != nil {
			c.Respond(&tele.CallbackResponse{})
		}
		return nil
	})
}

func (b *Bote) masterMiddleware(upd *tele.Update) bool {
	defer lang.Recover(b.bot.log)

	sender := GetSender(upd)
	if sender == nil {
		panic("sender cannot be nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

func (b *Bote) handleError(ctx Context, err error) error {
	user := ctx.User()

	if IsBlockedError(err) {
		b.bot.log.Info("bot is blocked, disable", userFields(user)...)
		user.Disable()
		b.um.removeUserFromMemory(ctx.User().ID())
		return nil
	}

	// TODO: handle other errors
	if IsNotFoundEditMsgErr(err) {
		b.bot.log.Warn("message not found", userFields(user)...)
		//return app.StartHandlerByUser(user, "")
		// user.ForgetHistoryDay(day)
		return nil
	}

	b.bot.log.Error("handler error", userFields(user, "error", err)...)

	msgID, _ := b.bot.send(user.ID(), b.msgs.Messages(b.defaultLanguageCode).GeneralError())
	user.SetErrorMessage(msgID)

	return nil
}

func userFields(user User, fields ...any) []any {
	f := make([]any, 0, len(fields)+4)
	f = append(f, "user_id", user.ID(), "username", user.Username())
	return append(f, fields...)
}

func prepareOpts(optsRaw ...Options) (Options, error) {
	opts := lang.First(optsRaw)

	err := opts.Config.prepareAndValidate()
	if err != nil {
		return opts, errm.Wrap(err, "prepare and validate config")
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: lang.If(opts.Config.Debug, slog.LevelDebug, slog.LevelInfo),
		}))
		if opts.UpdateLogger == nil && !opts.Config.Debug {
			opts.UpdateLogger = &updateLogger{slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))}
		}
	}
	if opts.UpdateLogger == nil {
		opts.UpdateLogger = &updateLogger{opts.Logger}
	}
	if !*opts.Config.EnableLogging {
		opts.Logger = noopLogger{}
	}

	if opts.UserDB == nil {
		opts.UserDB, err = newInMemoryUserStorage()
		if err != nil {
			return opts, errm.Wrap(err, "new user storage")
		}
	}
	if opts.Msgs == nil {
		opts.Msgs = NewDefaultMessageProvider()
	}

	return opts, nil
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

func maxLen(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
