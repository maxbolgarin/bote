package bote

import (
	"context"

	tele "gopkg.in/telebot.v3"
)

type Context[App any] interface {
	User() User
	Tele() tele.Context
	Data() string
	App() App
	Context() context.Context

	Set(key, value string)
	Get(key string) string

	Send(s State, msgMain, msgHead string, kbMain, kbHead *tele.ReplyMarkup, opts ...any) error
	SendMain(s State, msg string, kb *tele.ReplyMarkup, opts ...any) error
	SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error
	SendError(msg string, opts ...any) error

	Edit(s State, msgMain, msgHead string, kbMain, kbHead *tele.ReplyMarkup, opts ...any) error
	EditMain(s State, msg string, kb *tele.ReplyMarkup, opts ...any) error
	EditAny(s State, msgID int64, msg string, kb *tele.ReplyMarkup, opts ...any) error
	EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error
	EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error

	Delete(msgIDs ...int64) error
	DeleteHistory(lastMessageID int64)
}

func (b *Bote[T]) NewContext(c tele.Context) Context[T] {
	return nil
}
