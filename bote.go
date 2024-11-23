package bote

import (
	"context"
	"time"

	"github.com/maxbolgarin/contem"
	tele "gopkg.in/telebot.v3"
)

type manager interface {
	// PrepareUser checks if user is in memory or DB and creates it if it is a new user.
	PrepareUser(ctx context.Context, userID int64, username, firstName, lastName string) (UserModel, error)
	// GetUser returns user by ID, use it only after PrepareUser.
	GetUser(userID int64) UserModel
	// GetAllUsers returns all currently loaded users.
	GetAllUsers() []UserModel
	// RemoveUserFromMemory removes user from memory cache (e.g. if user disables bot).
	RemoveUserFromMemory(userID int64)

	// GetTimezone parses city from string and returns timezone.
	GetTimezone(city string) (*time.Location, error)
}

type HandlerFunc[T any] func(c Context[T]) error

type Bote[T any] struct {
	*BaseBot
	msgs MessageProvider
}

func New[T any](ctx contem.Context, cfg Config) (*Bote[T], error) {
	b, err := NewBase(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Bote[T]{BaseBot: b}, nil
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
		//b.m.RemoveUserFromMemory(ctx.User().ID())
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
