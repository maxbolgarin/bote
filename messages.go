package bote

type MessageProvider interface {
	Messages(languageCode string) Messages
}

type Messages interface {
	GeneralError() string
	PrepareMainMessage(main string, u User) string
}

func NewDefaultMessageProvider() MessageProvider {
	return &defaultMessageProvider{GeneralError: "Произошла ошибка"}
}

type defaultMessageProvider struct {
	GeneralError string
}

func (d defaultMessageProvider) Messages(languageCode string) Messages {
	return &defaultMessages{generalError: d.GeneralError}
}

type defaultMessages struct {
	generalError string
}

func (d defaultMessages) GeneralError() string {
	return d.generalError
}

func (d defaultMessages) PrepareMainMessage(main string, u User) string {
	return main
}
