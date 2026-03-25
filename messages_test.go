package bote

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>bold</b>", "&lt;b&gt;bold&lt;/b&gt;"},
		{"a & b", "a &amp; b"},
		{"no special chars", "no special chars"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, EscapeHTML(tt.input))
		})
	}
}

func TestSanitizeTextFileScheme(t *testing.T) {
	t.Run("removes_file_scheme", func(t *testing.T) {
		result := sanitizeText("file:///etc/passwd")
		assert.NotContains(t, result, "file:")
	})
	t.Run("case_insensitive", func(t *testing.T) {
		result := sanitizeText("FILE:///etc/passwd")
		assert.NotContains(t, result, "FILE:")
	})
	t.Run("with_whitespace", func(t *testing.T) {
		result := sanitizeText("file  :/etc/passwd")
		assert.NotContains(t, result, "file")
	})
}

func TestFormatAliases(t *testing.T) {
	msg := "hello"

	assert.Equal(t, FBold(msg), FB(msg), "FB should equal FBold")
	assert.Equal(t, FItalic(msg), FI(msg), "FI should equal FItalic")
	assert.Equal(t, FCode(msg), FC(msg), "FC should equal FCode")
	assert.Equal(t, FStrike(msg), FS(msg), "FS should equal FStrike")
	assert.Equal(t, FUnderline(msg), FU(msg), "FU should equal FUnderline")
	assert.Equal(t, FPre(msg), FP(msg), "FP should equal FPre")

	// Each alias must actually wrap (not return unchanged)
	assert.NotEqual(t, msg, FB(msg))
	assert.NotEqual(t, msg, FI(msg))
	assert.NotEqual(t, msg, FC(msg))
	assert.NotEqual(t, msg, FS(msg))
	assert.NotEqual(t, msg, FU(msg))
	assert.NotEqual(t, msg, FP(msg))
}

func TestFormatMulti(t *testing.T) {
	msg := "world"

	// No formats — identity
	assert.Equal(t, msg, F(msg))

	// Single format matches individual function
	assert.Equal(t, FBold(msg), F(msg, Bold))
	assert.Equal(t, FItalic(msg), F(msg, Italic))
	assert.Equal(t, FCode(msg), F(msg, Code))
	assert.Equal(t, FStrike(msg), F(msg, Strike))
	assert.Equal(t, FUnderline(msg), F(msg, Underline))
	assert.Equal(t, FPre(msg), F(msg, Pre))

	// Multiple formats — result should differ from single-format
	combined := F(msg, Bold, Italic)
	assert.NotEqual(t, FBold(msg), combined)
	assert.NotEqual(t, FItalic(msg), combined)

	// Ff is just fmt.Sprintf
	assert.Equal(t, "num=42", Ff("num=%d", 42))
	assert.Equal(t, "plain", Ff("plain"))
}

func TestFormatSprintf(t *testing.T) {
	// Without args — behaves like non-f variant
	assert.Equal(t, FBold("x"), FBoldf("x"))
	assert.Equal(t, FItalic("x"), FItalicf("x"))
	assert.Equal(t, FCode("x"), FCodef("x"))
	assert.Equal(t, FStrike("x"), FStrikef("x"))
	assert.Equal(t, FUnderline("x"), FUnderlinef("x"))
	assert.Equal(t, FPre("x"), FPref("x"))

	// With args — formats the string then wraps
	assert.Equal(t, FBold("v=7"), FBoldf("v=%d", 7))
	assert.Equal(t, FItalic("v=7"), FItalicf("v=%d", 7))
	assert.Equal(t, FCode("v=7"), FCodef("v=%d", 7))
	assert.Equal(t, FStrike("v=7"), FStrikef("v=%d", 7))
	assert.Equal(t, FUnderline("v=7"), FUnderlinef("v=%d", 7))
	assert.Equal(t, FPre("v=7"), FPref("v=%d", 7))

	// Aliases match their full-name counterparts
	assert.Equal(t, FBoldf("t=%s", "a"), FBf("t=%s", "a"))
	assert.Equal(t, FItalicf("t=%s", "a"), FIf("t=%s", "a"))
	assert.Equal(t, FCodef("t=%s", "a"), FCf("t=%s", "a"))
	assert.Equal(t, FStrikef("t=%s", "a"), FSf("t=%s", "a"))
	assert.Equal(t, FUnderlinef("t=%s", "a"), FUf("t=%s", "a"))
	assert.Equal(t, FPref("t=%s", "a"), FPf("t=%s", "a"))
}

func TestFormatCombinations(t *testing.T) {
	msg := "text"

	bu := FBoldUnderline(msg)
	assert.Equal(t, bu, FBU(msg), "FBU should equal FBoldUnderline")
	assert.Equal(t, FBold(FUnderline(msg)), bu)
	assert.NotEqual(t, msg, bu)

	bc := FBoldCode(msg)
	assert.Equal(t, bc, FBC(msg), "FBC should equal FBoldCode")
	assert.Equal(t, FBold(FCode(msg)), bc)
	assert.NotEqual(t, msg, bc)

	bi := FBoldItalic(msg)
	assert.Equal(t, bi, FBI(msg), "FBI should equal FBoldItalic")
	assert.Equal(t, FBold(FItalic(msg)), bi)
	assert.NotEqual(t, msg, bi)
}

func TestBuilderEdgeCases(t *testing.T) {
	t.Run("IsEmpty", func(t *testing.T) {
		b := NewBuilder()
		assert.True(t, b.IsEmpty(), "new builder should be empty")
		b.Write("x")
		assert.False(t, b.IsEmpty(), "builder with content should not be empty")
	})

	t.Run("WriteBytes", func(t *testing.T) {
		b := NewBuilder()
		b.WriteBytes([]byte("hello"))
		assert.Equal(t, "hello", b.String())
		// multiple slices
		b2 := NewBuilder()
		b2.WriteBytes([]byte("foo"), []byte("bar"))
		assert.Equal(t, "foobar", b2.String())
	})

	t.Run("Writef_with_args", func(t *testing.T) {
		b := NewBuilder()
		b.Writef("n=%d", 5)
		assert.Equal(t, "n=5", b.String())
	})

	t.Run("Writef_no_args", func(t *testing.T) {
		b := NewBuilder()
		b.Writef("plain")
		assert.Equal(t, "plain", b.String())
	})

	t.Run("WritelnIfFf_true", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfFf(true, "val=%d", 3)
		assert.Equal(t, "val=3\n", b.String())
	})

	t.Run("WritelnIfFf_false", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfFf(false, "val=%d", 3)
		assert.Equal(t, "", b.String(), "false condition should write nothing")
	})

	t.Run("WriteIfF_false_branch", func(t *testing.T) {
		b := NewBuilder()
		b.WriteIfF(false, "TRUE%d", "FALSE%d", 9)
		assert.Equal(t, "FALSE9", b.String())
	})

	t.Run("WriteIfF_true_branch", func(t *testing.T) {
		b := NewBuilder()
		b.WriteIfF(true, "TRUE%d", "FALSE%d", 9)
		assert.Equal(t, "TRUE9", b.String())
	})

	t.Run("WriteIfF_no_args_false", func(t *testing.T) {
		b := NewBuilder()
		b.WriteIfF(false, "T", "F")
		assert.Equal(t, "F", b.String())
	})

	t.Run("WritelnIfF_false_branch_with_args", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfF(false, "T%d", "F%d", 4)
		assert.Equal(t, "F4\n", b.String())
	})

	t.Run("WritelnIfF_true_branch_with_args", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfF(true, "T%d", "F%d", 4)
		assert.Equal(t, "T4\n", b.String())
	})

	t.Run("WritelnIfF_no_args_false", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfF(false, "T", "F")
		assert.Equal(t, "F\n", b.String())
	})

	t.Run("WritelnIfF_no_args_true", func(t *testing.T) {
		b := NewBuilder()
		b.WritelnIfF(true, "T", "F")
		assert.Equal(t, "T\n", b.String())
	})
}
