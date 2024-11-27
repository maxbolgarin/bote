package bote

type State struct {
	Name   string `bson:"name"`
	IsText bool   `bson:"is_text"`
}

var (
	NoChange      State = State{Name: ""}
	NotRegistered State = State{Name: "not_registered"}
)
