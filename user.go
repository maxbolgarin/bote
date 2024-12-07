package bote

import (
	"context"
	"strconv"
	"time"

	"github.com/maxbolgarin/abstract"
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
type State string

const (
	// Unknown is empty state of User that means error.
	Unknown State = ""
	// NoChange is a state that means that user state should not be changed after Send of Edit.
	NoChange State = Unknown
	// FirstRequest is a state of a user after first request to bot.
	FirstRequest State = "first_request"
	// Disabled is a state of a disabled user.
	Disabled State = "disabled"
)

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
	// RemoveTextState removes text state from any position in stack.
	RemoveTextState(state State, msgID int)
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
}

// UserStorage is a storage for users.
// You should implement it in your application if you want to persist users between applicataion restarts.
type UserStorage interface {
	// Insert inserts user in storage.
	Insert(ctx context.Context, userModel UserModel) error
	// Get returns user from storage. It returns true as a second argument if user was found without error.
	Get(ctx context.Context, id int64) (UserModel, bool, error)
	// GetAll returns all users from storage. It retutns empty slice if there are no users without error.
	GetAll(ctx context.Context) ([]UserModel, error)

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

// UserModel is a structure that represents user in DB.
type UserModel struct {
	// Info contains user info, that can be obtained from Telegram.
	Info UserInfo `bson:"info"`
	// Messages contains IDs of user messages.
	Messages UserMessages `bson:"messages"`
	// State contains state for every user's message.
	State UserState `bson:"state"`
	// LastSeen is the last time user interacted with the bot.
	LastSeen time.Time `bson:"last_seen"`
	// Created is the time when user was created.
	Created time.Time `bson:"created"`
	// Registered is the time when user was registered.
	Registered time.Time `bson:"registered"`
	// Disabled is the time when user was disabled.
	Disabled time.Time `bson:"disabled"`
	// IsDisabled returns true if user is disabled.
	IsDisabled bool `bson:"is_disabled"`
}

// UserInfo contains user info, that can be obtained from Telegram.
type UserInfo struct {
	// ID is Telegram user ID.
	ID int64 `bson:"id"`
	// FirstName is Telegram user first name.
	FirstName string `bson:"first_name"`
	// LastName is Telegram user last name.
	LastName string `bson:"last_name"`
	// Username is Telegram username (without @).
	Username string `bson:"username"`
	// LanguageCode is Telegram user language code.
	LanguageCode string `bson:"language_code"`
	// IsBot is true if Telegram user is a bot.
	IsBot bool `bson:"is_bot"`
	// IsPremium is true if Telegram user has Telegram Premium.
	IsPremium bool `bson:"is_premium"`
}

// UserMessages contains IDs of user messages.
type UserMessages struct {
	// Main message is the last and primary one in the chat.
	MainID int `bson:"main_id"`
	// Head message is sent right before main message for making bot more interactive.
	HeadID int `bson:"head_id"`
	// Notification message can be sent in any time and deleted after some time.
	NotificationID int `bson:"notification_id"`
	// Error message can be sent in any time in case of error and deleted automically after next action.
	ErrorID int `bson:"error_id"`
	// History message is the previous main messages. Main message becomes History message after new Main sending.
	HistoryIDs []int `bson:"history_ids"`
	// LastActions contains time of last interaction of user with every message.
	LastActions map[int]time.Time `bson:"last_actions"`
}

// UserState contains current user state and state history.
// State connects to message, every Main and Info message has its own state.
type UserState struct {
	// Main is the main state of the user, state of the Main message.
	Main State `bson:"main"`
	// MessageStates contains all states of the user for all messages. It is a map of message_id -> state.
	MessageStates map[int]State `bson:"message_states"`
	// TextStates contains all text states of the user.
	// Every message can produce text state and they should be handled as LIFO.
	TextStates *abstract.UniqueStack[TextStateWithMessage] `bson:"text_states"`
}

// TextStateWithMessage contains text state and message ID.
// It is used for storing text states in stack.
type TextStateWithMessage struct {
	// MessageID is the ID of the message that produced this text state.
	MessageID int `bson:"message_id"`
	// State is the text state.
	State State `bson:"state"`
}

// UserModelDiff contains changes that should be applied to user.
type UserModelDiff struct {
	Info       *UserInfoDiff     `bson:"info"`
	Messages   *UserMessagesDiff `bson:"messages"`
	State      *UserStateDiff    `bson:"state"`
	LastSeen   *time.Time        `bson:"last_seen"`
	Registered *time.Time        `bson:"registered"`
	Disabled   *time.Time        `bson:"disabled"`
	IsDisabled *bool             `bson:"is_disabled"`
}

// UserInfoDiff contains changes that should be applied to user info.
type UserInfoDiff struct {
	FirstName    *string `bson:"first_name"`
	LastName     *string `bson:"last_name"`
	Username     *string `bson:"username"`
	LanguageCode *string `bson:"language_code"`
	IsBot        *bool   `bson:"is_bot"`
	IsPremium    *bool   `bson:"is_premium"`
}

// UserMessagesDiff contains changes that should be applied to user messages.
type UserMessagesDiff struct {
	MainID         *int              `bson:"main_id"`
	HeadID         *int              `bson:"head_id"`
	NotificationID *int              `bson:"notification_id"`
	ErrorID        *int              `bson:"error_id"`
	HistoryIDs     []int             `bson:"history_ids"`
	LastActions    map[int]time.Time `bson:"last_actions"`
}

// UserStateDiff contains changes that should be applied to user state.
type UserStateDiff struct {
	Main          *State                                      `bson:"main"`
	MessageStates map[int]State                               `bson:"message_states"`
	TextStates    *abstract.UniqueStack[TextStateWithMessage] `bson:"text_states"`
}

// userContextImpl implements User interface.
type userContextImpl struct {
	user UserModel
	db   UserStorage
}

// IDString returns user ID as string.
func (u UserInfo) IDString() string {
	return strconv.FormatInt(u.ID, 10)
}

// Unknown returns true if state is Unknown.
func (s State) Unknown() bool {
	return s == Unknown
}

// IsChanged returns true if state is not empty and should be changed.
func (s State) IsChanged() bool {
	return s != NoChange
}

func (m *userManagerImpl) newUserContext(user UserModel) User {
	return &userContextImpl{db: m.db, user: user}
}

func (u *userContextImpl) ID() int64 {
	return u.user.Info.ID
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

	u.db.Update(u.user.Info.ID, &UserModelDiff{
		State:    &upd,
		Messages: &UserMessagesDiff{LastActions: u.user.Messages.LastActions},
		LastSeen: &u.user.LastSeen,
	})
}

func (u *userContextImpl) HasTextStates() bool {
	return u.user.State.TextStates.Len() > 0
}

func (u *userContextImpl) LastTextState() (State, int) {
	ts := u.user.State.TextStates.Last()
	return ts.State, ts.MessageID
}

func (u *userContextImpl) PopTextState() (State, int) {
	ts := u.user.State.TextStates.Pop()
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		State: &UserStateDiff{
			TextStates: u.user.State.TextStates,
		},
	})

	return ts.State, ts.MessageID
}

func (u *userContextImpl) PushTextState(state State, msgID int) {
	u.user.State.TextStates.Push(TextStateWithMessage{
		MessageID: msgID,
		State:     state,
	})
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		State: &UserStateDiff{
			TextStates: u.user.State.TextStates,
		},
	})
}

func (u *userContextImpl) RemoveTextState(state State, msgID int) {
	ok := u.user.State.TextStates.Remove(TextStateWithMessage{
		MessageID: msgID,
		State:     state,
	})
	if !ok {
		return
	}
	u.db.Update(u.user.Info.ID, &UserModelDiff{
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

	u.db.Update(u.user.Info.ID, &UserModelDiff{
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
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID: &u.user.Messages.MainID,
		},
	})
}

func (u *userContextImpl) SetHeadMessage(msgID int) {
	u.user.Messages.HeadID = msgID
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			HeadID: &u.user.Messages.HeadID,
		},
	})
}

func (u *userContextImpl) SetErrorMessage(msgID int) {
	u.user.Messages.ErrorID = msgID
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			ErrorID: &u.user.Messages.ErrorID,
		},
	})
}

func (u *userContextImpl) SetNotificationMessage(msgID int) {
	u.user.Messages.NotificationID = msgID
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			NotificationID: &u.user.Messages.NotificationID,
		},
	})
}

func (u *userContextImpl) AddHistoryMessage(msgID int) {
	u.user.Messages.HistoryIDs = append(u.user.Messages.HistoryIDs, msgID)
	u.db.Update(u.user.Info.ID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			HistoryIDs: u.user.Messages.HistoryIDs,
		},
	})
}

func (u *userContextImpl) Register() {
	u.user.Registered = time.Now().UTC()
	u.db.Update(u.user.Info.ID, &UserModelDiff{
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
	u.db.Update(u.user.Info.ID, &UserModelDiff{
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

	u.db.Update(u.user.Info.ID, &UserModelDiff{
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

	u.db.Update(u.user.Info.ID, &UserModelDiff{
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
		Info: newUserInfo(tUser),
		State: UserState{
			Main:          FirstRequest,
			MessageStates: make(map[int]State),
			TextStates:    abstract.NewUniqueStack[TextStateWithMessage](),
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
		ID:           tUser.ID,
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
	db    UserStorage
	log   Logger
}

func newUserManager(ctx context.Context, db UserStorage, log Logger) (*userManagerImpl, error) {
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

	m.log.Warn("bug: not found user in cache", "user_id", userID)

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
	userModel, isFound, err := m.db.Get(ctx, tUser.ID)
	if err != nil {
		return nil, errm.Wrap(err, "get")
	}
	if !isFound {
		userModel = newUserModel(tUser)
		if err := m.db.Insert(ctx, userModel); err != nil {
			return nil, errm.Wrap(err, "create user in db")
		}
	}

	user := m.newUserContext(userModel)
	m.users.Set(user.ID(), user)

	return user, nil
}

func (m *userManagerImpl) initAllUsersFromDB(ctx context.Context) error {
	users, err := m.db.GetAll(ctx)
	switch {
	case err == nil && len(users) == 0:
		m.log.Info("no users found in DB")
		return nil

	case err != nil:
		return errm.Wrap(err, "find all")
	}

	for _, u := range users {
		if u.IsDisabled {
			continue
		}
		m.users.Set(u.Info.ID, m.newUserContext(u))
	}

	m.log.Info("successfully init users", "count", m.users.Size())

	return nil
}

func (m *userManagerImpl) removeUserFromMemory(userID int64) {
	m.users.Delete(userID)
}

type inMemoryUserStorage struct {
	cache otter.Cache[int64, UserModel]
}

func newInMemoryUserStorage() (UserStorage, error) {
	s, err := otter.MustBuilder[int64, UserModel](100).Build()
	if err != nil {
		return nil, err
	}
	return &inMemoryUserStorage{
		cache: s,
	}, nil
}

func (m *inMemoryUserStorage) Insert(ctx context.Context, user UserModel) error {
	m.cache.Set(user.Info.ID, user)
	return nil
}

func (m *inMemoryUserStorage) Get(ctx context.Context, id int64) (UserModel, bool, error) {
	user, found := m.cache.Get(id)
	if !found {
		return UserModel{}, false, nil
	}
	return user, true, nil
}

func (m *inMemoryUserStorage) GetAll(ctx context.Context) ([]UserModel, error) {
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
			ID:           user.Info.ID,
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
