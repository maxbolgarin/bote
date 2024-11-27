package bote

import (
	"context"

	tele "gopkg.in/telebot.v4"
)

type UserProvider interface {
	GetUser(userID int64) User
	GetAllUsers() []User
}

type User interface {
	ID() int64
	Username() string
	Info() UserInfo

	Update(user *tele.User)

	SetErrorMessage(msgID int)

	Disable(ctx context.Context) error
}

type userImpl struct {
	user    userModel
	db      *Collection
	asyncDB *AsyncCollection
}

func (u *userImpl) ID() int64 {
	return u.user.Info.ID
}

func (u *userImpl) Username() string {
	return u.user.Info.Username
}

func (u *userImpl) Info() UserInfo {
	return u.user.Info
}

func (u *userImpl) Update(user *tele.User) {
	newInfo := newUserInfo(user)
	if newInfo == u.user.Info {
		return
	}
	u.asyncDB.SetFields(u.user.Info.IDString(), "SetFields",
		idFilter(u.user.Info.ID), Updates{
			"info": newInfo,
		})
}

func (u *userImpl) Disable(ctx context.Context) error {
	if u.user.IsDisabled {
		return nil
	}

	u.user.IsDisabled = true
	u.user.State.Main = NotRegistered

	return u.db.SetFields(ctx, idFilter(u.user.Info.ID), Updates{
		"disabled":   true,
		"state.main": NotRegistered,
	})
}

func idFilter(id int64) Filter {
	return Filter{"id": id}
}
