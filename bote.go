package bote

import (
	"github.com/maxbolgarin/contem"
	tele "gopkg.in/telebot.v4"
)

type HandlerFunc[T any] func(c Context[T]) error

type Bote[T any] struct {
	*BaseBot
	msgs MessageProvider // TODO
	um   userManagerImpl
}

func New[T any](ctx contem.Context, cfg Config) (*Bote[T], error) {
	b, err := NewBase(ctx, cfg)
	if err != nil {
		return nil, err
	}
	bote := &Bote[T]{BaseBot: b}

	b.AddMiddleware(bote.masterMiddleware)

	return bote, nil
}

func (b *Bote[T]) NewHandler(f HandlerFunc[T]) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx := b.NewContext(c)
		err := f(ctx)
		if err != nil {
			return b.handleError(ctx, err)
		}
		if c.Callback() != nil {
			c.Respond(&tele.CallbackResponse{})
		}
		return nil
	}
}

func (b *Bote[T]) handleError(ctx Context[T], err error) error {
	user := ctx.User()

	if IsBlockedError(err) {
		b.log.Info("bot is blocked, disable", userFields(user)...)
		user.Disable(ctx.Context())
		b.um.removeUserFromMemory(ctx.User().ID())
		return nil
	}

	if IsNotFoundEditMsgErr(err) {
		b.log.Warn("message not found, recovery", userFields(user)...)
		//return app.StartHandlerByUser(user, "")
		// user.ForgetHistoryDay(day)
		return nil
	}

	b.log.Err(err, userFields(user)...)

	msgID := b.Send(user.ID(), b.msgs.ErrorMsg())
	user.SetErrorMessage(msgID)

	return nil
}

func userFields(user User, fields ...any) []any {
	f := make([]any, 0, len(fields)+4)
	f = append(f, "user_id", user.ID(), "username", user.Username())
	return append(f, fields...)
}

func (b *Bote[T]) masterMiddleware(upd *tele.Update) bool {
	// TODO: recover

	// sender := getSender(upd)
	// if sender == nil {
	// 	return false
	// }

	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()

	// user, err := b.um.createUser(ctx, sender)
	// if err != nil {
	// 	b.log.Err(err, "method", "PrepareUser", "user_id", sender.ID, "username", sender.Username)
	// 	b.Send(sender.ID, b.msgs.ErrorMsg())
	// 	return false
	// }

	// bundle := messageBundle{msg, cb}
	// middlewares := []middlewareFunc{
	// 	app.cleanMiddleware,
	// 	app.sanitizeMiddleware,
	// 	app.logMiddleware,
	// 	app.authMiddleware,
	// }

	// for _, m := range middlewares {
	// 	if ok := m(&bundle, user); !ok {
	// 		return false
	// 	}
	// }

	return true
}

// func (b *Bote[T]) cleanMiddleware(b *messageBundle, user master.User) bool {
// 	msgIDs := user.Messages()
// 	if msgIDs.Error > 0 {
// 		app.Delete(user.ID(), msgIDs.Error)
// 		user.SetErrorMessage(0)
// 	}
// 	if b.msg != nil {
// 		app.Delete(user.ID(), b.msg.ID)
// 	}
// 	return true
// }

// func (b *Bote[T]) sanitizeMiddleware(b *messageBundle, user master.User) bool {
// 	if b.msg == nil {
// 		return true
// 	}

// 	text := b.msg.Text
// 	defer func() {
// 		b.msg.Text = text
// 	}()

// 	// Sanitize MongoDB operators and XSS scripts.
// 	text = sanitize.Custom(text, `(\$[a-zA-Z\[\]]+)`)
// 	text = sanitize.XSS(text)

// 	var (
// 		maxLen       = MaxThoughtLen
// 		_, textState = user.LastTextState()
// 	)

// 	if !textState.AllowLongMessage() {
// 		maxLen = MaxMessageTextLen

// 		// Some inputs may be mongodb keys, which can't contains $ or . signs.
// 		text = sanitize.Custom(text, `\$`)
// 		text = strings.ReplaceAll(text, ".", " ")
// 		text = sanitize.SingleLine(text)
// 		text = strings.TrimSpace(text)
// 	}

// 	if len(text) > maxLen {
// 		if textState.IsChanged() {
// 			app.sendErrorMsg(user, msgs.EntityTooLongError)
// 		}
// 		return false
// 	}

// 	return true
// }

// var cbackRx = regexp.MustCompile(`^\f([-\w]+)(\|(.+))?$`)

// func (b *Bote[T]) logMiddleware(b *messageBundle, user master.User) bool {
// 	switch {
// 	case b.msg != nil:
// 		fields := make([]any, 0, 6)
// 		fields = append(fields, "state", user.LastState())
// 		day, textState := user.LastTextState()
// 		if textState.IsChanged() {
// 			fields = append(fields, "text_state", textState, "text_state_day", day.String())
// 		}
// 		maxlog.LogRequest(user.ID(), user.Username(), b.msg.ID, b.msg.Text, fields...)

// 	case b.cb != nil:
// 		var (
// 			match   = cbackRx.FindAllStringSubmatch(b.cb.Data, -1)
// 			payload = b.cb.Data
// 			button  string
// 		)
// 		if match != nil {
// 			button, payload = match[0][1], match[0][3]
// 			button = maxbot.ParseUniqueWithRand(button)
// 		}
// 		maxlog.LogCallback(user.ID(), user.Username(), b.cb.Message.ID, button, payload, "state", user.LastState())

// 	default:
// 		return false
// 	}

// 	return true
// }

// func (b *Bote[T]) authMiddleware(b *messageBundle, user master.User) bool {
// 	defer func() {
// 		user.UpdateLastSeen(timestat.EmptyDate)
// 	}()

// 	var newState state.State
// 	switch user.State() {
// 	case state.Unknown, state.FirstRequest:
// 		newState = app.NotRegistered(user, "")

// 	default:
// 		if b.msg == nil || b.msg.Text != "/start" {
// 			return true
// 		}
// 		newState = app.StartHandlerByUser(user, "")
// 	}

// 	user.SetState(newState)

// 	return false
// }

func getSender(upd *tele.Update) *tele.User {
	switch {
	case upd.Callback != nil:
		return upd.Callback.Sender
	case upd.Message != nil:
		return upd.Message.Sender
	case upd.MyChatMember != nil:
		return upd.MyChatMember.Sender
	case upd.EditedMessage != nil:
		return upd.EditedMessage.Sender
	case upd.Query != nil:
		return upd.Query.Sender
	default:
		return nil
	}
}

type messageBundle struct {
	msg *tele.Message
	cb  *tele.Callback
}
