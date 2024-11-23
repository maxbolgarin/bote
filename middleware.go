package bote

import (
	tele "gopkg.in/telebot.v3"
)

// MiddlewareFunc executes at every bot reqeust, msg is nil if it is a callback
type (
	MiddlewareFunc         func(upd *tele.Update) bool
	MiddlewareMessageFunc  func(msg *tele.Message) bool
	MiddlewareCallbackFunc func(cb *tele.Callback) bool
)
