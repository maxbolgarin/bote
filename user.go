package bote

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	"github.com/maypok86/otter"
	tele "gopkg.in/telebot.v4"
)

// State is a user state in Telegram bot builded using this package.
// User changes states when he makes actions, e.g. sends a message, clicks a button, etc.
// State is connected to message, every Main and Info message has its own state.
// State is necessary for understanding user behavior and should be used in user init after bot restarting.
// You should create states as constants in your application and pass it in Send or Edit methods as first argument.
// States is generally changes in response to user actions inside handlers,
// but also can be changed in other places in case of any action.
type State interface {
	String() string
	IsChanged() bool
}

// User is an interface that represents user context in the bot.
type User interface {
	// ID is Telegram user ID.
	ID() int64
	// Info returns user info.
	Info() UserInfo
	// Username returns Telegram username (without @).
	Username() string
	// Language returns Telegram user language code.
	Language() string
	// State returns current state for the given message ID.
	// If message ID is not provided, it returns current state for the Main message.
	State(msgID ...int) State
	// SetState sets the given state for the given message ID.
	// If message ID is not provided, it sets the given state for the Main message.
	SetState(state State, msgID ...int)
	// HasTextStates returns true if there is text states in stack.
	HasTextStates() bool
	// LastTextState returns last text state from stack without poping.
	LastTextState() (State, int)
	// PopTextState pops last text state from stack.
	PopTextState() (State, int)
	// PushTextState pushes text state to the end of stack.
	PushTextState(state State, msgID int)
	// Messages returns all message IDs.
	Messages() UserMessages
	// SetMessages sets the given message IDs.
	SetMessages(msgIDs ...int)
	// SetMainMessage sets the given message ID as main message ID.
	SetMainMessage(msgID int)
	// SetHeadMessage sets the given message ID as head message ID.
	SetHeadMessage(msgID int)
	// SetErrorMessage sets the given message ID as error message ID.
	SetErrorMessage(msgID int)
	// SetNotificationMessage sets the given message ID as notification message ID.
	SetNotificationMessage(msgID int)
	// AddHistoryMessage adds the given message ID to history message IDs.
	AddHistoryMessage(msgID int)
	// Register set user as registered.
	Register()
	// IsRegistered returns true if user is registered.
	IsRegistered() bool
	// Update updates user info with the given Telegram user.
	Update(user *tele.User)
	// Disable set user as disabled.
	Disable()
	// IsDisabled returns true if user is disabled.
	IsDisabled() bool
	// HandleSend makes three user updates that usually happen after Send new message in one request:
	//SetState, SetMessages, AddHistoryMessage.
	HandleSend(s State, mainMsgID, headMsgID int)
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
	// FindAll returns all users from storage. It retutns empty slice if there are no users without error.
	FindAll(ctx context.Context) ([]UserModel, error)

	// Update updates user model in storage. The idea of that method is that it calls on every user action
	// (e.g. to update state), so it should be async to make it faster or user (without lags).
	// So this method doesn't accept context and doesn't return error because it should be called in async goroutine.
	//
	// Warning! You can't just simple use workers pool, because updates should be ordered. If you don't want to
	// make it async, you may use sync operation in this method and handle error using channels, for example.
	// You may be intersting in https://github.com/maxbolgarin/mongox for async operations in MongoDB or
	// https://github.com/maxbolgarin/gorder for general async queue if you use another db.
	Update(id int64, userModel *UserModelDiff)
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
	// LastSeen is the last time user interacted with the bot.
	LastSeen time.Time `bson:"last_seen" json:"last_seen" db:"last_seen"`
	// Created is the time when user was created.
	Created time.Time `bson:"created" json:"created" db:"created"`
	// Registered is the time when user was registered.
	Registered time.Time `bson:"registered" json:"registered" db:"registered"`
	// Disabled is the time when user was disabled.
	Disabled time.Time `bson:"disabled" json:"disabled" db:"disabled"`
	// IsDisabled returns true if user is disabled.
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
	// Notification message can be sent in any time and deleted after some time.
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
	Main State `bson:"main" json:"main" db:"main"`
	// MessageStates contains all states of the user for all messages. It is a map of message_id -> state.
	MessageStates map[int]State `bson:"message_states" json:"message_states" db:"message_states"`
	// TextStates contains all text states of the user.
	// Every message can produce text state and they should be handled as LIFO.
	TextStates []StateWithMessage `bson:"text_states" json:"text_states" db:"text_states"`

	// ind is used to handle text states as a unique stack (swap in push)
	textStatesInd map[StateWithMessage]int `bson:"-" json:"-" db:"-"`
}

// StateWithMessage contains text state and message ID.
// It is used for storing text states in stack.
type StateWithMessage string

// NewStateWithMessage returns new state with message.
func NewStateWithMessage(msgID int, state State) StateWithMessage {
	return StateWithMessage(state.String() + "_" + strconv.Itoa(msgID))
}

// MessageID returns message ID from state with message.
func (t StateWithMessage) MessageID() int {
	out, _ := strconv.Atoi(strings.Split(string(t), "_")[1])
	return out
}

// State returns state from state with message.
func (t StateWithMessage) State() State {
	return state(strings.Split(string(t), "_")[0])
}

// UserModelDiff contains changes that should be applied to user.
type UserModelDiff struct {
	Info       *UserInfoDiff     `bson:"info" json:"info" db:"info"`
	Messages   *UserMessagesDiff `bson:"messages" json:"messages" db:"messages"`
	State      *UserStateDiff    `bson:"state" json:"state" db:"state"`
	LastSeen   *time.Time        `bson:"last_seen" json:"last_seen" db:"last_seen"`
	Registered *time.Time        `bson:"registered" json:"registered" db:"registered"`
	Disabled   *time.Time        `bson:"disabled" json:"disabled" db:"disabled"`
	IsDisabled *bool             `bson:"is_disabled" json:"is_disabled" db:"is_disabled"`
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
	Main          *State             `bson:"main" json:"main" db:"main"`
	MessageStates map[int]State      `bson:"message_states" json:"message_states" db:"message_states"`
	TextStates    []StateWithMessage `bson:"text_states" json:"text_states" db:"text_states"`
}

// userContextImpl implements User interface.
type userContextImpl struct {
	user UserModel
	db   UsersStorage
}

func (u userContextImpl) String() string {
	return "[@" + u.user.Info.Username + "|" + strconv.Itoa(int(u.user.ID)) + "]"
}

func (m *userManagerImpl) newUserContext(user UserModel) User {
	return &userContextImpl{db: m.db, user: user}
}

func (u *userContextImpl) ID() int64 {
	return u.user.ID
}

func (u *userContextImpl) Info() UserInfo {
	return u.user.Info
}

func (u *userContextImpl) Username() string {
	return u.user.Info.Username
}

func (u *userContextImpl) Language() string {
	return u.user.Info.LanguageCode
}

func (u *userContextImpl) State(msgID ...int) State {
	if len(msgID) == 0 || msgID[0] == u.user.Messages.MainID {
		return u.user.State.Main
	}
	return u.user.State.MessageStates[msgID[0]]
}

func (u *userContextImpl) SetState(state State, msgIDRaw ...int) {
	if !state.IsChanged() {
		return
	}

	u.user.LastSeen = time.Now().UTC()

	var (
		msgID = lang.Check(lang.First(msgIDRaw), u.user.Messages.MainID)
		upd   UserStateDiff
	)

	if msgID == u.user.Messages.MainID {
		u.user.State.Main = state
		upd.Main = &state
	}

	u.user.State.MessageStates[msgID] = state
	u.user.Messages.LastActions[msgID] = u.user.LastSeen

	upd.MessageStates = u.user.State.MessageStates

	u.db.Update(u.user.ID, &UserModelDiff{
		State:    &upd,
		Messages: &UserMessagesDiff{LastActions: u.user.Messages.LastActions},
		LastSeen: &u.user.LastSeen,
	})
}

func (u *userContextImpl) prepareTextStates() {
	if len(u.user.State.TextStates) != len(u.user.State.textStatesInd) {
		u.user.State.textStatesInd = make(map[StateWithMessage]int, len(u.user.State.TextStates))
		for i, v := range u.user.State.TextStates {
			u.user.State.textStatesInd[v] = i
		}
	}
}

func (u *userContextImpl) HasTextStates() bool {
	u.prepareTextStates()
	return len(u.user.State.TextStates) > 0
}

func (u *userContextImpl) LastTextState() (State, int) {
	if len(u.user.State.TextStates) == 0 {
		return NoChange, 0
	}
	u.prepareTextStates()

	ts := u.user.State.TextStates[len(u.user.State.TextStates)-1]
	return ts.State(), ts.MessageID()
}

func (u *userContextImpl) PopTextState() (State, int) {
	if len(u.user.State.TextStates) == 0 {
		return NoChange, 0
	}
	u.prepareTextStates()

	index := len(u.user.State.TextStates) - 1

	item := u.user.State.TextStates[index]
	u.user.State.TextStates = u.user.State.TextStates[:index]
	delete(u.user.State.textStatesInd, item)

	u.db.Update(u.user.ID, &UserModelDiff{
		State: &UserStateDiff{
			TextStates: u.user.State.TextStates,
		},
	})

	return item.State(), item.MessageID()
}

func (u *userContextImpl) PushTextState(state State, msgID int) {
	item := NewStateWithMessage(msgID, state)
	if index, ok := u.user.State.textStatesInd[item]; ok {
		last := len(u.user.State.TextStates) - 1
		if index == last {
			return // already pushed
		}
		u.user.State.textStatesInd[u.user.State.TextStates[last]] = index
		u.user.State.textStatesInd[item] = last

		// swap with last
		u.user.State.TextStates[index], u.user.State.TextStates[last] = u.user.State.TextStates[last], u.user.State.TextStates[index]
		return
	}

	// append new
	u.user.State.textStatesInd[item] = len(u.user.State.TextStates)
	u.user.State.TextStates = append(u.user.State.TextStates, item)

	u.db.Update(u.user.ID, &UserModelDiff{
		State: &UserStateDiff{
			TextStates: u.user.State.TextStates,
		},
	})
}

func (u *userContextImpl) Messages() UserMessages {
	return u.user.Messages
}

func (u *userContextImpl) SetMessages(msgIDs ...int) {
	msgs := make([]int, 4)
	copy(msgs, msgIDs)
	u.user.Messages.MainID = msgs[0]
	u.user.Messages.HeadID = msgs[1]
	u.user.Messages.NotificationID = msgs[2]
	u.user.Messages.ErrorID = msgs[3]

	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID:         &u.user.Messages.MainID,
			HeadID:         &u.user.Messages.HeadID,
			NotificationID: &u.user.Messages.NotificationID,
			ErrorID:        &u.user.Messages.ErrorID,
		},
	})
}

func (u *userContextImpl) SetMainMessage(msgID int) {
	u.user.Messages.MainID = msgID
	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID: &u.user.Messages.MainID,
		},
	})
}

func (u *userContextImpl) SetHeadMessage(msgID int) {
	u.user.Messages.HeadID = msgID
	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			HeadID: &u.user.Messages.HeadID,
		},
	})
}

func (u *userContextImpl) SetErrorMessage(msgID int) {
	u.user.Messages.ErrorID = msgID
	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			ErrorID: &u.user.Messages.ErrorID,
		},
	})
}

func (u *userContextImpl) SetNotificationMessage(msgID int) {
	u.user.Messages.NotificationID = msgID
	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			NotificationID: &u.user.Messages.NotificationID,
		},
	})
}

func (u *userContextImpl) AddHistoryMessage(msgID int) {
	u.user.Messages.HistoryIDs = append(u.user.Messages.HistoryIDs, msgID)
	u.db.Update(u.user.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			HistoryIDs: u.user.Messages.HistoryIDs,
		},
	})
}

func (u *userContextImpl) Register() {
	u.user.Registered = time.Now().UTC()
	u.db.Update(u.user.ID, &UserModelDiff{
		Registered: &u.user.Registered,
	})
}

func (u *userContextImpl) IsRegistered() bool {
	return !u.user.Registered.IsZero()
}

func (u *userContextImpl) Update(user *tele.User) {
	newInfo := newUserInfo(user)
	if newInfo == u.user.Info {
		return
	}
	u.user.Info = newInfo
	u.db.Update(u.user.ID, &UserModelDiff{
		Info: &UserInfoDiff{
			FirstName:    &u.user.Info.FirstName,
			LastName:     &u.user.Info.LastName,
			Username:     &u.user.Info.Username,
			LanguageCode: &u.user.Info.LanguageCode,
			IsBot:        &u.user.Info.IsBot,
			IsPremium:    &u.user.Info.IsPremium,
		},
	},
	)
}

func (u *userContextImpl) Disable() {
	if u.user.IsDisabled {
		return
	}

	u.user.Disabled = time.Now().UTC()
	u.user.IsDisabled = true
	u.user.State.Main = Disabled

	u.db.Update(u.user.ID, &UserModelDiff{
		State: &UserStateDiff{
			Main: &u.user.State.Main,
		},
		Disabled:   &u.user.Disabled,
		IsDisabled: &u.user.IsDisabled,
	})
}

func (u *userContextImpl) IsDisabled() bool {
	return u.user.IsDisabled
}

func (u *userContextImpl) HandleSend(newState State, mainMsgID, headMsgID int) {
	u.user.LastSeen = time.Now().UTC()
	u.user.Messages.LastActions[mainMsgID] = u.user.LastSeen

	u.user.Messages.HistoryIDs = append(u.user.Messages.HistoryIDs, u.user.Messages.MainID)

	if newState.IsChanged() {
		u.user.State.Main = newState
		u.user.State.MessageStates[mainMsgID] = newState
	}

	u.user.Messages.MainID = mainMsgID
	u.user.Messages.HeadID = headMsgID

	u.db.Update(u.user.ID, &UserModelDiff{
		State: &UserStateDiff{
			Main:          &u.user.State.Main,
			MessageStates: u.user.State.MessageStates,
		},
		Messages: &UserMessagesDiff{
			MainID:      &u.user.Messages.MainID,
			HeadID:      &u.user.Messages.HeadID,
			HistoryIDs:  u.user.Messages.HistoryIDs,
			LastActions: u.user.Messages.LastActions,
		},
		LastSeen: &u.user.LastSeen,
	})
}

func newUserModel(tUser *tele.User) UserModel {
	return UserModel{
		ID:   tUser.ID,
		Info: newUserInfo(tUser),
		State: UserState{
			Main:          FirstRequest,
			MessageStates: make(map[int]State),
		},
		Messages: UserMessages{
			LastActions: make(map[int]time.Time),
		},
		LastSeen: time.Now().UTC(),
		Created:  time.Now().UTC(),
	}
}

func newUserInfo(tUser *tele.User) UserInfo {
	return UserInfo{
		FirstName:    tUser.FirstName,
		LastName:     tUser.LastName,
		Username:     tUser.Username,
		LanguageCode: tUser.LanguageCode,
		IsBot:        tUser.IsBot,
		IsPremium:    tUser.IsPremium,
	}
}

const (
	userCacheCapacity = 1000
)

type userManagerImpl struct {
	users otter.Cache[int64, User]
	db    UsersStorage
	log   Logger
}

func newUserManager(ctx context.Context, db UsersStorage, log Logger) (*userManagerImpl, error) {
	c, err := otter.MustBuilder[int64, User](userCacheCapacity).Build()
	if err != nil {
		return nil, err
	}

	m := &userManagerImpl{
		users: c,
		db:    db,
		log:   log,
	}

	err = m.initAllUsersFromDB(ctx)
	if err != nil {
		return nil, errm.Wrap(err, "init all users")
	}

	return m, nil
}

func (m *userManagerImpl) prepareUser(ctx context.Context, tUser *tele.User) (User, error) {
	user, found := m.users.Get(tUser.ID)
	if found {
		user.Update(tUser)
		return user, nil
	}
	return m.createUser(ctx, tUser)
}

func (m *userManagerImpl) getUser(userID int64) User {
	user, found := m.users.Get(userID)
	if found {
		return user
	}

	m.log.Warn("bug: not found in cache", "user_id", userID)

	tUser := &tele.User{ID: userID}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := m.createUser(ctx, tUser)
	if err != nil {
		m.log.Error("cannot create user after cache miss", "user_id", userID, "error", err)
		user = m.newUserContext(newUserModel(tUser))
	}

	return user
}

func (m *userManagerImpl) getAllUsers() []User {
	out := make([]User, 0, m.users.Size())
	m.users.Range(func(key int64, value User) bool {
		out = append(out, value)
		return true
	})
	return out
}

func (m *userManagerImpl) createUser(ctx context.Context, tUser *tele.User) (User, error) {
	userModel, isFound, err := m.db.Find(ctx, tUser.ID)
	if err != nil {
		return nil, errm.Wrap(err, "get")
	}
	if !isFound {
		userModel = newUserModel(tUser)
		if err := m.db.Insert(ctx, userModel); err != nil {
			return nil, errm.Wrap(err, "insert user")
		}
	}

	user := m.newUserContext(userModel)
	m.users.Set(user.ID(), user)

	m.log.Info("new user", "user_id", user.ID(), "username", user.Username())

	return user, nil
}

func (m *userManagerImpl) initAllUsersFromDB(ctx context.Context) error {
	users, err := m.db.FindAll(ctx)
	switch {
	case err == nil && len(users) == 0:
		m.log.Info("no users in DB")
		return nil

	case err != nil:
		return errm.Wrap(err, "find all")
	}

	for _, u := range users {
		if u.IsDisabled {
			continue
		}
		m.users.Set(u.ID, m.newUserContext(u))
	}

	m.log.Info("init users", "count", m.users.Size())

	return nil
}

func (m *userManagerImpl) removeUserFromMemory(userID int64) {
	m.users.Delete(userID)
}

type inMemoryUserStorage struct {
	cache otter.Cache[int64, UserModel]
}

func newInMemoryUserStorage() (UsersStorage, error) {
	s, err := otter.MustBuilder[int64, UserModel](100).Build()
	if err != nil {
		return nil, err
	}
	return &inMemoryUserStorage{
		cache: s,
	}, nil
}

func (m *inMemoryUserStorage) Insert(ctx context.Context, user UserModel) error {
	m.cache.Set(user.ID, user)
	return nil
}

func (m *inMemoryUserStorage) Find(ctx context.Context, id int64) (UserModel, bool, error) {
	user, found := m.cache.Get(id)
	if !found {
		return UserModel{}, false, nil
	}
	return user, true, nil
}

func (m *inMemoryUserStorage) FindAll(ctx context.Context) ([]UserModel, error) {
	out := make([]UserModel, 0, m.cache.Size())
	m.cache.Range(func(key int64, value UserModel) bool {
		out = append(out, value)
		return true
	})
	return out, nil
}

func (m *inMemoryUserStorage) Update(id int64, diff *UserModelDiff) {
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
			Main:          lang.Check(lang.Deref(diff.State.Main), user.State.Main),
			MessageStates: lang.If(len(diff.State.MessageStates) > 0, diff.State.MessageStates, user.State.MessageStates),
			TextStates:    lang.If(diff.State.TextStates != nil, diff.State.TextStates, user.State.TextStates),
		}
	}

	user.LastSeen = lang.CheckTime(lang.Deref(diff.LastSeen), user.LastSeen)
	user.Registered = lang.CheckTime(lang.Deref(diff.Registered), user.Registered)
	user.Disabled = lang.CheckTime(lang.Deref(diff.Disabled), user.Disabled)
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

func (s state) IsChanged() bool {
	return len(s) > 0
}

func (s state) String() string {
	return string(s)
}
