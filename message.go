package bote

import (
	"strconv"

	tele "gopkg.in/telebot.v3"
)

type UserID int

func (u UserID) Recipient() string {
	return strconv.Itoa(int(u))
}

func getEditable(senderID int64, messageID int) tele.Editable {
	return &tele.Message{ID: messageID, Chat: &tele.Chat{ID: senderID}}
}

// type Context struct {
// 	Callback *tele.Callback
// }

func GetCallback(m *tele.Message) *tele.Callback {
	return &tele.Callback{Sender: m.Sender, Message: m, Data: m.Text}
}
