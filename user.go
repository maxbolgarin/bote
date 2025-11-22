package bote

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/erro"
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

// RegisterTextState registers a state that expects text input from the user.
// It is used to handle text input from the user in a correct order.
func RegisterTextStates(state ...State) bool {
	textStateManager.mu.Lock()
	defer textStateManager.mu.Unlock()
	for _, s := range state {
		textStateManager.states[s.String()] = struct{}{}
	}
	return true
}

// UserState is a bote implementation of [State] interface.
type UserState string

const (
	// NoChange is a state that means that user state should not be changed after Send of Edit.
	NoChange UserState = ""
	// FirstRequest is a state of a user after first request to bot.
	FirstRequest UserState = "first_request"
	// Unknown is a state of a user when hiw real state is not provided after creation.
	Unknown UserState = "unknown"
	// Disabled is a state of a disabled user.
	Disabled UserState = "disabled"
)

func NewUserState(state string) UserState {
	return UserState(state)
}

func ConvertUserState(state State) UserState {
	return NewUserState(state.String())
}

func (s UserState) String() string {
	return string(s)
}

func (s UserState) IsText() bool {
	return textStateManager.has(s)
}

func (s UserState) NotChanged() bool {
	return s == NoChange
}

// EncryptionKey is a type that represents an AES encryption key for private user ID.
type EncryptionKey struct {
	key     *[32]byte
	version *int64
}

// NewEncryptionKey creates a new [EncryptionKey].
func NewEncryptionKey(version *int64) *EncryptionKey {
	return &EncryptionKey{
		key:     abstract.NewEncryptionKey(),
		version: version,
	}
}

// NewEncryptionKeyFromString creates a new [EncryptionKey] from a string.
// Key should be a hex encoded string of 32 bytes.
func NewEncryptionKeyFromString(key string, version *int64) (*EncryptionKey, error) {
	bytes, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}
	if len(bytes) != 32 {
		return nil, errors.New("invalid encryption key length")
	}
	result := [32]byte{}
	copy(result[:], bytes)
	return &EncryptionKey{
		key:     &result,
		version: version,
	}, nil
}

// String converts [EncryptionKey] to a string.
func (k *EncryptionKey) String() string {
	return hex.EncodeToString(k.key[:])
}

// Clear clears the encryption key from memory to prevent reading it from memory dump.
// Use it after using the key to avoid leaking it.
func (k *EncryptionKey) Clear() {
	k.key = nil
	k.version = nil
}

// FullUserID is a structure that represents secure user ID with encrypted and HMAC parts.
type FullUserID struct {
	// IDPlain is Telegram user ID in plain format.
	// It is empty if privacy mode is strict.
	IDPlain *int64 `json:"id_plain,omitempty" bson:"id_plain,omitempty" db:"id_plain,omitempty"`

	// IDHMAC is a HMAC of IDPlain. It is used for searching user in DB by ID with plain ID is disabled.
	// It is used if privacy mode is strict.
	IDHMAC []byte `json:"id_hmac,omitempty" bson:"id_hmac,omitempty" db:"id_hmac,omitempty"`
	// IDEnc is an encrypted Telegram user ID. It is used to get readable IDPlain from IDEnc (HMAC is one way function).
	// It is used instead of IDPlain when privacy mode is strict.
	IDEnc []byte `json:"id_enc,omitempty" bson:"id_enc,omitempty" db:"id_enc,omitempty"`
	// HMACKeyVersion is a version of HMAC key for IDHMAC that used for HMAC of IDHMAC.
	// It is used if privacy mode is strict.
	HMACKeyVersion *int64 `json:"hmac_key_version,omitempty" bson:"hmac_key_version,omitempty" db:"hmac_key_version,omitempty"`
	// EncKeyVersion is a version of encryption key for IDEnc that used for encryption and decryption of IDEnc.
	// It is used if privacy mode is strict.
	EncKeyVersion *int64 `json:"enc_key_version,omitempty" bson:"enc_key_version,omitempty" db:"enc_key_version,omitempty"`
}

// NewPlainUserID creates a new [FullUserID] with the given ID as a plain text.
func NewPlainUserID(id int64) FullUserID {
	return FullUserID{
		IDPlain: &id,
	}
}

// NewPrivateUserID creates a new [FullUserID] with the given ID as an encrypted text.
// It do not store plain user ID, only encrypted and HMAC parts.
func NewPrivateUserID(id int64, hmacKey, encKey *EncryptionKey) (FullUserID, error) {
	if hmacKey == nil || encKey == nil {
		return FullUserID{}, errors.New("hmac key or enc key is nil")
	}

	var bytesID [8]byte
	binary.BigEndian.PutUint64(bytesID[:], uint64(id))
	idEnc, err := abstract.EncryptAES(bytesID[:], encKey.key)
	if err != nil {
		return FullUserID{}, err
	}

	mac := hmac.New(sha256.New, hmacKey.key[:])
	mac.Write(bytesID[:])
	idHMAC := mac.Sum(nil)

	return FullUserID{
		IDHMAC:         idHMAC,
		IDEnc:          idEnc,
		EncKeyVersion:  encKey.version,
		HMACKeyVersion: hmacKey.version,
	}, nil
}

// NewHMAC creates a new HMAC of provided ID using provided HMAC key.
func NewHMAC(id int64, hmacKey *EncryptionKey) []byte {
	if hmacKey == nil {
		return nil
	}
	var bytesID [8]byte
	binary.BigEndian.PutUint64(bytesID[:], uint64(id))
	mac := hmac.New(sha256.New, hmacKey.key[:])
	mac.Write(bytesID[:])
	return mac.Sum(nil)
}

// NewHMAC creates a new HMAC of provided ID using provided HMAC key.
func NewHMACString(id int64, hmacKey *EncryptionKey) string {
	return hex.EncodeToString(NewHMAC(id, hmacKey))
}

// IsEmpty returns true if all fields are nil.
func (u FullUserID) IsEmpty() bool {
	return u.IDPlain == nil && u.IDEnc == nil && u.EncKeyVersion == nil
}

// ID returns plain user ID if it is set. Otherwise it tries to decrypt encrypted ID.
// If encryption keys are provided, it tries to decrypt it with each key and returns plain user ID.
// If key is not provided, it returns error.
// If there is an error during decryption, it returns error.
func (u FullUserID) ID(encKeys ...*EncryptionKey) (int64, error) {
	if u.IDPlain != nil {
		return *u.IDPlain, nil
	}
	if u.IDEnc == nil {
		return 0, errors.New("encrypted ID is not set")
	}
	if len(encKeys) == 0 || encKeys[0] == nil {
		return 0, errors.New("keys are not provided")
	}
	var errs []error
	for i, encKey := range encKeys {
		if encKey == nil || encKey.key == nil {
			errs = append(errs, fmt.Errorf("%d: key is nil", i+1))
			continue
		}
		plaintext, err := abstract.DecryptAES(u.IDEnc, encKey.key)
		if err != nil {
			errs = append(errs, fmt.Errorf("%d: decrypt with key version=%d: %w", i+1, lang.Deref(encKey.version), err))
			continue
		}
		return int64(binary.BigEndian.Uint64(plaintext[:])), nil
	}
	return 0, fmt.Errorf("decrypt ID: %w", errors.Join(errs...))
}

// String returns plain user ID as a string if it is set, otherwise returns "[ENCRYPTED]".
func (u FullUserID) String() string {
	if u.IDPlain != nil {
		return strconv.FormatInt(*u.IDPlain, 10)
	}
	if len(u.IDHMAC) > 0 {
		if len(u.IDHMAC) > 8 {
			return hex.EncodeToString(u.IDHMAC)[:8]
		}
		return hex.EncodeToString(u.IDHMAC)
	}
	return "[ENCRYPTED]"
}

// User is an interface that represents user context in the bot.
type User interface {
	// ID returns plain Telegram user ID if it is called from [Context.User] (in handler)
	ID() int64
	// IDFull returns full user ID with encrypted and HMAC parts if they are set.
	IDFull() FullUserID
	// Username returns Telegram username (without @).
	// It is empty if privacy mode is strict.
	Username() string
	// Language returns Telegram user language code.
	Language() Language
	// Info returns user info.
	// It is empty if privacy mode is strict.
	Info() UserInfo
	// State returns current state for the given message ID.
	State(msgID int) (State, bool)
	// StateMain returns state for the Main message.
	StateMain() State
	// Messages returns all message IDs.
	Messages() UserMessages
	// IsDisabled returns true if user disabled the bot.
	IsDisabled() bool
	// String returns user string representation in format '[@username|id]'.
	String() string

	// Stats returns user stats.
	Stats() UserStat
	// LastSeenTime returns the time when user interacts with provided message.
	// If message ID is not provided, it returns the time when user interacts with bot's any message.
	LastSeenTime(optionalMsgID ...int) time.Time

	// UpdateLanguage updates user language.
	UpdateLanguage(language Language)

	// GetValue returns value from user context.
	GetValue(key string) (any, bool)

	// SetValue sets value in user context (persistent).
	SetValue(key string, value any)

	// DeleteValue deletes value from user context (persistent).
	DeleteValue(key string)

	// ClearCache deletes all values from SetValue function.
	ClearCache()
}

// UsersStorage is a storage for users.
// You should implement it in your application if you want to persist users between applicataion restarts.
type UsersStorage interface {
	// Insert inserts user in storage.
	// It is better to NOT modify document before saving to make everything working properly.
	Insert(ctx context.Context, userModel UserModel) error
	// Find returns user from storage. It returns true as a second argument if user was found without error.
	Find(ctx context.Context, id FullUserID) (UserModel, bool, error)

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
	UpdateAsync(id FullUserID, userModel *UserModelDiff)
}

const (
	// UserIDDBFieldName is a field name for plain user ID in DB.
	UserIDDBFieldName = "id.id_plain"
	// IDEncDBFieldName is a field name for encrypted user ID in DB.
	UserIDEncDBFieldName = "id.id_enc"
	// EncKeyVersionDBFieldName is a field name for encryption key version in DB.
	UserIDEncKeyVersionDBFieldName = "id.enc_key_version"
	// UserIDHMACDBFieldName is a field name for HMAC of user ID in DB.
	UserIDHMACDBFieldName = "id.id_hmac"
	// UserIDHMACKeyVersionDBFieldName is a field name for HMAC key version in DB.
	UserIDHMACKeyVersionDBFieldName = "id.hmac_key_version"
)

// UserModel is a structure that represents user in DB.
type UserModel struct {
	// ID is user ID.
	ID FullUserID `bson:"id" json:"id" db:"id"`
	// LanguageCode is Telegram user language code.
	// It is not encrypted even in strict privacy mode because it has low cardinality and cannot be used for identification.
	LanguageCode Language `bson:"language_code" json:"language_code" db:"language_code"`
	// ForceLanguageCode is a custom language code for user that can be set by user actions in bot.
	// It is not encrypted even in strict privacy mode because it has low cardinality and cannot be used for identification.
	ForceLanguageCode Language `bson:"force_language_code" json:"force_language_code" db:"force_language_code"`
	// Info contains user info, that can be obtained from Telegram.
	// It is empty if privacy mode is strict.
	Info UserInfo `bson:"info" json:"info" db:"info"`
	// Messages contains IDs of user messages.
	Messages UserMessages `bson:"messages" json:"messages" db:"messages"`
	// State contains state for every user's message.
	State MessagesState `bson:"state" json:"state" db:"state"`
	// Stats contains user stats.
	Stats UserStat `bson:"stats" json:"stats" db:"stats"`
	// IsBot is true if Telegram user is a bot.
	IsBot bool `bson:"is_bot" json:"is_bot" db:"is_bot"`
	// IsDisabled returns true if user is disabled. Disabled means that user blocks bot.
	IsDisabled bool `bson:"is_disabled" json:"is_disabled" db:"is_disabled"`

	// Values is a map of user values.
	Values map[string]any `bson:"values" json:"values" db:"values"`
}

type UserStat struct {
	// NumberOfStateChangesTotal is the total number of actions user made.
	NumberOfStateChangesTotal int `bson:"number_of_state_changes_total" json:"number_of_state_changes_total" db:"number_of_state_changes_total"`
	// LastSeenTime is the last time user interacted with the bot.
	LastSeenTime time.Time `bson:"last_seen_time" json:"last_seen_time" db:"last_seen_time"`
	// CreatedTime is the time when user was created.
	CreatedTime time.Time `bson:"created_time" json:"created_time" db:"created_time"`
	// DisabledTime is the time when user was disabled.
	DisabledTime time.Time `bson:"disabled_time" json:"disabled_time" db:"disabled_time"`
}

// UserInfo contains user info, that can be obtained from Telegram.
type UserInfo struct {
	// Username is Telegram username (without @).
	// It is empty if privacy mode is strict.
	Username string `bson:"username,omitempty" json:"username,omitempty" db:"username,omitempty"`
	// FirstName is Telegram user first name.
	// It is empty if privacy mode is enabled.
	FirstName string `bson:"first_name,omitempty" json:"first_name,omitempty" db:"first_name,omitempty"`
	// LastName is Telegram user last name.
	// It is empty if privacy mode is enabled.
	LastName string `bson:"last_name,omitempty" json:"last_name,omitempty" db:"last_name,omitempty"`
	// IsPremium is true if Telegram user has Telegram Premium.
	// It is empty if privacy mode is strict.
	IsPremium *bool `bson:"is_premium,omitempty" json:"is_premium,omitempty" db:"is_premium,omitempty"`
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

// MessagesState contains current user state and state history.
// State connects to message, every Main and Info message has its own state.
type MessagesState struct {
	// Main is the main state of the user, state of the Main message.
	Main UserState `bson:"main" json:"main" db:"main"`
	// MessageStates contains all states of the user for all messages. It is a map of message_id -> state.
	MessageStates map[int]UserState `bson:"message_states" json:"message_states" db:"message_states"`
	// MessagesAwaitingText is a unique stack that contains all messages IDs that awaits text.
	// Every message can produce text state and they should be handled as LIFO.
	MessagesAwaitingText []int `bson:"messages_awaiting_text" json:"messages_awaiting_text" db:"messages_awaiting_text"`

	// messagesStackInd is used to handle messages as a unique stack (swap in push)
	messagesStackInd map[int]int `bson:"-" json:"-" db:"-"`
}

// UserModelDiff contains changes that should be applied to user.
type UserModelDiff struct {
	LanguageCode      *Language         `bson:"language_code" json:"language_code" db:"language_code"`
	ForceLanguageCode *Language         `bson:"force_language_code" json:"force_language_code" db:"force_language_code"`
	Info              *UserInfoDiff     `bson:"info" json:"info" db:"info"`
	Messages          *UserMessagesDiff `bson:"messages" json:"messages" db:"messages"`
	State             *UserStateDiff    `bson:"state" json:"state" db:"state"`
	Stats             *UserStatDiff     `bson:"stats" json:"stats" db:"stats"`
	IsDisabled        *bool             `bson:"is_disabled" json:"is_disabled" db:"is_disabled"`
	IsBot             *bool             `bson:"is_bot" json:"is_bot" db:"is_bot"`
	Values            map[string]any    `bson:"values" json:"values" db:"values"`
}

// UserInfoDiff contains changes that should be applied to user info.
type UserInfoDiff struct {
	FirstName *string `bson:"first_name" json:"first_name" db:"first_name"`
	LastName  *string `bson:"last_name" json:"last_name" db:"last_name"`
	Username  *string `bson:"username" json:"username" db:"username"`
	IsPremium *bool   `bson:"is_premium" json:"is_premium" db:"is_premium"`
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
	Main                 *UserState        `bson:"main" json:"main" db:"main"`
	MessageStates        map[int]UserState `bson:"message_states" json:"message_states" db:"message_states"`
	MessagesAwaitingText []int             `bson:"messages_awaiting_text" json:"messages_awaiting_text" db:"messages_awaiting_text"`
}

type UserStatDiff struct {
	NumberOfStateChanges *int       `bson:"number_of_state_changes_total" json:"number_of_state_changes_total" db:"number_of_state_changes_total"`
	LastSeenTime         *time.Time `bson:"last_seen_time" json:"last_seen_time" db:"last_seen_time"`
	DisabledTime         *time.Time `bson:"disabled_time" json:"disabled_time" db:"disabled_time"`
}

func (u *UserModel) prepareAfterDB() {
	if u.Messages.LastActions == nil {
		u.Messages.LastActions = make(map[int]time.Time)
	}
	if u.State.MessageStates == nil {
		u.State.MessageStates = make(map[int]UserState)
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
	priv PrivacyMode

	btnName string
	payload string

	buttonMap   *abstract.SafeMap[string, InitBundle]
	isInitedMsg *abstract.SafeMap[int, bool]
	isPublic    bool

	// Add mutex for protecting user state and message updates
	mu sync.Mutex

	// We can get this id from Telegram Update in handler
	// We should clear it in the end of handler to prevent from leak into memory dump
	userID *int64
}

func (m *userManagerImpl) newUserContext(user UserModel, priv PrivacyMode) *userContextImpl {
	user.prepareAfterDB()
	return &userContextImpl{
		db:          m.db,
		user:        user,
		priv:        priv,
		buttonMap:   abstract.NewSafeMap[string, InitBundle](),
		isInitedMsg: abstract.NewSafeMap[int, bool](),
	}
}

func newPublicUserContext(user *tele.User) *userContextImpl {
	return &userContextImpl{
		user:     newUserModel(user, NewPlainUserID(user.ID), ""),
		isPublic: true,
	}
}

func (u *userContextImpl) ID() int64 {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.userID == nil {
		return lang.Deref(u.user.ID.IDPlain)
	}
	return *u.userID
}

func (u *userContextImpl) IDFull() FullUserID {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.ID
}

func (u *userContextImpl) Username() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Info.Username
}

func (u *userContextImpl) Language() Language {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.user.ForceLanguageCode != "" {
		return u.user.ForceLanguageCode
	}
	return u.user.LanguageCode
}

func (u *userContextImpl) UpdateLanguage(language Language) {
	if u.isPublic {
		return
	}

	u.mu.Lock()
	u.user.ForceLanguageCode = language
	u.mu.Unlock()
	u.db.UpdateAsync(u.user.ID, &UserModelDiff{
		ForceLanguageCode: &language,
	})
}

func (u *userContextImpl) Model() UserModel {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user
}

func (u *userContextImpl) Info() UserInfo {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Info
}

func (u *userContextImpl) Stats() UserStat {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.user.Stats
}

func (u *userContextImpl) State(msgID int) (State, bool) {
	if u.isPublic {
		return NoChange, false
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	st, ok := u.user.State.MessageStates[msgID]
	return State(st), ok
}

func (u *userContextImpl) StateMain() State {
	if u.isPublic {
		return NoChange
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	return State(u.user.State.Main)
}

func (u *userContextImpl) Messages() UserMessages {
	if u.isPublic {
		return UserMessages{}
	}

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
	if u.isPublic {
		return time.Time{}
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	if len(msgID) == 0 {
		return u.user.Stats.LastSeenTime
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
	if u.user.Info.Username == "" {
		return u.user.ID.String()
	}
	return "[@" + u.user.Info.Username + "|" + u.user.ID.String() + "]"
}

func (u *userContextImpl) GetValue(key string) (any, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.user.Values == nil {
		return nil, false
	}
	value, ok := u.user.Values[key]
	return value, ok
}

func (u *userContextImpl) SetValue(key string, value any) {
	u.mu.Lock()
	if u.user.Values == nil {
		u.user.Values = make(map[string]any)
	}
	u.user.Values[key] = value
	values := make(map[string]any, len(u.user.Values))
	maps.Copy(values, u.user.Values)
	u.mu.Unlock()

	u.db.UpdateAsync(u.user.ID, &UserModelDiff{
		Values: values,
	})
}

func (u *userContextImpl) DeleteValue(key string) {
	u.mu.Lock()
	delete(u.user.Values, key)
	values := make(map[string]any, len(u.user.Values))
	maps.Copy(values, u.user.Values)
	userID := u.user.ID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Values: values,
	})
}

func (u *userContextImpl) ClearCache() {
	u.mu.Lock()
	u.user.Values = nil
	userID := u.user.ID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Values: make(map[string]any),
	})
}

func (u *userContextImpl) setState(newState State, msgIDRaw ...int) {
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

	if ok && newState != State(currentState) && State(currentState).IsText() {
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
		u.user.State.Main = ConvertUserState(newState)
		upd.Main = &u.user.State.Main
	}

	u.user.Stats.LastSeenTime = time.Now().UTC()
	u.user.Stats.NumberOfStateChangesTotal++
	u.user.State.MessageStates[msgID] = ConvertUserState(newState)
	u.user.Messages.LastActions[msgID] = u.user.Stats.LastSeenTime

	upd.MessageStates = u.user.State.MessageStates

	// Make a copy of the data for the database update
	userID := u.user.ID
	lastActions := make(map[int]time.Time, len(u.user.Messages.LastActions))
	maps.Copy(lastActions, u.user.Messages.LastActions)
	lastSeenTime := u.user.Stats.LastSeenTime
	numberOfActionsTotal := u.user.Stats.NumberOfStateChangesTotal

	// Release the lock before making DB calls to avoid holding it too long
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State:    &upd,
		Messages: &UserMessagesDiff{LastActions: lastActions},
		Stats:    &UserStatDiff{LastSeenTime: &lastSeenTime, NumberOfStateChanges: &numberOfActionsTotal},
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

func (u *userContextImpl) lastTextMessage() int {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.user.State.MessagesAwaitingText) == 0 {
		return 0
	}
	u.prepareTextStates()

	return u.user.State.MessagesAwaitingText[len(u.user.State.MessagesAwaitingText)-1]
}

func (u *userContextImpl) lastTextMessageState() (int, State) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.user.State.MessagesAwaitingText) == 0 {
		return 0, NoChange
	}
	u.prepareTextStates()

	msgID := u.user.State.MessagesAwaitingText[len(u.user.State.MessagesAwaitingText)-1]
	st, ok := u.user.State.MessageStates[msgID]
	if !ok {
		return msgID, NoChange
	}

	return msgID, UserState(st)
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

func (u *userContextImpl) setMainMessage(msgID int) {
	u.mu.Lock()
	u.user.Messages.MainID = msgID

	// Capture values for the DB update
	userID := u.user.ID
	mainID := msgID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			MainID: &mainID,
		},
	})
}

func (u *userContextImpl) setHeadMessage(msgID int) {
	u.mu.Lock()
	u.user.Messages.HeadID = msgID

	// Capture values for the DB update
	userID := u.user.ID
	headID := msgID
	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		Messages: &UserMessagesDiff{
			HeadID: &headID,
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
		updatedMessageStates map[int]UserState
		updatedLastActions   map[int]time.Time
		updatedAwaitingText  []int
	)

	for _, msgIDToDelete := range msgIDs {
		for i, historyID := range u.user.Messages.HistoryIDs {
			if historyID != msgIDToDelete {
				continue
			}
			u.user.Messages.HistoryIDs = slices.Delete(u.user.Messages.HistoryIDs, i, i+1)
			delete(u.user.State.MessageStates, msgIDToDelete)
			delete(u.user.Messages.LastActions, msgIDToDelete)

			// Remove from MessagesAwaitingText (iterate backwards to avoid index issues)
			for j := len(u.user.State.MessagesAwaitingText) - 1; j >= 0; j-- {
				if u.user.State.MessagesAwaitingText[j] == msgIDToDelete {
					u.user.State.MessagesAwaitingText = slices.Delete(u.user.State.MessagesAwaitingText, j, j+1)
					break
				}
			}
			found = true
			break // Break inner loop after finding and deleting
		}
	}

	if found {
		updatedHistoryIDs = make([]int, len(u.user.Messages.HistoryIDs))
		copy(updatedHistoryIDs, u.user.Messages.HistoryIDs)

		updatedMessageStates = make(map[int]UserState, len(u.user.State.MessageStates))
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
	if user == nil {
		return
	}

	newLanguageCode := ParseLanguageOrDefault(user.LanguageCode)

	u.mu.Lock()

	var updateBase bool
	if u.user.IsBot != user.IsBot || u.user.LanguageCode != newLanguageCode {
		updateBase = true
	}

	if u.priv == PrivacyModeStrict {
		u.mu.Unlock()
		u.updateBase(updateBase, newLanguageCode, user.IsBot)
		return
	}

	infoToCheck := newUserInfoNoSanitize(user, u.priv)

	// Fast check because sanitize is expensive
	if infoToCheck == u.user.Info {
		u.mu.Unlock()
		u.updateBase(updateBase, newLanguageCode, user.IsBot)
		return
	}

	newInfo := newUserInfoWithSanitize(user, u.priv)
	if newInfo == u.user.Info {
		u.mu.Unlock()
		u.updateBase(updateBase, newLanguageCode, user.IsBot)
		return
	}

	u.user.Info = newInfo
	userID := u.user.ID

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		IsBot:        &user.IsBot,
		LanguageCode: &newLanguageCode,
		Info: &UserInfoDiff{
			FirstName: &newInfo.FirstName,
			LastName:  &newInfo.LastName,
			Username:  &newInfo.Username,
			IsPremium: newInfo.IsPremium,
		},
	})
}

func (u *userContextImpl) updateBase(updateBase bool, languageCode Language, isBot bool) {
	if updateBase {
		u.db.UpdateAsync(u.user.ID, &UserModelDiff{
			IsBot:        &isBot,
			LanguageCode: &languageCode,
		})
	}
}

func (u *userContextImpl) handleSend(newState State, mainMsgID, headMsgID int) {
	if mainMsgID == 0 {
		return
	}

	u.mu.Lock()

	currentTime := time.Now().UTC()
	u.user.Stats.LastSeenTime = currentTime
	u.user.Messages.LastActions[mainMsgID] = currentTime
	if headMsgID != 0 {
		u.user.Messages.LastActions[headMsgID] = currentTime
	}

	// Append to history IDs
	var historyIDs []int

	// Second+ message - main already exists, so we need to add it to history IDs
	if u.user.Messages.MainID != 0 {
		historyIDs = make([]int, len(u.user.Messages.HistoryIDs)+1)
		copy(historyIDs, u.user.Messages.HistoryIDs)
		historyIDs[len(historyIDs)-1] = u.user.Messages.MainID
		u.user.Messages.HistoryIDs = historyIDs
	}

	stateDiff := &UserStateDiff{}

	// If new state is changing, we need to update it
	if !newState.NotChanged() {
		u.user.State.Main = ConvertUserState(newState)
		u.user.State.MessageStates[mainMsgID] = ConvertUserState(newState)

		stateDiff.Main = &u.user.State.Main
		stateDiff.MessageStates = make(map[int]UserState, len(u.user.State.MessageStates))
		maps.Copy(stateDiff.MessageStates, u.user.State.MessageStates)
	}

	// If new state is text state, we need to add it to the stack
	if newState.IsText() {
		u.pushTextMessageLocked(mainMsgID)
		stateDiff.MessagesAwaitingText = u.user.State.MessagesAwaitingText
	}

	u.user.Messages.MainID = mainMsgID
	u.user.Messages.HeadID = headMsgID

	// Capture values for DB update
	userID := u.user.ID
	lastSeenTime := u.user.Stats.LastSeenTime
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
		State: stateDiff,
		Stats: &UserStatDiff{LastSeenTime: &lastSeenTime},
	}

	u.isInitedMsg.Set(mainMsgID, true)
	if headMsgID != 0 {
		u.isInitedMsg.Set(headMsgID, true)
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
	u.user.Stats.DisabledTime = currentTime
	u.user.IsDisabled = true
	u.user.State.Main = Disabled
	u.user.State.MessageStates[u.user.Messages.MainID] = Disabled

	// Capture values for DB update
	userID := u.user.ID
	disabledTime := u.user.Stats.DisabledTime
	isDisabled := u.user.IsDisabled
	stateMain := u.user.State.Main

	messageStates := make(map[int]UserState, len(u.user.State.MessageStates))
	maps.Copy(messageStates, u.user.State.MessageStates)

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State: &UserStateDiff{
			Main:          &stateMain,
			MessageStates: messageStates,
		},
		Stats: &UserStatDiff{
			DisabledTime: &disabledTime,
		},
		IsDisabled: &isDisabled,
	})
}

func (u *userContextImpl) enable() {
	u.mu.Lock()

	if !u.user.IsDisabled {
		u.mu.Unlock()
		return
	}

	u.user.Stats.DisabledTime = time.Time{}
	u.user.IsDisabled = false
	u.user.State.Main = FirstRequest
	u.user.State.MessageStates[u.user.Messages.MainID] = FirstRequest

	// Capture values for DB update
	userID := u.user.ID
	disabledTime := u.user.Stats.DisabledTime
	isDisabled := u.user.IsDisabled
	stateMain := u.user.State.Main

	messageStates := make(map[int]UserState, len(u.user.State.MessageStates))
	maps.Copy(messageStates, u.user.State.MessageStates)

	u.mu.Unlock()

	u.db.UpdateAsync(userID, &UserModelDiff{
		State: &UserStateDiff{
			Main:          &stateMain,
			MessageStates: messageStates,
		},
		Stats: &UserStatDiff{
			DisabledTime: &disabledTime,
		},
		IsDisabled: &isDisabled,
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

func (u *userContextImpl) setBtnAndPayload(btnName, payload string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	u.btnName = btnName
	u.payload = payload
}
func (u *userContextImpl) getBtnAndPayload() (btnName, payload string) {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.btnName, u.payload
}

func (u *userContextImpl) setUserID(userID int64) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.userID = &userID
}

func (u *userContextImpl) clearUserID() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.userID = nil
}

func newUserModel(tUser *tele.User, userID FullUserID, priv PrivacyMode) UserModel {
	if priv.IsStrict() {
		userID.IDPlain = nil
	}
	return UserModel{
		ID:           userID,
		IsBot:        tUser.IsBot,
		LanguageCode: ParseLanguageOrDefault(tUser.LanguageCode),
		Info:         newUserInfoWithSanitize(tUser, priv),
		State: MessagesState{
			Main:          FirstRequest,
			MessageStates: make(map[int]UserState),
		},
		Messages: UserMessages{
			LastActions: make(map[int]time.Time),
		},
		Stats: UserStat{
			LastSeenTime: time.Now().UTC(),
			CreatedTime:  time.Now().UTC(),
		},
		Values: make(map[string]any),
	}
}

func newUserInfoWithSanitize(tUser *tele.User, priv PrivacyMode) UserInfo {
	ui := newUserInfoNoSanitize(tUser, priv)
	return UserInfo{
		FirstName: sanitizeText(ui.FirstName, 1000),
		LastName:  sanitizeText(ui.LastName, 1000),
		Username:  sanitizeText(ui.Username, 1000),
		IsPremium: ui.IsPremium,
	}
}

func newUserInfoNoSanitize(tUser *tele.User, priv PrivacyMode) UserInfo {
	if tUser == nil {
		return UserInfo{}
	}

	switch priv {
	case PrivacyModeStrict:
		return UserInfo{}

	case PrivacyModeLow:
		return UserInfo{
			Username:  tUser.Username,
			IsPremium: &tUser.IsPremium,
		}

	default:
		return UserInfo{
			FirstName: tUser.FirstName,
			LastName:  tUser.LastName,
			Username:  tUser.Username,
			IsPremium: &tUser.IsPremium,
		}
	}
}

type userManagerImpl struct {
	users *userCache
	db    UsersStorage
	log   Logger
	metr  *metrics

	priv         PrivacyMode
	keysProvider KeysProvider
}

func newUserManager(opts Options) (*userManagerImpl, error) {
	users, err := newUserCache(opts.Config.Bot.Privacy.Mode, opts.KeysProvider,
		opts.Config.Bot.UserCacheCapacity, opts.Config.Bot.UserCacheTTL)
	if err != nil {
		return nil, erro.Wrap(err, "failed to create user cache")
	}

	m := &userManagerImpl{
		metr:         opts.metrics,
		users:        users,
		db:           opts.UserDB,
		log:          opts.Logger,
		priv:         opts.Config.Bot.Privacy.Mode,
		keysProvider: opts.KeysProvider,
	}

	return m, nil
}

func (m *userManagerImpl) prepareUser(tUser *tele.User) (*userContextImpl, error) {
	if tUser == nil {
		return nil, erro.New("cannot prepare user: telegram user is nil")
	}
	defer func() {
		m.metr.setUserCacheSize(m.users.size())
	}()

	user, found := m.users.get(tUser.ID)
	if found {
		user.update(tUser)
		return user, nil
	}

	return m.createUser(tUser,
		m.keysProvider.GetEncryptionKey(),
		m.keysProvider.GetHMACKey(),
	)
}

func (m *userManagerImpl) getUser(userID int64) *userContextImpl {
	if userID == 0 {
		m.log.Error("invalid user ID", "user_id", prepareUserID(userID, m.priv), "error", "userID cannot be zero")
		return m.createFallbackUser(&tele.User{ID: 1},
			m.keysProvider.GetEncryptionKey(),
			m.keysProvider.GetHMACKey(),
		)
	}

	user, found := m.users.get(userID)
	if found {
		return user
	}

	m.log.Warn("user not found in cache", "user_id", prepareUserID(userID, m.priv))

	tUser := &tele.User{ID: userID}
	encryptionKey := m.keysProvider.GetEncryptionKey()
	hmacKey := m.keysProvider.GetHMACKey()

	user, err := m.createUser(tUser, encryptionKey, hmacKey)
	if err != nil {
		m.log.Error("cannot create user after cache miss",
			"user_id", prepareUserID(userID, m.priv),
			"error", err.Error(),
			"error_type", fmt.Sprintf("%T", err))

		return m.createFallbackUser(tUser, encryptionKey, hmacKey)
	}

	return user
}

func (m *userManagerImpl) createFallbackUser(tUser *tele.User, encryptionKey, hmacKey *EncryptionKey) *userContextImpl {
	userID, err := NewPrivateUserID(tUser.ID, encryptionKey, hmacKey)
	if err != nil {
		m.log.Error("failed to create private user ID for fallback user", "error", err.Error())
		userID = NewPlainUserID(tUser.ID)
	}
	um := newUserModel(tUser, userID, m.priv)
	return m.newUserContext(um, m.priv)
}

func (m *userManagerImpl) getAllUsers() []User {
	out := make([]User, 0, m.users.size())
	m.users.forEach(func(value *userContextImpl) bool {
		out = append(out, value)
		return true
	})
	return out
}

func (m *userManagerImpl) createUser(tUser *tele.User, encryptionKey, hmacKey *EncryptionKey) (*userContextImpl, error) {
	if tUser == nil {
		return nil, erro.New("cannot create user: telegram user is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error

	userID := NewPlainUserID(tUser.ID)

	if m.priv.IsStrict() {
		userID, err = NewPrivateUserID(tUser.ID, encryptionKey, hmacKey)
		if err != nil {
			return nil, erro.Wrap(err, "failed to create user ID", "user_id", userID.String())
		}
	}

	userModel, isFound, err := m.db.Find(ctx, userID)
	if err != nil {
		return nil, erro.Wrap(err, "failed to find user in database", "user_id", userID.String())
	}

	if !isFound {
		userModel = newUserModel(tUser, userID, m.priv)
		if err := m.db.Insert(ctx, userModel); err != nil {
			return nil, erro.Wrap(err, "failed to insert new user into database", "user_id", userID.String())
		}
	}

	user := m.newUserContext(userModel, m.priv)
	if ok := m.users.set(tUser.ID, user); !ok {
		m.log.Warn("failed to add user to cache", "user_id", user.user.ID.String())
	}

	// Disabled user -> user blocked bot
	// If user gets here -> he makes request -> he unblock bot
	if userModel.IsDisabled {
		m.log.Info("enabling previously disabled user", "user_id", user.user.ID.String())
		user.enable()
	} else {
		m.log.Info("new user created", "user_id", user.user.ID.String())
	}

	return user, nil
}

func (m *userManagerImpl) disableUser(userID int64) {
	u, ok := m.users.get(userID)
	if !ok {
		return
	}
	u.disable()
	m.users.delete(userID)
}

func (m *userManagerImpl) deleteUser(userID int64) {
	m.users.delete(userID)
}

type inMemoryUserStorage struct {
	cache otter.Cache[int64, UserModel]
}

func newInMemoryUserStorage(userCacheCapacity int, userCacheTTL time.Duration) (UsersStorage, error) {
	c, err := otter.MustBuilder[int64, UserModel](userCacheCapacity).WithTTL(userCacheTTL).Build()
	if err != nil {
		return nil, erro.Wrap(err, "failed to create user cache with capacity %d", userCacheCapacity)
	}
	return &inMemoryUserStorage{
		cache: c,
	}, nil
}

func (m *inMemoryUserStorage) Insert(ctx context.Context, user UserModel) error {
	if ctx == nil {
		return erro.New("cannot insert user: context is nil")
	}

	if user.ID.IsEmpty() {
		return erro.New("cannot insert user: invalid user ID (zero)")
	}

	id := lang.Deref(user.ID.IDPlain)
	if id == 0 {
		return erro.New("cannot insert user: invalid user ID (zero)")
	}

	if !m.cache.Set(id, user) {
		return erro.Wrap(erro.New("cache rejected insertion"), "failed to insert user into in-memory storage")
	}
	return nil
}

func (m *inMemoryUserStorage) Find(ctx context.Context, id FullUserID) (UserModel, bool, error) {
	if ctx == nil {
		return UserModel{}, false, erro.New("cannot find user: context is nil")
	}

	if id.IsEmpty() {
		return UserModel{}, false, erro.New("cannot find user: invalid user ID (zero)")
	}

	user, found := m.cache.Get(lang.Deref(id.IDPlain))
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

func (m *inMemoryUserStorage) UpdateAsync(id FullUserID, diff *UserModelDiff) {
	user, found := m.cache.Get(lang.Deref(id.IDPlain))
	if !found {
		return
	}

	// Ensure user model is properly initialized
	(&user).prepareAfterDB()

	if diff.Info != nil {
		user.Info = UserInfo{
			FirstName: lang.Check(lang.Deref(diff.Info.FirstName), user.Info.FirstName),
			LastName:  lang.Check(lang.Deref(diff.Info.LastName), user.Info.LastName),
			Username:  lang.Check(lang.Deref(diff.Info.Username), user.Info.Username),
			IsPremium: user.Info.IsPremium,
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
		// Preserve messagesStackInd when updating State
		messagesStackInd := user.State.messagesStackInd
		if messagesStackInd == nil {
			messagesStackInd = make(map[int]int)
		}
		user.State = MessagesState{
			Main:                 lang.Check(lang.Deref(diff.State.Main), user.State.Main),
			MessageStates:        lang.If(len(diff.State.MessageStates) > 0, diff.State.MessageStates, user.State.MessageStates),
			MessagesAwaitingText: lang.If(diff.State.MessagesAwaitingText != nil, diff.State.MessagesAwaitingText, user.State.MessagesAwaitingText),
			messagesStackInd:     messagesStackInd,
		}
		// Rebuild messagesStackInd if MessagesAwaitingText was updated
		if diff.State.MessagesAwaitingText != nil {
			user.State.messagesStackInd = make(map[int]int, len(user.State.MessagesAwaitingText))
			for i, v := range user.State.MessagesAwaitingText {
				user.State.messagesStackInd[v] = i
			}
		}
	}

	if diff.Stats != nil {
		user.Stats.LastSeenTime = lang.CheckTime(lang.Deref(diff.Stats.LastSeenTime), user.Stats.LastSeenTime)
		user.Stats.DisabledTime = lang.CheckTime(lang.Deref(diff.Stats.DisabledTime), user.Stats.DisabledTime)
		user.Stats.NumberOfStateChangesTotal = lang.Check(lang.Deref(diff.Stats.NumberOfStateChanges), user.Stats.NumberOfStateChangesTotal)
	}
	if diff.IsDisabled != nil {
		user.IsDisabled = lang.Check(lang.Deref(diff.IsDisabled), user.IsDisabled)
	}

	if diff.IsBot != nil {
		user.IsBot = lang.Check(lang.Deref(diff.IsBot), user.IsBot)
	}

	if diff.LanguageCode != nil {
		user.LanguageCode = lang.Check(lang.Deref(diff.LanguageCode), user.LanguageCode)
	}

	if diff.Values != nil {
		// Make a copy to avoid sharing the same map reference
		user.Values = make(map[string]any, len(diff.Values))
		maps.Copy(user.Values, diff.Values)
	}

	m.cache.Set(lang.Deref(id.IDPlain), user)
}

type textStateManagerImpl struct {
	states map[string]struct{}
	mu     sync.RWMutex
}

var textStateManager = textStateManagerImpl{
	states: make(map[string]struct{}),
}

func (t *textStateManagerImpl) has(state State) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	_, ok := t.states[state.String()]
	return ok
}

type userCache struct {
	usersByPlainID *otter.Cache[int64, *userContextImpl]
	usersByHMACID  *otter.Cache[string, *userContextImpl]
	keysProvider   KeysProvider
}

func newUserCache(priv PrivacyMode, keysProvider KeysProvider, userCacheCapacity int, userCacheTTL time.Duration) (out *userCache, err error) {
	out = &userCache{
		keysProvider: keysProvider,
	}
	if priv.IsStrict() {
		out.usersByHMACID, err = newCache[string](userCacheCapacity, userCacheTTL)
	} else {
		out.usersByPlainID, err = newCache[int64](userCacheCapacity, userCacheTTL)
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *userCache) get(id int64) (user *userContextImpl, found bool) {
	if c.usersByPlainID != nil {
		user, found = c.usersByPlainID.Get(id)
	} else if c.usersByHMACID != nil {
		idHMAC := NewHMAC(id, c.keysProvider.GetHMACKey())
		user, found = c.usersByHMACID.Get(hex.EncodeToString(idHMAC))
	}
	return user, found
}

func (c *userCache) set(id int64, user *userContextImpl) bool {
	if c.usersByPlainID != nil {
		return c.usersByPlainID.Set(id, user)

	} else if c.usersByHMACID != nil {
		idHMAC := NewHMAC(id, c.keysProvider.GetHMACKey())
		return c.usersByHMACID.Set(hex.EncodeToString(idHMAC), user)
	}
	return false
}

func (c *userCache) delete(id int64) {
	if c.usersByPlainID != nil {
		c.usersByPlainID.Delete(id)
	} else if c.usersByHMACID != nil {
		idHMAC := NewHMAC(id, c.keysProvider.GetHMACKey())
		c.usersByHMACID.Delete(hex.EncodeToString(idHMAC))
	}
}

func (c *userCache) forEach(callback func(user *userContextImpl) bool) {
	if c.usersByPlainID != nil {
		c.usersByPlainID.Range(func(_ int64, value *userContextImpl) bool {
			return callback(value)
		})
	} else if c.usersByHMACID != nil {
		c.usersByHMACID.Range(func(_ string, value *userContextImpl) bool {
			return callback(value)
		})
	}
}

func (c *userCache) size() int {
	if c.usersByPlainID != nil {
		return c.usersByPlainID.Size()
	} else if c.usersByHMACID != nil {
		return c.usersByHMACID.Size()
	}
	return 0
}

func newCache[K comparable](capacity int, ttl time.Duration) (*otter.Cache[K, *userContextImpl], error) {
	// Configure in-memory storage with proper eviction settings
	c, err := otter.MustBuilder[K, *userContextImpl](capacity).
		// Add cost function to better manage memory
		Cost(func(_ K, value *userContextImpl) uint32 {
			// Cost is roughly based on the number of messages a user has
			// This helps prioritize eviction of users with more stored messages
			return uint32(1 + len(value.user.Messages.HistoryIDs))
		}).
		// Set TTL for inactive users to prevent memory leaks
		WithTTL(ttl).
		Build()
	if err != nil {
		return nil, erro.Wrap(err, "failed to create user cache with capacity %d", capacity)
	}
	return &c, nil
}

func prepareUserID(userID int64, priv PrivacyMode) string {
	if priv.IsStrict() {
		return "[REDACTED]"
	}
	return strconv.FormatInt(userID, 10)
}
