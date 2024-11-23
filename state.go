package bote

type State struct {
	Name   string `bson:"name"`
	IsText bool   `bson:"is_text"`
}

var (
	NotRegistered State = State{Name: "not_registered"}
)

type State2 string

// Current user state in bot
const (
	NoChange     State2 = ""
	FirstRequest State2 = "first_request"
	Disabled     State2 = "disabled"
)

// Historical states
const ()

func (s State2) String() string {
	return string(s)
}

func (s State2) IsEmpty() bool {
	return s == ""
}

func (s State2) NotChanged() bool {
	return s == NoChange
}

func (s State2) IsChanged() bool {
	return s != NoChange
}

func (s State2) IsDisabled() bool {
	return s == Disabled
}
