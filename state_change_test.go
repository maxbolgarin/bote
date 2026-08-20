package bote

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "github.com/maxbolgarin/telebot/v4"
)

type stateChange struct {
	from  State
	to    State
	msgID int
}

// recordingHook returns an OnStateChange callback plus an accessor for what it saw.
func recordingHook() (StateChangeFunc, func() []stateChange) {
	var (
		mu   sync.Mutex
		seen []stateChange
	)
	fn := func(_ User, from, to State, msgID int) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, stateChange{from: from, to: to, msgID: msgID})
	}
	return fn, func() []stateChange {
		mu.Lock()
		defer mu.Unlock()
		out := make([]stateChange, len(seen))
		copy(out, seen)
		return out
	}
}

func newManagerWithHook(t *testing.T, hook StateChangeFunc) *userManagerImpl {
	t.Helper()
	opts := newTestOptions()
	opts.OnStateChange = hook
	um, err := newUserManager(context.Background(), opts)
	require.NoError(t, err)
	return um
}

func TestOnStateChangeReportsTransitions(t *testing.T) {
	hook, seen := recordingHook()
	um := newManagerWithHook(t, hook)

	user, err := um.prepareUser(&tele.User{ID: 9001, Username: "hooktest"})
	require.NoError(t, err)

	user.setState(UserState("main_menu"))
	user.setState(UserState("stack_info"))
	user.setState(UserState("assistant_chat"))

	got := seen()
	require.Len(t, got, 3, "one callback per real transition")

	// The first sighting of a message has no previous state.
	assert.Equal(t, UserState(""), got[0].from)
	assert.Equal(t, UserState("main_menu"), got[0].to)

	assert.Equal(t, UserState("main_menu"), got[1].from)
	assert.Equal(t, UserState("stack_info"), got[1].to)

	assert.Equal(t, UserState("stack_info"), got[2].from)
	assert.Equal(t, UserState("assistant_chat"), got[2].to)
}

// NoChange is the "keep whatever state you had" sentinel — it is not a transition and must
// not be reported, or every no-op handler would look like a screen view.
func TestOnStateChangeIgnoresNoChange(t *testing.T) {
	hook, seen := recordingHook()
	um := newManagerWithHook(t, hook)

	user, err := um.prepareUser(&tele.User{ID: 9002, Username: "nochange"})
	require.NoError(t, err)

	user.setState(UserState("main_menu"))
	user.setState(NoChange)
	user.setState(NoChange)

	assert.Len(t, seen(), 1)
}

// The hook must fire after the user lock is released, so a callback that reads the user
// (StateMain takes the same mutex) cannot deadlock. Without the ordering this test hangs.
func TestOnStateChangeCallbackMayReadTheUser(t *testing.T) {
	var observed State
	um := newManagerWithHook(t, func(u User, _, _ State, _ int) {
		observed = u.StateMain()
	})

	user, err := um.prepareUser(&tele.User{ID: 9003, Username: "reentrant"})
	require.NoError(t, err)

	user.setState(UserState("profile"))

	assert.Equal(t, UserState("profile"), observed,
		"callback should observe the already-committed state")
}

// A faulty analytics callback must never take the bot down mid-update.
func TestOnStateChangePanicIsContained(t *testing.T) {
	um := newManagerWithHook(t, func(User, State, State, int) {
		panic("callback blew up")
	})

	user, err := um.prepareUser(&tele.User{ID: 9004, Username: "panicky"})
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		user.setState(UserState("main_menu"))
	})
	// The transition itself must still have been applied.
	assert.Equal(t, UserState("main_menu"), user.StateMain())
}

func TestOnStateChangeIsOptional(t *testing.T) {
	um := newManagerWithHook(t, nil)

	user, err := um.prepareUser(&tele.User{ID: 9005, Username: "nohook"})
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		user.setState(UserState("main_menu"))
	})
	assert.Equal(t, UserState("main_menu"), user.StateMain())
}

func TestOnStateChangeReportsMessageID(t *testing.T) {
	hook, seen := recordingHook()
	um := newManagerWithHook(t, hook)

	user, err := um.prepareUser(&tele.User{ID: 9006, Username: "msgids"})
	require.NoError(t, err)

	user.setState(UserState("first"), 111)
	user.setState(UserState("second"), 222)

	got := seen()
	require.Len(t, got, 2)
	assert.Equal(t, 111, got[0].msgID)
	assert.Equal(t, 222, got[1].msgID)
	// Different messages carry independent state, so the second is also a first sighting.
	assert.Equal(t, UserState(""), got[1].from)
}
