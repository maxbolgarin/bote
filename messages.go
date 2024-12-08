package bote

// MessageProvider is an interface for providing messages based on the user language code.
type MessageProvider interface {
	// Messages returns messages for a specific language.
	Messages(languageCode string) Messages
}

// Messages is a collection of messages for a specific language.
type Messages interface {
	// GeneralError returns the general error message that sends when an unhandled error occurs.
	GeneralError() string

	// PrepareMainMessage calls before every Send or Edit of the main message.
	PrepareMainMessage(main string, u User) string
}

func newDefaultMessageProvider() MessageProvider {
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
