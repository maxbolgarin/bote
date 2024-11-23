package bote

import (
	"context"
	"time"
)

type User interface {
	ID() int64
	Username() string
	Model() UserModel

	SetErrorMessage(msgID int)

	Disable(ctx context.Context)
}

type UserModel struct {
	ID           int64    `bson:"id"`
	FirstName    string   `bson:"first_name"`
	LastName     string   `bson:"last_name"`
	Username     string   `bson:"username"`
	LanguageCode string   `bson:"language_code"`
	IsBot        bool     `bson:"is_bot"`
	IsPremium    bool     `bson:"is_premium"`
	Usernames    []string `bson:"active_usernames"`

	Messages UserMessages `bson:"messages"`
	State    UserState    `bson:"state"`

	Created    time.Time `bson:"created"`
	Registered time.Time `bson:"registered"`
	// Settings   UserSettings `bson:"settings"`

	IsDisabled bool `bson:"is_disabled"`

	IDString string `bson:"-"`
}

type UserMessages struct {
	Main         int64 `bson:"main_id"`
	Head         int64 `bson:"head_id"`
	Notification int64 `bson:"notification_id"`
	Error        int64 `bson:"error_id"`

	Info []int64 `bson:"info_id"` // all main becomes info after new main sending
}

type UserState struct {
	Main   State           `bson:"main"`
	States map[int64]State `bson:"states"` // message_id -> state

	TextStates *stackImpl[textStateWithMessage] `bson:"text_states"`

	// Time of last interaction with bot.
	LastSeen       time.Time           `bson:"last_seen"`
	ActionsHistory map[int64]time.Time `bson:"actions_history"`
}

type userImpl struct {
	user    UserModel
	db      *Collection
	asyncDB *AsyncCollection
}

func (u *userImpl) ID() int64 {
	return u.user.ID
}

func (u *userImpl) Username() string {
	return u.user.Username
}

func (u *userImpl) Model() UserModel {
	return u.user
}

func (m *UserMessages) SetMessages(messages ...int64) {
	msgs := make([]int64, 4)
	copy(msgs, messages)
	m.Main = msgs[0]
	m.Head = msgs[1]
	m.Notification = msgs[2]
	m.Error = msgs[3]
}

type textStateWithMessage struct {
	MessageID int64  `bson:"message_id"`
	State     string `bson:"state"`
}

func (u *userImpl) Disable(ctx context.Context) {
	if u.user.IsDisabled {
		return
	}

	u.user.IsDisabled = true
	u.user.State.Main = NotRegistered

	u.db.SetFields(ctx, Filter{
		"id": u.user.ID,
	}, Updates{
		"disabled":   true,
		"state_main": NotRegistered,
	})
}

// type UserSettings struct {
// 	// Additional settings
// 	// Timezone stores user's current timezone.
// 	Timezone timestat.Timezone `bson:"timezone"`
// 	// DayStartTime is time when `day starts` -> send new message and change `today`.
// 	// Is constraint from 01:00 to 12:00, default 4:00.
// 	DayStartTime timestat.Time `bson:"day_start_time"`
// 	// AskSummaryTime - time when make summary button active.
// 	AskSummaryTime timestat.Time `bson:"ask_summary_time"`
// 	// ActiveDuration shows how long user should be AFK to mark him as not active now.
// 	ActiveDuration time.Duration `bson:"active_duration"`
// 	// ForceWakeUp shows time when bot can go to main menu from wake message.
// 	ForceWakeUp timestat.Time `bson:"force_wake_up"`
// 	// MainMenuReturn shows how long user should be AFK to return to main menu.
// 	MainMenuReturn time.Duration `bson:"main_menu_return"`
// 	// IsNewDaySilent sets if send message with new day without notification.
// 	IsNewDaySilent bool `bson:"is_new_day_silent"`

// 	// Summary settings
// 	// IsAskTaskTransfer - ask user about transferring tasks to tomorrow in summary.
// 	IsAskTaskTransfer bool `bson:"is_ask_task_transfer"`
// 	// AutoTransferAllTasks - transfer all not completed tasks automatically.
// 	AutoTransferAllTasks bool `bson:"auto_transfer_all_tasks"`
// 	// IsAskDayScore - ask user about "day score" in summary.
// 	IsAskDayMood bool `bson:"is_ask_day_mood"`
// 	// IsAskDayScore - ask user about "day score" in summary.
// 	IsAskDayScore bool `bson:"is_ask_day_score"`
// 	// IsAskDayThought - ask user about "day thought" in summary.
// 	IsAskDayThought bool `bson:"is_ask_day_thought"`

// 	// Sleep settings
// 	// IsAskGetUp - ask user about "when you got up from the bed".
// 	IsAskGetUp bool `bson:"is_ask_get_up"`
// 	// IsAskMorningMood - ask user about OR general morning mood OR how hard the getting up was.
// 	IsAskMorningMood bool `bson:"is_ask_morning_mood"`
// 	// FallAsleepDuration is expected time from pressing button "go to sleep" and real
// 	// falling asleep. Default: 15m.
// 	FallAsleepDuration time.Duration `bson:"fall_asleep_duration"`
// }

// func (s *User) SetTimezone(tz *time.Location) {
// 	s.Settings.Timezone = timestat.NewTimezone(tz)
// }

// func (u User) String() string {
// 	return "[@" + u.Username + "|" + strconv.Itoa(int(u.ID)) + "]"
// }

// func (s UserSettings) String() string {
// 	out := message.Builder{}
// 	out.WriteString("User Settings\n")
// 	out.Writef("Timezone:\t\t%s\n", s.Timezone)
// 	out.Writef("DayStartTime:\t\t%s\n", s.DayStartTime)
// 	out.Writef("AskSummaryTime:\t\t%s\n", s.AskSummaryTime)
// 	out.Writef("ActiveDuration:\t\t%s\n", s.ActiveDuration)
// 	out.Writef("ForceWakeUp:\t\t%s\n", s.ForceWakeUp)
// 	out.Writef("MainMenuReturn:\t\t%s\n", s.MainMenuReturn)
// 	out.Writef("IsNewDaySilent:\t\t%t\n", s.IsNewDaySilent)
// 	out.Writef("IsAskTaskTransfer:\t%t\n", s.IsAskTaskTransfer)
// 	out.Writef("AutoTransferAllTasks:\t%t\n", s.AutoTransferAllTasks)
// 	out.Writef("IsAskDayMood:\t\t%t\n", s.IsAskDayMood)
// 	out.Writef("IsAskDayScore:\t\t%t\n", s.IsAskDayScore)
// 	out.Writef("IsAskDayThought:\t%t\n", s.IsAskDayThought)
// 	out.Writef("IsAskGetUp:\t\t%t\n", s.IsAskGetUp)
// 	out.Writef("IsAskMorningMood:\t%t\n", s.IsAskMorningMood)
// 	out.Writef("FallAsleepDuration:\t%s\n", s.FallAsleepDuration)
// 	return out.String()
// }

// func (s UserSettings) ToggleByName(name string) UserSettings {
// 	switch name {
// 	case IsAskTaskTransferField:
// 		s.IsAskTaskTransfer = !s.IsAskTaskTransfer
// 	case AutoTransferAllTasksField:
// 		s.AutoTransferAllTasks = !s.AutoTransferAllTasks
// 	case IsAskDayMoodField:
// 		s.IsAskDayMood = !s.IsAskDayMood
// 	case IsAskDayScoreField:
// 		s.IsAskDayScore = !s.IsAskDayScore
// 	case IsAskDayThoughtField:
// 		s.IsAskDayThought = !s.IsAskDayThought
// 	case IsAskGetUpField:
// 		s.IsAskGetUp = !s.IsAskGetUp
// 	case IsAskMorningMoodField:
// 		s.IsAskMorningMood = !s.IsAskMorningMood
// 	}
// 	return s
// }

// func GetDefaultUser(userID int64) User {
// 	user := User{
// 		ID: userID,
// 		State: UserState{
// 			Current:        state.FirstRequest,
// 			History:        abstract.NewDateMapDB[state.State](),
// 			ActionsHistory: abstract.NewDateMapDB[time.Time](),
// 			TextStates:     abstract.NewStackAdvancedDB[state.StateWithDate](),
// 			LastSeen:       time.Now().UTC(),
// 		},
// 		Created:  time.Now().UTC(),
// 		Settings: GetDefaultUserSettings(),
// 		Messages: UserMessages{
// 			History: abstract.NewDateMapDB[int](),
// 		},
// 	}
// 	return user
// }

// func GetDefaultUserSettings() UserSettings {
// 	return settings
// }

// var settings = UserSettings{
// 	DayStartTime:       timestat.NewTime(4, 0),
// 	FallAsleepDuration: 30 * time.Minute,
// 	IsAskGetUp:         true,
// 	IsAskMorningMood:   true,
// 	ForceWakeUp:        timestat.NewTime(12, 0),

// 	IsAskTaskTransfer:    true,
// 	AutoTransferAllTasks: false,
// 	IsAskDayMood:         true,
// 	IsAskDayScore:        true,
// 	IsAskDayThought:      true,
// 	//AskSummaryTime:       timestat.NewTime(18, 00),

// 	ActiveDuration: 5 * time.Minute,
// 	MainMenuReturn: 30 * time.Minute,
// 	IsNewDaySilent: true,
// }

// func init() {
// 	loc, _ := timestat.ParseUTCOffset("+3")
// 	settings.Timezone = timestat.NewTimezone(loc)
// }

// func InitDefaultSettings(s UserSettings) {
// 	settings = s
// }

// const (
// 	UserIDField              = "user_id"
// 	UsernameField            = "username"
// 	FirstNameField           = "first_name"
// 	LastNameField            = "last_name"
// 	StateCurrentField        = "state.current"
// 	StateHistoryField        = "state.history"
// 	StateTextStatesField     = "state.text_states"
// 	StateLastSeenField       = "state.last_seen"
// 	StateActionsHistoryField = "state.actions_history"
// 	RegisteredField          = "registered"
// 	MessagesField            = "messages"
// 	MainMessageIDField       = "messages.main_id"
// 	HeaderMessageIDField     = "messages.head_id"
// 	ErrorMessageIDField      = "messages.error_id"
// 	InfoMessageIDField       = "messages.info_id"
// 	HistoryMessageIDsField   = "messages.history_ids"

// 	SettingsField = "settings"
// 	TimezoneField = "settings.timezone"

// 	IsDisabledField = "is_disabled"
// )

// const (
// 	IsAskTaskTransferField    = "is_ask_task_transfer"
// 	AutoTransferAllTasksField = "auto_transfer_all_tasks"
// 	IsAskDayMoodField         = "is_ask_day_mood"
// 	IsAskDayScoreField        = "is_ask_day_score"
// 	IsAskDayThoughtField      = "is_ask_day_thought"
// 	IsAskGetUpField           = "is_ask_get_up"
// 	IsAskMorningMoodField     = "is_ask_morning_mood"
// )

// func (user *User) PrepareUserAfterDB() {
// 	if user.State.History.DateMap == nil {
// 		user.State.History = abstract.NewDateMapDB[state.State]()
// 	}
// 	if user.State.ActionsHistory.DateMap == nil {
// 		user.State.ActionsHistory = abstract.NewDateMapDB[time.Time]()
// 	}
// 	if user.Messages.History.DateMap == nil {
// 		user.Messages.History = abstract.NewDateMapDB[int]()
// 	}
// 	if user.State.TextStates.StackAdvanced == nil {
// 		user.State.TextStates = abstract.NewStackAdvancedDB[state.StateWithDate]()
// 	}
// }
