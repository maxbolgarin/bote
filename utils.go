package bote

import (
	"strings"

	"github.com/maxbolgarin/errm"
	tele "gopkg.in/telebot.v4"
)

func GetSender(upd *tele.Update) *tele.User {
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

func ParseCallbackData(data string) []string {
	return strings.Split(data, "|")
}

func ParseDoubleCallbackData(data string) (string, string) {
	spl := strings.Split(data, "|")
	if len(spl) < 2 {
		return spl[0], ""
	}
	return spl[0], spl[1]
}

func ParseLastItemFromData(data string) string {
	spl := strings.Split(data, "|")
	return spl[len(spl)-1]
}

// PrepareNumberInput returns first numeric symbols from string, so for `332fdqa` -> `332`.
func PrepareNumberInput(s string, isDecimal bool) string {
	for i := range s {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if isDecimal && s[i] == '.' {
			continue
		}
		return s[:i]
	}
	return s
}

func IsNotFoundEditMsgErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message to edit not found")
}

func IsInvalidArgument(err error) bool {
	if err == nil {
		return false
	}
	return errm.Is(err, errEmptyUserID) || errm.Is(err, errEmptyMsgID)
}

func IsBlockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "bot was blocked by the user")
}
