package bote

import (
	"strconv"
	"time"

	"github.com/maxbolgarin/abstract"
	tele "gopkg.in/telebot.v4"
)

// UserInfo contains user info, that can be obtained from Telegram.
type UserInfo struct {
	ID           int64  `bson:"id"`
	FirstName    string `bson:"first_name"`
	LastName     string `bson:"last_name"`
	Username     string `bson:"username"`
	LanguageCode string `bson:"language_code"`
	IsBot        bool   `bson:"is_bot"`
	IsPremium    bool   `bson:"is_premium"`
}

// UserMessages contains IDs of user messages.
type UserMessages struct {
	// Main message is the last and primary one in the chat.
	Main int `bson:"main_id"`
	// Head message is sent right before main message for making bot more interactive.
	Head int `bson:"head_id"`
	// Notification message can be sent in any time and deleted after some time.
	Notification int `bson:"notification_id"`
	// Error message can be sent in any time in case of error and deleted automically after next action.
	Error int `bson:"error_id"`
	// Info message is the previous main messages. Main message becomes Info after new Main sending.
	Info []int `bson:"info_id"`
}

// UserState contains current user state and state history.
// State connects to message, every Main and Info message has its own state.
type UserState struct {
	// Main is the main state of the user, state of the Main message.
	Main State `bson:"main"`
	// States contains all states of the user (for all Info messages). It is a map of message_id -> state.
	States abstract.Map[int, State] `bson:"states"`
	// TextStates contains all text states of the user.
	// Every message can produce text state and they should be handled as LIFO.
	TextStates *abstract.UniqueStack[TextStateWithMessage] `bson:"text_states"`
	// ActionsHistory contains time of last interaction of user with every message.
	ActionsHistory abstract.Map[int, time.Time] `bson:"actions_history"`
	// LastSeen is the time of last interaction with bot.
	LastSeen time.Time `bson:"last_seen"`
}

type TextStateWithMessage struct {
	MessageID int    `bson:"message_id"`
	State     string `bson:"state"`
}

func (u UserInfo) IDString() string {
	return strconv.FormatInt(u.ID, 10)
}

func (m *UserMessages) SetMessages(messages ...int) {
	msgs := make([]int, 4)
	copy(msgs, messages)
	m.Main = msgs[0]
	m.Head = msgs[1]
	m.Notification = msgs[2]
	m.Error = msgs[3]
}

// Structure that represents user in DB.
type userModel struct {
	Info UserInfo `bson:"info"`

	Messages UserMessages `bson:"messages"`
	State    UserState    `bson:"state"`

	Created    time.Time `bson:"created"`
	Registered time.Time `bson:"registered"`
	IsDisabled bool      `bson:"is_disabled"`
}

func newUserModel(tUser *tele.User) userModel {
	return userModel{
		Info: newUserInfo(tUser),
		State: UserState{
			Main:           NotRegistered,
			States:         abstract.NewMap[int, State](),
			ActionsHistory: abstract.NewMap[int, time.Time](),
			TextStates:     abstract.NewUniqueStack[TextStateWithMessage](),
			LastSeen:       time.Now().UTC(),
		},
		Created: time.Now().UTC(),
	}
}

func newUserInfo(tUser *tele.User) UserInfo {
	return UserInfo{
		ID:           tUser.ID,
		FirstName:    tUser.FirstName,
		LastName:     tUser.LastName,
		Username:     tUser.Username,
		LanguageCode: tUser.LanguageCode,
		IsBot:        tUser.IsBot,
		IsPremium:    tUser.IsPremium,
	}
}
