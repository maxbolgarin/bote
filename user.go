package bote

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	"github.com/maypok86/otter"
	tele "gopkg.in/telebot.v4"
)

// State is a user state in Telegram bot builded using this package.
// User changes states when he makes actions, e.g. sends a message, clicks a button, etc.
// State is connected to message, every Main and Info (history) message has its own state.
// State is necessary for understanding user behavior and should be used in user init after bot restarting.
// You should create states as constants in your application and pass it in Send or Edit methods as first argument.
// States is generally changes in response to user actions inside handlers,
// but also can be changed in other places in case of any action.
type State interface {
	fmt.Stringer
	IsText() bool
	NotChanged() bool
}

// User is an interface that represents user context in the bot.
type User interface {
	// ID is Telegram user ID.
	ID() int64
	// Username returns Telegram username (without @).
	Username() string
	// Language returns Telegram user language code.
	Language() string
	// Model returns user model.
	Model() UserModel
	// Info returns user info.
	Info() UserInfo
	// State returns current state for the given message ID.
	State(msgID int) (State, bool)
	// StateMain returns state for the Main message.
	StateMain() State
	// Messages returns all message IDs.
	Messages() UserMessages
	// LastSeenTime returns the time when user interacts with provided message.
	// If message ID is not provided, it returns the time when user interacts with bot's any message.
	LastSeenTime(optionalMsgID ...int) time.Time
	// IsDisabled returns true if user disabled the bot.
	IsDisabled() bool
	// String returns user string representation in format '[@username|id]'.
	String() string
}

// UsersStorage is a storage for users.
// You should implement it in your application if you want to persist users between applicataion restarts.
type UsersStorage interface {
	// Insert inserts user in storage.
	Insert(ctx context.Context, userModel UserModel) error
	// Find returns user from storage. It returns true as a second argument if user was found without error.
	Find(ctx context.Context, id int64) (UserModel, bool, error)

	// UpdateAsync updates user model in storage. The idea of that method is that it calls on every user action
	// (e.g. for update state), so it should be async to make it faster for user (without IO latency).
	// So this method doesn't accept context and doesn't return error because it should be called in async goroutine.
	//
	// Warning! You can't use simple worker pool, because updates should be ordered. If you don't want to
	// make it async, you may use sync operation in this method and handle error using channels, for example.
	// You may be intersting in https://github.com/maxbolgarin/mongox for async operations in MongoDB or
	// https://github.com/maxbolgarin/gorder for general async queue if you use another db.
	//
	// If you want stable work of bote package, don't update user model by yourself. Bote will do it for you.
	// If you want to expand user model by your additional fields, create an another table/collection in your db.
	UpdateAsync(id int64, userModel *UserModelDiff)
}

// UserIDDBFieldName is a field name for user ID in DB.
const UserIDDBFieldName = "id"

// UserModel is a structure that represents user in DB.
type UserModel struct {
	// ID is Telegram user ID.
	ID int64 `bson:"id" json:"id" db:"id"`
	// Info contains user info, that can be obtained from Telegram.
	Info UserInfo `bson:"info" json:"info" db:"info"`
	// Messages contains IDs of user messages.
	Messages UserMessages `bson:"messages" json:"messages" db:"messages"`
	// State contains state for every user's message.
	State UserState `bson:"state" json:"state" db:"state"`
	// LastSeenTime is the last time user interacted with the bot.
	LastSeenTime time.Time `bson:"last_seen_time" json:"last_seen_time" db:"last_seen_time"`
	// CreatedTime is the time when user was created.
	CreatedTime time.Time `bson:"created_time" json:"created_time" db:"created_time"`
	// DisabledTime is the time when user was disabled.
	DisabledTime time.Time `bson:"disabled_time" json:"disabled_time" db:"disabled_time"`
	// IsDisabled returns true if user is disabled. Disabled means that user blocks bot.
	IsDisabled bool `bson:"is_disabled" json:"is_disabled" db:"is_disabled"`
}

// UserInfo contains user info, that can be obtained from Telegram.
type UserInfo struct {
	// FirstName is Telegram user first name.
	FirstName string `bson:"first_name" json:"first_name" db:"first_name"`
	// LastName is Telegram user last name.
	LastName string `bson:"last_name" json:"last_name" db:"last_name"`
	// Username is Telegram username (without @).
	Username string `bson:"username" json:"username" db:"username"`
	// LanguageCode is Telegram user language code.
	LanguageCode string `bson:"language_code" json:"language_code" db:"language_code"`
	// IsBot is true if Telegram user is a bot.
	IsBot bool `bson:"is_bot" json:"is_bot" db:"is_bot"`
	// IsPremium is true if Telegram user has Telegram Premium.
	IsPremium bool `bson:"is_premium" json:"is_premium" db:"is_premium"`
}

// UserMessages contains IDs of user messages.
type UserMessages struct {
	// Main message is the last and primary one in the chat.
	MainID int `bson:"main_id" json:"main_id" db:"main_id"`
	// Head message is sent right before main message for making bot more interactive.
	HeadID int `bson:"head_id" json:"head_id" db:"head_id"`
	// Notification message can be sent in any time. Old notification message will be deleted when new one is sent.
	NotificationID int `bson:"notification_id" json:"notification_id" db:"notification_id"`
	// Error message can be sent in any time in case of error and deleted automically after next action.
	ErrorID int `bson:"error_id" json:"error_id" db:"error_id"`
	// History message is the previous main messages. Main message becomes History message after new Main sending.
	HistoryIDs []int `bson:"history_ids" json:"history_ids" db:"history_ids"`
	// LastActions contains time of last interaction of user with every message.
	LastActions map[int]time.Time `bson:"last_actions" json:"last_actions" db:"last_actions"`
}

// UserState contains current user state and state history.
// State connects to message, every Main and Info message has its own state.
type UserState struct {
	// Main is the main state of the user, state of the Main message.
	Main string `bson:"main" json:"main" db:"main"`
	// MessageStates contains all states of the user for all messages. It is a map of message_id -> state.
	MessageStates map[int]string `bson:"message_states" json:"message_states" db:"message_states"`
	// MessagesAwaitingText is a unique stack that contains all messages IDs that awaits text.
	// Every message can produce text state and they should be handled as LIFO.
	MessagesAwaitingText []int `bson:"messages_awaiting_text" json:"messages_awaiting_text" db:"messages_awaiting_text"`

	// messagesStackInd is used to handle messages as a unique stack (swap in push)
	messagesStackInd map[int]int `bson:"-" json:"-" db:"-"`
}

// UserModelDiff contains changes that should be applied to user.
type UserModelDiff struct {
	Info         *UserInfoDiff     `bson:"info" json:"info" db:"info"`
	Messages     *UserMessagesDiff `bson:"messages" json:"messages" db:"messages"`
	State        *UserStateDiff    `bson:"state" json:"state" db:"state"`
	LastSeenTime *time.Time        `bson:"last_seen_time" json:"last_seen_time" db:"last_seen_time"`
	DisabledTime *time.Time        `bson:"disabled_time" json:"disabled_time" db:"disabled_time"`
	IsDisabled   *bool             `bson:"is_disabled" json:"is_disabled" db:"is_disabled"`
}

// UserInfoDiff contains changes that should be applied to user info.
type UserInfoDiff struct {
	FirstName    *string `bson:"first_name" json:"first_name" db:"first_name"`
	LastName     *string `bson:"last_name" json:"last_name" db:"last_name"`
	Username     *string `bson:"username" json:"username" db:"username"`
	LanguageCode *string `bson:"language_code" json:"language_code" db:"language_code"`
	IsBot        *bool   `bson:"is_bot" json:"is_bot" db:"is_bot"`
	IsPremium    *bool   `bson:"is_premium" json:"is_premium" db:"is_premium"`
}

// UserMessagesDiff contains changes that should be applied to user messages.
type UserMessagesDiff struct {
	MainID         *int              `bson:"main_id" json:"main_id" db:"main_id"`
	HeadID         *int              `bson:"head_id" json:"head_id" db:"head_id"`
	NotificationID *int              `bson:"notification_id" json:"notification_id" db:"notification_id"`
	ErrorID        *int              `bson:"error_id" json:"error_id" db:"error_id"`
	HistoryIDs     []int             `bson:"history_ids" json:"history_ids" db:"history_ids"`
	LastActions    map[int]time.Time `bson:"last_actions" json:"last_actions" db:"last_actions"`
}

// UserStateDiff contains changes that should be applied to user state.
type UserStateDiff struct {
	Main                 *string        `bson:"main" json:"main" db:"main"`
	MessageStates        map[int]string `bson:"message_states" json:"message_states" db:"message_states"`
	MessagesAwaitingText []int          `bson:"messages_awaiting_text" json:"messages_awaiting_text" db:"messages_awaiting_text"`
}

func (u *UserModel) prepareAfterDB() {
	if u.Messages.LastActions == nil {
		u.Messages.LastActions = make(map[int]time.Time)
	}
	if u.State.MessageStates == nil {
		u.State.MessageStates = make(map[int]string)
	}
	if u.State.messagesStackInd == nil {
		u.State.messagesStackInd = make(map[int]int)
	}
}

func (m UserMessages) HasMsgID(msgID int) bool {
	return m.MainID == msgID ||
		m.HeadID == msgID ||
		m.NotificationID == msgID ||
		m.ErrorID == msgID ||
		slices.Contains(m.HistoryIDs, msgID)
}

// userContextImpl implements User interface.
type userContextImpl struct {
	user UserModel
	db   UsersStorage

	btnName string
	payload string

	buttonMap   *abstract.SafeMap[string, InitBundle]
	isInitedMsg *abstract.SafeMap[int, bool]

	// Add mutex for protecting user state and message updates
	mu sync.Mutex
}

func (m *userManagerImpl) newUserContext(user UserModel) *userContextImpl {
	user.prepareAfterDB()
	return &userContextImpl{
		db:          m.db,
		user:        user,
		buttonMap:   abstract.NewSafeMap[string, InitBundle](),
		isInitedMsg: abstract.NewSafeMap[int, bool](),
	}
}

func (u *userContextImpl) ID() int64 {
	return u.user.ID
}

func (u *userContextImpl) Username() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Info.Username
}

func (u *userContextImpl) Language() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Info.LanguageCode
}

func (u *userContextImpl) Model() UserModel {
	u.mu.Lock()
	defer u.mu.Unlock()
	// Return a copy to avoid race conditions if the caller modifies the model
	return u.user
}

func (u *userContextImpl) Info() UserInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Info
}

func (u *userContextImpl) State(msgID int) (State, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	st, ok := u.user.State.MessageStates[msgID]
	return state(st), ok
}

func (u *userContextImpl) StateMain() State {
	u.mu.Lock()
	defer u.mu.Unlock()
	return state(u.user.State.Main)
}

func (u *userContextImpl) Messages() UserMessages {
	u.mu.Lock()
	defer u.mu.Unlock()
	// Return a copy to avoid race conditions
	messages := u.user.Messages

	// Deep copy the slices and maps to avoid race conditions
	if len(messages.HistoryIDs) > 0 {
		historyCopy := make([]int, len(messages.HistoryIDs))
		copy(historyCopy, messages.HistoryIDs)
		messages.HistoryIDs = historyCopy
	}

	if len(messages.LastActions) > 0 {
		lastActionsCopy := make(map[int]time.Time, len(messages.LastActions))
		maps.Copy(lastActionsCopy, messages.LastActions)
		messages.LastActions = lastActionsCopy
	}

	return messages
}

func (u *userContextImpl) LastSeenTime(msgID ...int) time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(msgID) == 0 {
		return u.user.LastSeenTime
	}
	return u.user.Messages.LastActions[lang.First(msgID)]
}

func (u *userContextImpl) IsDisabled() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.IsDisabled
}

func (u *userContextImpl) String() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return "[@" + u.user.Info.Username + "|" + strconv.Itoa(int(u.user.ID)) + "]"
}

func (u *userContextImpl) setState(newState State, msgIDRaw ...int) {
	if newState == nil {
		return
	}

	if newState.NotChanged() {
		return
	}

	u.mu.Lock()

	var (
		msgID = lang.Check(lang.First(msgIDRaw), u.user.Messages.MainID)
		upd   UserStateDiff
	)

	if msgID <= 0 {
		msgID = u.user.Messages.MainID // Fallback to main message ID
	}

	currentState, ok := u.user.State.MessageStates[msgID]
	if ok && newState != state(currentState) && state(currentState).IsText() {
		// If we got new state we should remove current pending text state
		u.removeTextMessageLocked(msgID)
		upd.MessagesAwaitingText = u.user.State.MessagesAwaitingText
	}

	if newState.IsText() {
		// If new state - text state, we shoudld add it
		u.pushTextMessageLocked(msgID)
		upd.MessagesAwaitingText = u.user.State.MessagesAwaitingText
	}

	if msgID == u.user.Messages.MainID {
		u.user.State.Main = newState.String()
		upd.Main = &u.user.State.Main
	}

	u.user.LastSeenTime = time.Now().UTC()
	u.user.State.MessageStates[msgID] = newState.String()
	u.user.Messages.LastActions[msgID] = u.user.LastSeenTime

	upd.MessageStates = u.user.State.MessageStates

	// Make a copy of the data for the database update
	userID := u.user.ID
	lastActions := make(map[int]time.Time, len(u.user.Messages.LastActions))
	maps.Copy(lastActions, u.user.Messages.LastActions)
	lastSeenTime := u.user.LastSeenTime

	// Release the lock before making DB calls to avoid holding it too long
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State:        &upd,
		Messages:     &UserMessagesDiff{LastActions: lastActions},
		LastSeenTime: &lastSeenTime,
	})
}

func (u *userContextImpl) prepareTextStates() {
	// This method is always called with the lock held
	if len(u.user.State.MessagesAwaitingText) != len(u.user.State.messagesStackInd) {
		u.user.State.messagesStackInd = make(map[int]int, len(u.user.State.MessagesAwaitingText))
		for i, v := range u.user.State.MessagesAwaitingText {
			u.user.State.messagesStackInd[v] = i
		}
	}
}

func (u *userContextImpl) hasTextMessages() bool {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.prepareTextStates()
	return len(u.user.State.MessagesAwaitingText) > 0
}

func (u *userContextImpl) lastTextMessage() int {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.user.State.MessagesAwaitingText) == 0 {
		return 0
	}
	u.prepareTextStates()

	return u.user.State.MessagesAwaitingText[len(u.user.State.MessagesAwaitingText)-1]
}

// pushTextMessageLocked assumes the lock is already held
func (u *userContextImpl) pushTextMessageLocked(msgID int) {
	u.prepareTextStates()

	if index, ok := u.user.State.messagesStackInd[msgID]; ok {
		last := len(u.user.State.MessagesAwaitingText) - 1
		if index == last {
			return // already pushed
		}
		u.user.State.messagesStackInd[u.user.State.MessagesAwaitingText[last]] = index
		u.user.State.messagesStackInd[msgID] = last

		// swap with last
		u.user.State.MessagesAwaitingText[index], u.user.State.MessagesAwaitingText[last] =
			u.user.State.MessagesAwaitingText[last], u.user.State.MessagesAwaitingText[index]
		return
	}

	// append new
	u.user.State.messagesStackInd[msgID] = len(u.user.State.MessagesAwaitingText)
	u.user.State.MessagesAwaitingText = append(u.user.State.MessagesAwaitingText, msgID)
}

// removeTextMessageLocked assumes the lock is already held
func (u *userContextImpl) removeTextMessageLocked(msgID int) {
	u.prepareTextStates()

	indexToRemove, ok := u.user.State.messagesStackInd[msgID]
	if !ok {
		return
	}
	delete(u.user.State.messagesStackInd, msgID)

	if msgID == u.user.State.MessagesAwaitingText[len(u.user.State.MessagesAwaitingText)-1] {
		u.user.State.MessagesAwaitingText = u.user.State.MessagesAwaitingText[:len(u.user.State.MessagesAwaitingText)-1]
		return
	}

	for item, ind := range u.user.State.messagesStackInd {
		if ind < indexToRemove {
			continue
		}
		if ind > indexToRemove {
			u.user.State.messagesStackInd[item] = ind - 1
		}
	}

	if indexToRemove >= len(u.user.State.MessagesAwaitingText)-1 {
		u.user.State.MessagesAwaitingText = u.user.State.MessagesAwaitingText[:len(u.user.State.MessagesAwaitingText)-1]
		return
	}

	if indexToRemove == 0 {
		u.user.State.MessagesAwaitingText = u.user.State.MessagesAwaitingText[1:]
		return
	}

	u.user.State.MessagesAwaitingText = slices.Delete(u.user.State.MessagesAwaitingText, indexToRemove, indexToRemove+1)
}

func (u *userContextImpl) setMessages(msgIDs ...int) {
	u.mu.Lock()

	msgs := make([]int, 4)
	copy(msgs, msgIDs)
	u.user.Messages.MainID = msgs[0]
	u.user.Messages.HeadID = msgs[1]
	u.user.Messages.NotificationID = msgs[2]
	u.user.Messages.ErrorID = msgs[3]

	var historyIDs []int
	if len(msgs) > 4 {
		historyIDs = make([]int, len(msgs)-4)
		copy(historyIDs, msgs[4:])
		u.user.Messages.HistoryIDs = historyIDs
	}

	// Capture values for the DB update
	userID := u.user.ID
	mainID := u.user.Messages.MainID
	headID := u.user.Messages.HeadID
	notificationID := u.user.Messages.NotificationID
	errorID := u.user.Messages.ErrorID

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID:         &mainID,
			HeadID:         &headID,
			NotificationID: &notificationID,
			ErrorID:        &errorID,
			HistoryIDs:     historyIDs,
		},
	})
}

func (u *userContextImpl) setErrorMessage(msgID int) {
	u.mu.Lock()
	u.user.Messages.ErrorID = msgID

	// Capture values for the DB update
	userID := u.user.ID
	errorID := msgID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			ErrorID: &errorID,
		},
	})
}

func (u *userContextImpl) setNotificationMessage(msgID int) {
	u.mu.Lock()
	u.user.Messages.NotificationID = msgID

	// Capture values for the DB update
	userID := u.user.ID
	notificationID := msgID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			NotificationID: &notificationID,
		},
	})
}

func (u *userContextImpl) forgetHistoryMessage(msgIDs ...int) (found bool) {
	u.mu.Lock()

	var (
		userID               = u.user.ID
		updatedHistoryIDs    []int
		updatedMessageStates map[int]string
		updatedLastActions   map[int]time.Time
		updatedAwaitingText  []int
	)

	for _, msgIDToDelete := range msgIDs {
		for i, historyID := range u.user.Messages.HistoryIDs {
			if historyID != msgIDToDelete {
				continue
			}
			if i < len(u.user.Messages.HistoryIDs) {
				u.user.Messages.HistoryIDs = slices.Delete(u.user.Messages.HistoryIDs, i, i+1)
			}
			delete(u.user.State.MessageStates, msgIDToDelete)
			delete(u.user.Messages.LastActions, msgIDToDelete)

			for j, textID := range u.user.State.MessagesAwaitingText {
				if textID == msgIDToDelete {
					u.user.State.MessagesAwaitingText = slices.Delete(u.user.State.MessagesAwaitingText, j, j+1)
				}
			}
			found = true
		}
	}

	if found {
		updatedHistoryIDs = make([]int, len(u.user.Messages.HistoryIDs))
		copy(updatedHistoryIDs, u.user.Messages.HistoryIDs)

		updatedMessageStates = make(map[int]string, len(u.user.State.MessageStates))
		maps.Copy(updatedMessageStates, u.user.State.MessageStates)

		updatedLastActions = make(map[int]time.Time, len(u.user.Messages.LastActions))
		maps.Copy(updatedLastActions, u.user.Messages.LastActions)

		updatedAwaitingText = make([]int, len(u.user.State.MessagesAwaitingText))
		copy(updatedAwaitingText, u.user.State.MessagesAwaitingText)
	}

	u.mu.Unlock()

	if found {
		u.db.UpdateAsync(userID, &UserModelDiff{
			Messages: &UserMessagesDiff{
				HistoryIDs:  updatedHistoryIDs,
				LastActions: updatedLastActions,
			},
			State: &UserStateDiff{
				MessageStates:        updatedMessageStates,
				MessagesAwaitingText: updatedAwaitingText,
			},
		})
	}

	return found
}

func (u *userContextImpl) update(user *tele.User) {
	// Sanitize and validate the input user
	sanitizedUser := sanitizeTelegramUser(user)
	if sanitizedUser == nil {
		return
	}

	// Create the new info outside the lock
	newInfo := newUserInfo(sanitizedUser)

	u.mu.Lock()

	// Compare and early return if no changes
	if newInfo == u.user.Info {
		u.mu.Unlock()
		return
	}

	// Update the info
	u.user.Info = newInfo

	// Capture values for DB update
	userID := u.user.ID
	userInfoDiff := &UserInfoDiff{
		FirstName:    &u.user.Info.FirstName,
		LastName:     &u.user.Info.LastName,
		Username:     &u.user.Info.Username,
		LanguageCode: &u.user.Info.LanguageCode,
		IsBot:        &u.user.Info.IsBot,
		IsPremium:    &u.user.Info.IsPremium,
	}

	u.mu.Unlock()

	// Update the database
	u.db.UpdateAsync(userID, &UserModelDiff{
		Info: userInfoDiff,
	})
}

func (u *userContextImpl) handleSend(newState State, mainMsgID, headMsgID int) {
	u.mu.Lock()

	currentTime := time.Now().UTC()
	u.user.LastSeenTime = currentTime
	u.user.Messages.LastActions[mainMsgID] = currentTime

	// Append to history IDs
	var historyIDs []int
	if u.user.Messages.MainID != 0 {
		historyIDs = make([]int, len(u.user.Messages.HistoryIDs)+1)
		copy(historyIDs, u.user.Messages.HistoryIDs)
		historyIDs[len(historyIDs)-1] = u.user.Messages.MainID
		u.user.Messages.HistoryIDs = historyIDs
	} else {
		historyIDs = make([]int, len(u.user.Messages.HistoryIDs))
		copy(historyIDs, u.user.Messages.HistoryIDs)
	}

	var stateMain *string
	var messageStates map[int]string

	if !newState.NotChanged() {
		u.user.State.Main = newState.String()
		u.user.State.MessageStates[mainMsgID] = newState.String()

		stateMain = &u.user.State.Main
		messageStates = make(map[int]string, len(u.user.State.MessageStates))
		maps.Copy(messageStates, u.user.State.MessageStates)
	}

	u.user.Messages.MainID = mainMsgID
	u.user.Messages.HeadID = headMsgID

	// Capture values for DB update
	userID := u.user.ID
	lastSeenTime := u.user.LastSeenTime
	mainID := mainMsgID
	headID := headMsgID

	lastActions := make(map[int]time.Time, len(u.user.Messages.LastActions))
	maps.Copy(lastActions, u.user.Messages.LastActions)

	u.mu.Unlock()

	// Update DB
	diff := &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID:      &mainID,
			HeadID:      &headID,
			HistoryIDs:  historyIDs,
			LastActions: lastActions,
		},
		LastSeenTime: &lastSeenTime,
	}

	if !newState.NotChanged() {
		diff.State = &UserStateDiff{
			Main:          stateMain,
			MessageStates: messageStates,
		}
	}

	u.db.UpdateAsync(userID, diff)
}

func (u *userContextImpl) disable() {
	u.mu.Lock()

	if u.user.IsDisabled {
		u.mu.Unlock()
		return
	}

	currentTime := time.Now().UTC()
	u.user.DisabledTime = currentTime
	u.user.IsDisabled = true
	u.user.State.Main = Disabled.String()
	u.user.State.MessageStates[u.user.Messages.MainID] = Disabled.String()

	// Capture values for DB update
	userID := u.user.ID
	disabledTime := u.user.DisabledTime
	isDisabled := u.user.IsDisabled
	stateMain := u.user.State.Main

	messageStates := make(map[int]string, len(u.user.State.MessageStates))
	maps.Copy(messageStates, u.user.State.MessageStates)

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State: &UserStateDiff{
			Main:          &stateMain,
			MessageStates: messageStates,
		},
		DisabledTime: &disabledTime,
		IsDisabled:   &isDisabled,
	})
}

func (u *userContextImpl) enable() {
	u.mu.Lock()

	if !u.user.IsDisabled {
		u.mu.Unlock()
		return
	}

	u.user.DisabledTime = time.Time{}
	u.user.IsDisabled = false
	u.user.State.Main = FirstRequest.String()
	u.user.State.MessageStates[u.user.Messages.MainID] = FirstRequest.String()

	// Capture values for DB update
	userID := u.user.ID
	disabledTime := u.user.DisabledTime
	isDisabled := u.user.IsDisabled
	stateMain := u.user.State.Main

	messageStates := make(map[int]string, len(u.user.State.MessageStates))
	maps.Copy(messageStates, u.user.State.MessageStates)

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State: &UserStateDiff{
			Main:          &stateMain,
			MessageStates: messageStates,
		},
		DisabledTime: &disabledTime,
		IsDisabled:   &isDisabled,
	})
}

func (u *userContextImpl) isMsgInited(msgID int) bool {
	if msgID == 0 || !u.user.Messages.HasMsgID(msgID) {
		return true
	}
	return u.isInitedMsg.Get(msgID)
}

func (u *userContextImpl) setMsgInited(msgID int) {
	if msgID == 0 || !u.user.Messages.HasMsgID(msgID) {
		return
	}
	u.isInitedMsg.Set(msgID, true)
}

func (u *userContextImpl) getBtnName() string {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.btnName
}

func (u *userContextImpl) setBtnName(btnName string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.btnName = btnName
}

func (u *userContextImpl) getPayload() string {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.payload
}

func (u *userContextImpl) setPayload(payload string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.payload = payload
}

func newUserModel(tUser *tele.User) UserModel {
	return UserModel{
		ID:   tUser.ID,
		Info: newUserInfo(tUser),
		State: UserState{
			Main:          FirstRequest.String(),
			MessageStates: make(map[int]string),
		},
		Messages: UserMessages{
			LastActions: make(map[int]time.Time),
		},
		LastSeenTime: time.Now().UTC(),
		CreatedTime:  time.Now().UTC(),
	}
}

func newUserInfo(tUser *tele.User) UserInfo {
	// Safety check
	if tUser == nil {
		return UserInfo{}
	}

	return UserInfo{
		FirstName:    sanitizeText(tUser.FirstName, 1000),
		LastName:     sanitizeText(tUser.LastName, 1000),
		Username:     sanitizeText(tUser.Username, 1000),
		LanguageCode: sanitizeText(tUser.LanguageCode, 1000),
		IsBot:        tUser.IsBot,
		IsPremium:    tUser.IsPremium,
	}
}

type userManagerImpl struct {
	users otter.Cache[int64, *userContextImpl]
	db    UsersStorage
	log   Logger
}

func newUserManager(db UsersStorage, log Logger, userCacheCapacity int, userCacheTTL time.Duration) (*userManagerImpl, error) {
	// Configure otter cache with proper eviction settings and TTL
	c, err := otter.MustBuilder[int64, *userContextImpl](userCacheCapacity).
		// Add cost function to better manage memory
		Cost(func(_ int64, value *userContextImpl) uint32 {
			// Cost is roughly based on the number of messages a user has
			// This helps prioritize eviction of users with more stored messages
			return uint32(1 + len(value.user.Messages.HistoryIDs))
		}).
		// Set TTL for inactive users to prevent memory leaks
		WithTTL(userCacheTTL).
		Build()
	if err != nil {
		return nil, errm.Wrapf(err, "failed to create user cache with capacity %d", userCacheCapacity)
	}

	m := &userManagerImpl{
		users: c,
		db:    db,
		log:   log,
	}

	return m, nil
}

func (m *userManagerImpl) prepareUser(ctx context.Context, tUser *tele.User) (*userContextImpl, error) {
	if tUser == nil {
		return nil, errm.New("cannot prepare user: telegram user is nil")
	}

	// Sanitize user input before processing
	sanitizedUser := sanitizeTelegramUser(tUser)

	user, found := m.users.Get(sanitizedUser.ID)
	if found {
		user.update(sanitizedUser)
		return user, nil
	}
	return m.createUser(ctx, sanitizedUser)
}

func (m *userManagerImpl) getUser(userID int64) *userContextImpl {
	if userID == 0 {
		m.log.Error("invalid user ID", "user_id", userID, "error", "userID cannot be zero")
		tUser := &tele.User{ID: 1} // Fallback to a safe default
		user := m.newUserContext(newUserModel(tUser))
		return user
	}

	user, found := m.users.Get(userID)
	if found {
		return user
	}

	m.log.Warn("user not found in cache", "user_id", userID, "action", "attempting to create new user")

	tUser := &tele.User{ID: userID}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := m.createUser(ctx, tUser)
	if err != nil {
		m.log.Error("cannot create user after cache miss",
			"user_id", userID,
			"error", err,
			"error_type", fmt.Sprintf("%T", err))

		// Create an emergency fallback user
		m.log.Info("creating fallback user object", "user_id", userID)
		user = m.newUserContext(newUserModel(tUser))
	}

	return user
}

func (m *userManagerImpl) getAllUsers() []User {
	out := make([]User, 0, m.users.Size())
	m.users.Range(func(_ int64, value *userContextImpl) bool {
		out = append(out, value)
		return true
	})
	return out
}

func (m *userManagerImpl) createUser(ctx context.Context, tUser *tele.User) (*userContextImpl, error) {
	if ctx == nil {
		return nil, errm.New("cannot create user: context is nil")
	}

	if tUser == nil {
		return nil, errm.New("cannot create user: telegram user is nil")
	}

	userModel, isFound, err := m.db.Find(ctx, tUser.ID)
	if err != nil {
		return nil, errm.Wrapf(err, "failed to find user %d in database", tUser.ID)
	}

	if !isFound {
		userModel = newUserModel(tUser)
		if err := m.db.Insert(ctx, userModel); err != nil {
			return nil, errm.Wrapf(err, "failed to insert new user %d into database", tUser.ID)
		}
	}

	user := m.newUserContext(userModel)
	if ok := m.users.Set(user.ID(), user); !ok {
		m.log.Warn("failed to add user to cache", "user_id", user.ID(), "username", user.Username())
	}

	// Disabled user -> user blocked bot
	// If user gets here -> he makes request -> he unblock bot
	if userModel.IsDisabled {
		m.log.Info("enabling previously disabled user", "user_id", user.ID(), "username", user.Username())
		user.enable()
	} else {
		m.log.Info("new user created", "user_id", user.ID(), "username", user.Username())
	}

	return user, nil
}

func (m *userManagerImpl) disableUser(userID int64) {
	u, ok := m.users.Get(userID)
	if !ok {
		return
	}
	u.disable()
	m.users.Delete(userID)
}

func (m *userManagerImpl) deleteUser(userID int64) {
	m.users.Delete(userID)
}

type inMemoryUserStorage struct {
	cache otter.Cache[int64, UserModel]
}

func newInMemoryUserStorage(userCacheCapacity int, userCacheTTL time.Duration) (UsersStorage, error) {
	// Configure in-memory storage with proper eviction settings
	s, err := otter.MustBuilder[int64, UserModel](userCacheCapacity).
		// Add cost function to better manage memory
		Cost(func(_ int64, value UserModel) uint32 {
			// Cost is roughly based on the number of messages a user has
			return uint32(1 + len(value.Messages.HistoryIDs))
		}).
		// Set TTL for inactive users to prevent memory leaks
		WithTTL(userCacheTTL).
		Build()
	if err != nil {
		return nil, errm.Wrapf(err, "failed to create in-memory storage with capacity %d", userCacheCapacity)
	}
	return &inMemoryUserStorage{
		cache: s,
	}, nil
}

func (m *inMemoryUserStorage) Insert(ctx context.Context, user UserModel) error {
	if ctx == nil {
		return errm.New("cannot insert user: context is nil")
	}

	if user.ID == 0 {
		return errm.New("cannot insert user: invalid user ID (zero)")
	}

	if !m.cache.Set(user.ID, user) {
		return errm.Wrap(errm.New("cache rejected insertion"), fmt.Sprintf("failed to insert user %d into in-memory storage", user.ID))
	}
	return nil
}

func (m *inMemoryUserStorage) Find(ctx context.Context, id int64) (UserModel, bool, error) {
	if ctx == nil {
		return UserModel{}, false, errm.New("cannot find user: context is nil")
	}

	if id == 0 {
		return UserModel{}, false, errm.New("cannot find user: invalid user ID (zero)")
	}

	user, found := m.cache.Get(id)
	if !found {
		return UserModel{}, false, nil
	}
	return user, true, nil
}

func (m *inMemoryUserStorage) FindAll(context.Context) ([]UserModel, error) {
	out := make([]UserModel, 0, m.cache.Size())
	m.cache.Range(func(_ int64, value UserModel) bool {
		out = append(out, value)
		return true
	})
	return out, nil
}

func (m *inMemoryUserStorage) UpdateAsync(id int64, diff *UserModelDiff) {
	user, found := m.cache.Get(id)
	if !found {
		return
	}

	if diff.Info != nil {
		user.Info = UserInfo{
			FirstName:    lang.Check(lang.Deref(diff.Info.FirstName), user.Info.FirstName),
			LastName:     lang.Check(lang.Deref(diff.Info.LastName), user.Info.LastName),
			Username:     lang.Check(lang.Deref(diff.Info.Username), user.Info.Username),
			LanguageCode: lang.Check(lang.Deref(diff.Info.LanguageCode), user.Info.LanguageCode),
			IsBot:        lang.Check(lang.Deref(diff.Info.IsBot), user.Info.IsBot),
			IsPremium:    lang.Check(lang.Deref(diff.Info.IsPremium), user.Info.IsPremium),
		}
	}
	if diff.Messages != nil {
		user.Messages = UserMessages{
			MainID:         lang.Check(lang.Deref(diff.Messages.MainID), user.Messages.MainID),
			HeadID:         lang.Check(lang.Deref(diff.Messages.HeadID), user.Messages.HeadID),
			NotificationID: lang.Check(lang.Deref(diff.Messages.NotificationID), user.Messages.NotificationID),
			ErrorID:        lang.Check(lang.Deref(diff.Messages.ErrorID), user.Messages.ErrorID),
			HistoryIDs:     lang.If(len(diff.Messages.HistoryIDs) > 0, diff.Messages.HistoryIDs, user.Messages.HistoryIDs),
			LastActions:    lang.If(len(diff.Messages.LastActions) > 0, diff.Messages.LastActions, user.Messages.LastActions),
		}
	}
	if diff.State != nil {
		user.State = UserState{
			Main:                 lang.Check(lang.Deref(diff.State.Main), user.State.Main),
			MessageStates:        lang.If(len(diff.State.MessageStates) > 0, diff.State.MessageStates, user.State.MessageStates),
			MessagesAwaitingText: lang.If(diff.State.MessagesAwaitingText != nil, diff.State.MessagesAwaitingText, user.State.MessagesAwaitingText),
		}
	}

	user.LastSeenTime = lang.CheckTime(lang.Deref(diff.LastSeenTime), user.LastSeenTime)
	user.DisabledTime = lang.CheckTime(lang.Deref(diff.DisabledTime), user.DisabledTime)
	user.IsDisabled = lang.Check(lang.Deref(diff.IsDisabled), user.IsDisabled)

	m.cache.Set(id, user)
}

type state string

const (
	// NoChange is a state that means that user state should not be changed after Send of Edit.
	NoChange state = ""
	// FirstRequest is a state of a user after first request to bot.
	FirstRequest state = "first_request"
	// Disabled is a state of a disabled user.
	Disabled state = "disabled"
)

func (s state) String() string {
	return string(s)
}

func (state) IsText() bool {
	return false
}

func (s state) NotChanged() bool {
	return s == NoChange
}

// sanitizeTelegramUser sanitizes fields in a Telegram user object
func sanitizeTelegramUser(user *tele.User) *tele.User {
	if user == nil {
		return nil
	}

	// Create a copy to avoid modifying the original
	sanitized := *user

	// Sanitize text fields
	sanitized.FirstName = sanitizeText(user.FirstName, 1000)
	sanitized.LastName = sanitizeText(user.LastName, 1000)
	sanitized.Username = sanitizeText(user.Username, 1000)
	sanitized.LanguageCode = sanitizeText(user.LanguageCode, 1000)

	return &sanitized
}
