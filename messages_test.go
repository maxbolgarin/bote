package bote

import (
	"testing"
)

type testMessages struct{}

func (testMessages) CloseBtn() string { return "Close" }
func (testMessages) GeneralError() string { return "General error" }
func (testMessages) PrepareMessage(msg string, u User, newState State, msgID int, isHistorical bool) string {
	// For testing, just echo the message unchanged
	return msg
}

type testProvider struct{}

func (testProvider) Messages(language Language) Messages { return testMessages{} }

func TestMessagesFormattingHelpers(t *testing.T) {
	if got := FBold("x"); got == "x" {
		t.Fatalf("FBold should wrap text")
	}
	if got := FItalic("x"); got == "x" {
		t.Fatalf("FItalic should wrap text")
	}
	if got := FCode("x"); got == "x" {
		t.Fatalf("FCode should wrap text")
	}
	if got := FStrike("x"); got == "x" {
		t.Fatalf("FStrike should wrap text")
	}
	if got := FUnderline("x"); got == "x" {
		t.Fatalf("FUnderline should wrap text")
	}
	if got := FPre("x"); got == "x" {
		t.Fatalf("FPre should wrap text")
	}

	// Builder
	b := NewBuilder()
	b.Write("a", "b")
	b.Writeln("c")
	b.Writelnf("%s", "d")
	b.Writelnn("e")
	b.Writelnnf("%s", "f")
	b.Writebn("g")
	b.Writebnf("%s", "h")
	b.Writebln("i")
	b.Writeblnf("%s", "j")
	b.WriteIf(true, "k")
	b.WriteIfF(false, "T%v", "F%v", 1)
	b.WritelnIf(true, "l")
	b.WritelnIfF(false, "T%v", "F%v", 2)
	if s := b.String(); s == "" {
		t.Fatalf("builder should produce some content")
	}
}

func TestDefaultMessageProvider(t *testing.T) {
	p := newDefaultMessageProvider()
	m := p.Messages(LanguageDefault)
	if m == nil {
		t.Fatal("default message provider returned nil")
	}
	// Ensure PrepareMessage is idempotent for simple case
	u := &userContextImpl{user: UserModel{ID: 1}}
	out := m.PrepareMessage("hello", u, NoChange, 0, false)
	if out != "hello" {
		t.Fatalf("unexpected PrepareMessage output: %q", out)
	}

	// Custom provider wiring
	var pr MessageProvider = testProvider{}
	if got := pr.Messages(LanguageDefault); got.CloseBtn() == "" {
		t.Fatalf("custom provider returned unexpected messages implementation")
	}
}

