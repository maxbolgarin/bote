package bote

import (
	"context"
	"sync"
	"time"

	"github.com/joomcode/errorx"
	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/logze"
	"github.com/maypok86/otter"
	"github.com/panjf2000/ants/v2"
	tele "gopkg.in/telebot.v4"
)

const (
	TimezonesCollectionName = "timezones"
	UsersCollectionName     = "users"

	userCacheCapacity = 1000
)

type userManagerImpl struct {
	users otter.Cache[int64, User]
	db    *MongoDB
	pool  *ants.Pool
	log   logze.Logger
}

func newUserManager(ctx context.Context, db *MongoDB, l logze.Logger) (*userManagerImpl, error) {
	c, err := otter.MustBuilder[int64, User](userCacheCapacity).Build()
	if err != nil {
		return nil, err
	}
	// TODO: check consumprion of memory
	pool, err := ants.NewPool(userCacheCapacity, ants.WithPreAlloc(true))
	if err != nil {
		return nil, err
	}
	m := &userManagerImpl{
		users: c,
		pool:  pool,
		db:    db,
		log:   l,
	}

	err = m.initAllUsersFromDB(ctx)
	if err != nil {
		return nil, errm.Wrap(err, "init all users")
	}

	return m, nil
}

func (m *userManagerImpl) CreateUser(ctx context.Context, tUser *tele.User) (User, error) {
	user, found := m.users.Get(tUser.ID)
	if found {
		user.Update(tUser)
		return user, nil
	}
	return m.createUser(ctx, tUser)
}

func (m *userManagerImpl) GetUser(userID int64) User {
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
		m.log.Error(err, "cannot create user after cache miss", "user_id", userID)
		user, _ = m.newUserContext(ctx, newUserModel(tUser))
	}

	return user
}

func (m *userManagerImpl) GetAllUsers() []User {
	out := make([]User, 0, m.users.Size())
	m.users.Range(func(key int64, value User) bool {
		out = append(out, value)
		return true
	})
	return out
}

func (m *userManagerImpl) removeUserFromMemory(userID int64) {
	m.users.Delete(userID)
}

func (m *userManagerImpl) createUser(ctx context.Context, tUser *tele.User) (User, error) {
	var userModel userModel
	err := m.db.Collection(UsersCollectionName).FindOne(ctx, userModel, Filter{"id": tUser.ID})
	switch {
	case errm.Is(err, ErrNotFound):
		// First request to bot
		userModel, err = m.createUserInDB(ctx, tUser)
		if err != nil {
			return nil, errm.Wrap(err, "create user in db")
		}

	case err != nil:
		return nil, errm.Wrap(err, "find")
	}

	user, err := m.newUserContext(ctx, userModel)
	if err != nil {
		return nil, errm.Wrap(err, "new user")
	}
	m.users.Set(user.ID(), user)

	return user, nil
}

func (m *userManagerImpl) createUserInDB(ctx context.Context, tUser *tele.User) (userModel, error) {
	um := newUserModel(tUser)

	err := m.db.Collection(UsersCollectionName).Insert(ctx, um)
	switch {
	case errorx.IsDuplicate(err):
		// Data race?
		um, found := m.users.Get(um.Info.ID)
		if found {
			um.Update(tUser)
		}

	case err != nil:
		return userModel{}, err
	}

	return um, nil
}

func (m *userManagerImpl) initAllUsersFromDB(ctx context.Context) error {
	tm := abstract.StartTimer()

	var users []userModel

	err := m.db.Collection(UsersCollectionName).FindAll(ctx, &users)
	switch {
	case errm.Is(err, ErrNotFound):
		m.log.Info("no users found in DB")
		return nil

	case err != nil:
		return errm.Wrap(err, "find all")
	}

	var (
		errList    = errm.NewSafeList()
		wg         sync.WaitGroup
		totalUsers int
	)

	for _, u := range users {
		if u.IsDisabled {
			continue
		}

		wg.Add(1)
		m.pool.Submit(func() {
			defer wg.Done()

			uc, err := m.newUserContext(ctx, u)
			if err != nil {
				errList.Wrap(err, "new user", "user_id", u.Info.ID)
				return
			}
			m.users.Set(u.Info.ID, uc)
		})
		totalUsers++
	}

	wg.Wait()

	if m.users.Size() == 0 {
		return errList.Err()
	}

	if errList.NotEmpty() {
		m.log.Warnf("init %d/%d users with errors", m.users.Size(), totalUsers, "error", errList.Err())
	}

	m.log.Infof("successfully init %d/%d users", m.users.Size(), totalUsers, "elapsed_time", tm.ElapsedTime())

	return nil
}

func (m *userManagerImpl) newUserContext(ctx context.Context, user userModel) (User, error) {
	return nil, nil
}
