package bote

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	tele "gopkg.in/telebot.v4"
)

// MockUsersStorage is a mock implementation of UsersStorage interface using testify/mock
type MockUsersStorage struct {
	mock.Mock
}

func (m *MockUsersStorage) Insert(ctx context.Context, userModel UserModel) error {
	args := m.Called(ctx, userModel)
	return args.Error(0)
}

func (m *MockUsersStorage) Find(ctx context.Context, id int64) (UserModel, bool, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(UserModel), args.Bool(1), args.Error(2)
}

func (m *MockUsersStorage) UpdateAsync(id int64, userModel *UserModelDiff) {
	m.Called(id, userModel)
}

// MockPoller is a mock implementation of tele.Poller interface using testify/mock
type MockPoller struct {
	mock.Mock
	updates chan tele.Update
	stopped bool
}

func NewMockPoller() *MockPoller {
	return &MockPoller{
		updates: make(chan tele.Update, 10),
	}
}

func (m *MockPoller) Poll(bot *tele.Bot, updates chan tele.Update, stop chan struct{}) {
	m.Called(bot, updates, stop)
	for {
		select {
		case upd := <-m.updates:
			select {
			case updates <- upd:
			case <-stop:
				m.stopped = true
				return
			}
		case <-stop:
			m.stopped = true
			return
		}
	}
}

func (m *MockPoller) SendUpdate(upd tele.Update) {
	if !m.stopped {
		select {
		case m.updates <- upd:
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// MockLogger is a mock implementation of Logger interface using testify/mock
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Info(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Warn(msg string, args ...any) {
	m.Called(msg, args)
}

func (m *MockLogger) Error(msg string, args ...any) {
	m.Called(msg, args)
}

// MockMessages is a mock implementation of Messages interface using testify/mock
type MockMessages struct {
	mock.Mock
}

func (m *MockMessages) CloseBtn() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMessages) GeneralError() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMessages) PrepareMessage(msg string, u User, newState State, msgID int, isHistorical bool) string {
	args := m.Called(msg, u, newState, msgID, isHistorical)
	return args.String(0)
}

// MockMessageProvider is a mock implementation of MessageProvider interface using testify/mock
type MockMessageProvider struct {
	mock.Mock
}

func (m *MockMessageProvider) Messages(language Language) Messages {
	args := m.Called(language)
	return args.Get(0).(Messages)
}

// MockUpdateLogger is a mock implementation of UpdateLogger interface using testify/mock
type MockUpdateLogger struct {
	mock.Mock
}

func (m *MockUpdateLogger) Log(updateType UpdateType, args ...any) {
	m.Called(updateType, args)
}

// MockUser is a mock implementation of User interface using testify/mock
type MockUser struct {
	mock.Mock
}

func (m *MockUser) ID() int64 {
	args := m.Called()
	return args.Get(0).(int64)
}

func (m *MockUser) Username() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockUser) Language() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockUser) Info() UserInfo {
	args := m.Called()
	return args.Get(0).(UserInfo)
}

func (m *MockUser) State(msgID int) (State, bool) {
	args := m.Called(msgID)
	return args.Get(0).(State), args.Bool(1)
}

func (m *MockUser) StateMain() State {
	args := m.Called()
	return args.Get(0).(State)
}

func (m *MockUser) Messages() UserMessages {
	args := m.Called()
	return args.Get(0).(UserMessages)
}

func (m *MockUser) Stats() UserStat {
	args := m.Called()
	return args.Get(0).(UserStat)
}

func (m *MockUser) LastSeenTime() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

func (m *MockUser) IsDisabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockUser) String() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockUser) GetValue(key string) (any, bool) {
	args := m.Called(key)
	return args.Get(0), args.Bool(1)
}

func (m *MockUser) SetValue(key string, value any) {
	m.Called(key, value)
}

func (m *MockUser) DeleteValue(key string) {
	m.Called(key)
}

func (m *MockUser) ClearCache() {
	m.Called()
}

func (m *MockUser) UpdateLanguage(languageCode string) {
	m.Called(languageCode)
}
