package bote

import (
	"testing"
)

func TestParseLanguage(t *testing.T) {
	cases := []struct {
		in  string
		exp Language
		ok  bool
	}{
		{"en", LanguageEnglish, true},
		{"EN", LanguageEnglish, true},
		{"ru", LanguageRussian, true},
		{"", "", false},
		{"toolong", "", false},
		{"xx", "", false},
	}
	for _, c := range cases {
		lang, err := ParseLanguage(c.in)
		if c.ok {
			if err != nil {
				t.Fatalf("ParseLanguage(%q) unexpected error: %v", c.in, err)
			}
			if lang != c.exp {
				t.Fatalf("ParseLanguage(%q) = %q, want %q", c.in, lang, c.exp)
			}
		} else {
			if err == nil {
				t.Fatalf("ParseLanguage(%q) expected error", c.in)
			}
		}
	}

	if got := ParseLanguageOrDefault("xx"); got != LanguageDefault {
		t.Fatalf("ParseLanguageOrDefault: expected default %q, got %q", LanguageDefault, got)
	}
}

func TestMustLanguagePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic for invalid language code")
		}
	}()
	_ = MustLanguage("xx")
}
