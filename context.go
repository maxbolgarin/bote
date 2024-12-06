package bote

import (
	"context"

	tele "gopkg.in/telebot.v4"
)

type Context interface {
	User() User
	Tele() tele.Context
	Data() string
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

func (b *Bote) newContext(c tele.Context) Context {
	return &contextImpl{
		b: b,
		c: c,
	}
}

type contextImpl struct {
	b *Bote
	c tele.Context
}

func (c *contextImpl) User() User {
	upd := c.c.Update()
	return c.b.um.getUser(GetSender(&upd).ID)
}

func (c *contextImpl) Tele() tele.Context {
	return c.c
}

func (c *contextImpl) Data() string {
	if cb := c.c.Callback(); cb != nil {
		return cb.Data
	}
	if msg := c.c.Message(); msg != nil {
		return msg.Text
	}
	return ""
}

func (c *contextImpl) Context() context.Context {
	return context.TODO()
}

func (c *contextImpl) Set(key, value string) {
	c.c.Set(key, value)
}

func (c *contextImpl) Get(key string) string {
	return c.c.Get(key).(string)
}

func (c *contextImpl) Send(s State, msgMain, msgHead string, kbMain, kbHead *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) SendMain(s State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) SendNotification(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	return c.c.Send(msg, append(opts, kb)...)
}

func (c *contextImpl) SendError(msg string, opts ...any) error {
	return c.c.Send(msg, append(opts, tele.NoPreview)...)
}

func (c *contextImpl) Edit(s State, msgMain, msgHead string, kbMain, kbHead *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) EditMain(s State, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) EditAny(s State, msgID int64, msg string, kb *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) EditHead(msg string, kb *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) EditHeadReplyMarkup(kb *tele.ReplyMarkup, opts ...any) error {
	return nil
}

func (c *contextImpl) Delete(msgIDs ...int64) error {
	return nil
}

func (c *contextImpl) DeleteHistory(lastMessageID int64) {
}
