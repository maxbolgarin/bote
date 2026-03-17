package bote

import (
	"testing"
)

type testMessages struct{}

func (testMessages) CloseBtn() string     { return "Close" }
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
	u := &userContextImpl{user: UserModel{ID: NewPlainUserID(1)}}
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

func TestGetFilledMessage(t *testing.T) {
	left := "A"
	right := "B"
	sep := " : "
	fill := "-"
	maxLeft := 5
	maxRight := 5
	maxLen := 20
	out := GetFilledMessage(left, right, sep, fill, maxLeft, maxRight, maxLen)
	if out == "" {
		t.Fatalf("expected non-empty filled message")
	}
}

func TestSanitizeText(t *testing.T) {
	cases := []struct {
		in  string
		max int
	}{
		{"hello", 100},
		{"\x00\x01bad\x7F", 100},
		{"<b>bold</b>", 100},
		{"javascript:alert(1)", 100},
		{"data:payload", 100},
		{"trim  ", 3},
	}
	for _, c := range cases {
		_ = sanitizeText(c.in, c.max) // should not panic
	}

	out := sanitizeText("javascript:alert(1)")
	if contains(out, "javascript:") {
		t.Fatalf("expected to remove javascript: pattern, got %q", out)
	}
	out = sanitizeText("data:foo")
	if contains(out, "data:") {
		t.Fatalf("expected to remove data: pattern, got %q", out)
	}

	// max length handling (by runes)
	if got := sanitizeText("абвгд", 3); runeCount(got) != 3 {
		t.Fatalf("expected 3 runes, got %d, %q", runeCount(got), got)
	}
}

// TestSanitizeTextExtended tests the sanitization fixes for URI schemes and multi-byte handling
func TestSanitizeTextExtended(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		blocked  string // substring that must NOT appear in output
		allowed  string // substring that MUST appear in output (optional)
	}{
		{"javascript lowercase", "javascript:alert(1)", "javascript:", ""},
		{"javascript mixed case", "JaVaScRiPt:alert(1)", "JaVaScRiPt:", ""},
		{"javascript uppercase", "JAVASCRIPT:alert(1)", "JAVASCRIPT:", ""},
		{"data scheme", "data:text/html,<h1>hi</h1>", "data:", ""},
		{"vbscript scheme", "vbscript:MsgBox", "vbscript:", ""},
		{"blob scheme", "blob:http://example.com", "blob:", ""},
		{"javascript with spaces", "javascript  :alert(1)", "javascript  :", ""},
		{"legitimate colon", "time: 12:30pm", "", "time: 12:30pm"},
		{"null bytes removed", "hello\x00world", "\x00", "helloworld"},
		{"DEL character removed", "hello\x7Fworld", "\x7F", "helloworld"},
		{"tabs preserved", "hello\tworld", "", "hello\tworld"},
		{"newlines preserved", "hello\nworld", "", "hello\nworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := sanitizeText(tt.input)
			if tt.blocked != "" && contains(out, tt.blocked) {
				t.Errorf("expected %q to be removed, got %q", tt.blocked, out)
			}
			if tt.allowed != "" && !contains(out, tt.allowed) {
				t.Errorf("expected %q to be preserved, got %q", tt.allowed, out)
			}
		})
	}

	// Turkish İ followed by javascript: should not cause index mismatch
	t.Run("turkish character before pattern", func(t *testing.T) {
		out := sanitizeText("İİİ javascript:alert(1)")
		if contains(out, "javascript:") {
			t.Errorf("expected javascript: to be removed with Turkish chars, got %q", out)
		}
		if !contains(out, "İİİ") {
			t.Errorf("expected Turkish chars to be preserved, got %q", out)
		}
	})
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (len(sub) == 0 || index(s, sub) >= 0) }

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func runeCount(s string) int { return len([]rune(s)) }
