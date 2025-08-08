package bote

import (
	"testing"

	tele "gopkg.in/telebot.v4"
)

func TestCreateBtnData(t *testing.T) {
	if got := CreateBtnData(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := CreateBtnData("a"); got != "a" {
		t.Fatalf("expected 'a', got %q", got)
	}
	if got := CreateBtnData("a", "", "b", "c"); got != "a|b|c" {
		t.Fatalf("expected 'a|b|c', got %q", got)
	}
}

func TestGetBtnIDAndUnique_LengthsAndDerivation(t *testing.T) {
	name := "button-name"
	id, unique := getBtnIDAndUnique(name)
	if id == "" || unique == "" {
		t.Fatalf("id or unique should not be empty")
	}
	if len(unique) <= len(id) {
		t.Fatalf("unique should include random suffix: id=%q unique=%q", id, unique)
	}
	// unique without random part should decode back to name
	decoded := getNameFromUnique(unique)
	if decoded != name {
		t.Fatalf("expected decoded name %q, got %q", name, decoded)
	}
	// id derivation from unique should match id
	if got := getIDFromUnique(unique); got != id {
		t.Fatalf("expected id %q from unique, got %q", id, got)
	}
}

func TestInlineBuilder_SingleRowAndColumns(t *testing.T) {
	btns := []tele.Btn{
		{Text: "1"}, {Text: "2"}, {Text: "3"}, {Text: "4"},
	}
	kb := NewKeyboardWithLength(OneBytePerRune)
	for i, btn := range btns {
		if i%2 == 0 && i != 0 {
			kb.StartNewRow()
		}
		kb.Add(btn)
	}
	mk := kb.CreateInlineMarkup()
	if got := len(mk.InlineKeyboard); got != 2 {
		t.Fatalf("expected 2 rows, got %d", got)
	}
	if len(mk.InlineKeyboard[0]) != 2 || len(mk.InlineKeyboard[1]) != 2 {
		t.Fatalf("expected 2 buttons per row")
	}
}

func TestRemoveKeyboard(t *testing.T) {
	mk := RemoveKeyboard()
	if mk == nil || !mk.RemoveKeyboard {
		t.Fatalf("expected RemoveKeyboard to be true")
	}
}

func TestGetIDFromUnparsedData(t *testing.T) {
	// unique has 8 bytes where last 4 are random; id should be unique without last 4 bytes
	unique := "abcdef12"
	data := unique + "|payload"
	got := getIDFromUnparsedData(data)
	exp := unique[:len(unique)-4]
	if got != exp {
		t.Fatalf("expected %q, got %q", exp, got)
	}
}
