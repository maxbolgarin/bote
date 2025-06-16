package bote

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/maxbolgarin/lang"
)

// MessageProvider is an interface for providing messages based on the user language code.
type MessageProvider interface {
	// Messages returns messages for a specific language.
	Messages(languageCode string) Messages
}

// Messages is a collection of messages for a specific language.
type Messages interface {
	// CloseBtn is a message on inline keyboard button that closes the error message.
	// Remain it empty if you don't want to show this button.
	CloseBtn() string

	// GeneralError is a general error message that sends when an unhandled error occurs.
	GeneralError() string

	// PrepareMessage calls before every Send, SendMain, Edit, EditMain or EditHistory.
	// Provide zero msgID in Send and SendMain methods.
	// If isHistorical is true, it means that the message is a history message called by EditHistory.
	PrepareMessage(msg string, u User, newState State, msgID int, isHistorical bool) string
}

// Format is a type of message formatting in Telegram in HTML format.
type Format string

const (
	Bold      Format = "<b>"
	Italic    Format = "<i>"
	Code      Format = "<code>"
	Strike    Format = "<s>"
	Underline Format = "<u>"
	Pre       Format = "<pre>"

	boldEnd      = "</b>"
	italicEnd    = "</i>"
	codeEnd      = "</code>"
	strikeEnd    = "</s>"
	underlineEnd = "</u>"
	preEnd       = "</pre>"
)

// F returns a formatted string.
func F(msg string, formats ...Format) string {
	for _, f := range formats {
		switch f {
		case Bold:
			msg = string(Bold) + msg + boldEnd
		case Italic:
			msg = string(Italic) + msg + italicEnd
		case Code:
			msg = string(Code) + msg + codeEnd
		case Strike:
			msg = string(Strike) + msg + strikeEnd
		case Underline:
			msg = string(Underline) + msg + underlineEnd
		case Pre:
			msg = string(Pre) + msg + preEnd
		}
	}
	return msg
}

// Ff returns a formatted string, just like [fmt.Sprintf].
func Ff(msg string, args ...any) string {
	return fmt.Sprintf(msg, args...)
}

// FBold returns a string with bold formatting.
func FBold(msg string) string {
	return string(Bold) + msg + boldEnd
}

// FB returns a string with bold formatting.
func FB(msg string) string {
	return FBold(msg)
}

// FItalic returns a string with italic formatting.
func FItalic(msg string) string {
	return string(Italic) + msg + italicEnd
}

// FI returns a string with italic formatting.
func FI(msg string) string {
	return FItalic(msg)
}

// FCode returns a string with code formatting.
func FCode(msg string) string {
	return string(Code) + msg + codeEnd
}

// FC returns a string with code formatting.
func FC(msg string) string {
	return FCode(msg)
}

// FStrike returns a string with strike formatting.
func FStrike(msg string) string {
	return string(Strike) + msg + strikeEnd
}

// FS returns a string with strike formatting.
func FS(msg string) string {
	return FStrike(msg)
}

// FUnderline returns a string with underline formatting.
func FUnderline(msg string) string {
	return string(Underline) + msg + underlineEnd
}

// FU returns a string with underline formatting.
func FU(msg string) string {
	return FUnderline(msg)
}

// FPre returns a string with pre formatting.
func FPre(msg string) string {
	return string(Pre) + msg + preEnd
}

// FP returns a string with pre formatting.
func FP(msg string) string {
	return FPre(msg)
}

// FBoldf returns a string with bold formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FBoldf(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Bold) + fmt.Sprintf(msg, args...) + boldEnd
	}
	return string(Bold) + msg + boldEnd
}

// FBf returns a string with bold formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FBf(msg string, args ...any) string {
	return FBoldf(msg, args...)
}

// FItalicf returns a string with italic formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FItalicf(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Italic) + fmt.Sprintf(msg, args...) + italicEnd
	}
	return string(Italic) + msg + italicEnd
}

// FI returns a string with italic formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FIf(msg string, args ...any) string {
	return FItalicf(msg, args...)
}

// FCodef returns a string with code formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FCodef(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Code) + fmt.Sprintf(msg, args...) + codeEnd
	}
	return string(Code) + msg + codeEnd
}

// FCf returns a string with code formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FCf(msg string, args ...any) string {
	return FCodef(msg, args...)
}

// FStrikef returns a string with strike formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FStrikef(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Strike) + fmt.Sprintf(msg, args...) + strikeEnd
	}
	return string(Strike) + msg + strikeEnd
}

// FSf returns a string with strike formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FSf(msg string, args ...any) string {
	return FStrikef(msg, args...)
}

// FUnderlinef returns a string with underline formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FUnderlinef(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Underline) + fmt.Sprintf(msg, args...) + underlineEnd
	}
	return string(Underline) + msg + underlineEnd
}

// FUf returns a string with underline formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FUf(msg string, args ...any) string {
	return FUnderlinef(msg, args...)
}

// FPref returns a string with pre formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FPref(msg string, args ...any) string {
	if len(args) > 0 {
		return string(Pre) + fmt.Sprintf(msg, args...) + preEnd
	}
	return string(Pre) + msg + preEnd
}

// FPf returns a string with pre formatting.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func FPf(msg string, args ...any) string {
	return FPref(msg, args...)
}

// FBoldUnderline returns a string with bold and underline formatting.
func FBoldUnderline(msg string) string {
	return FBold(FUnderline(msg))
}

// FBU is an alias for [FBoldUnderline].
func FBU(msg string) string {
	return FBoldUnderline(msg)
}

// FBoldCode returns a string with bold and code formatting.
func FBoldCode(msg string) string {
	return FBold(FCode(msg))
}

// FBC is an alias for [FBoldCode].
func FBC(msg string) string {
	return FBoldCode(msg)
}

// FBoldItalic returns a string with bold and italic formatting.
func FBoldItalic(msg string) string {
	return FBold(FItalic(msg))
}

// FBI is an alias for [FBoldItalic].
func FBI(msg string) string {
	return FBoldItalic(msg)
}

// MessageBuilder is a wrapper for [strings.Builder] with additional methods.
// You should not copy it. Empty value of [MessageBuilder] is ready to use.
type MessageBuilder struct {
	strings.Builder
}

// NewBuilder creates a new Builder instance.
func NewBuilder() *MessageBuilder {
	return &MessageBuilder{}
}

// Write writes a string to the builder.
// It is an alias for [strings.Builder.WriteString].
func (b *MessageBuilder) Write(msg ...string) {
	for _, s := range msg {
		b.WriteString(s)
	}
}

// Writef writes a string to the builder.
// It is an alias for [strings.Builder.WriteString].
// If args are provided, it uses [fmt.Sprintf] to format the string.
func (b *MessageBuilder) Writef(msg string, args ...any) {
	if len(args) > 0 {
		b.WriteString(fmt.Sprintf(msg, args...))
	} else {
		b.WriteString(msg)
	}
}

// Writeln writes a string to the builder and adds a newline at the end.
func (b *MessageBuilder) Writeln(s ...string) {
	for _, s := range s {
		b.WriteString(s + "\n")
	}
}

// Writeln writes a string to the builder and adds a newline at the end.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func (b *MessageBuilder) Writelnf(s string, args ...any) {
	if len(args) > 0 {
		b.WriteString(fmt.Sprintf(s, args...) + "\n")
	} else {
		b.WriteString(s + "\n")
	}
}

// Writelnn writes a string to the builder and adds two newlines at the end.
func (b *MessageBuilder) Writelnn(s ...string) {
	for _, s := range s {
		b.WriteString(s + "\n\n")
	}
}

// Writelnn writes a string to the builder and adds two newlines at the end.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func (b *MessageBuilder) Writelnnf(s string, args ...any) {
	if len(args) > 0 {
		b.WriteString(fmt.Sprintf(s, args...) + "\n\n")
	} else {
		b.WriteString(s + "\n\n")
	}
}

// Writebn writes a string to the builder and adds a newline at the beginning.
// It is an alias for [strings.Builder.WriteString].
func (b *MessageBuilder) Writebn(msg ...string) {
	for _, s := range msg {
		b.WriteString("\n" + s)
	}
}

// Writebnf writes a string to the builder and adds a newline at the beginning.
// It is an alias for [strings.Builder.WriteString].
// If args are provided, it uses [fmt.Sprintf] to format the string.
func (b *MessageBuilder) Writebnf(msg string, args ...any) {
	if len(args) > 0 {
		b.WriteString("\n" + fmt.Sprintf(msg, args...))
	} else {
		b.WriteString("\n" + msg)
	}
}

// Writebln writes a string to the builder and adds a newline at the beginning and a newline at the end.
func (b *MessageBuilder) Writebln(msg ...string) {
	for _, s := range msg {
		b.WriteString("\n" + s + "\n")
	}
}

// Writeblnf writes a string to the builder and adds a newline at the beginning and a newline at the end.
// If args are provided, it uses [fmt.Sprintf] to format the string.
func (b *MessageBuilder) Writeblnf(msg string, args ...any) {
	if len(args) > 0 {
		b.WriteString("\n" + fmt.Sprintf(msg, args...) + "\n")
	} else {
		b.WriteString("\n" + msg + "\n")
	}
}

// WriteIf writes either writeTrue or writeFalse depending on the value of first argument.
func (b *MessageBuilder) WriteIf(condition bool, writeTrue, writeFalse string) {
	if condition {
		b.WriteString(writeTrue)
	} else {
		b.WriteString(writeFalse)
	}
}

// WriteIf writes either writeTrue or writeFalse depending on the value of first argument.
func (b *MessageBuilder) WriteIfF(condition bool, writeTrue, writeFalse string, args ...any) {
	if condition {
		if len(args) > 0 {
			b.WriteString(fmt.Sprintf(writeTrue, args...))
		} else {
			b.WriteString(writeTrue)
		}
	} else {
		if len(args) > 0 {
			b.WriteString(fmt.Sprintf(writeFalse, args...))
		} else {
			b.WriteString(writeFalse)
		}
	}
}

// WriteBytes writes a byte slice to the builder.
// It is an alias for [strings.Builder.Write].
func (b *MessageBuilder) WriteBytes(data ...[]byte) {
	for _, d := range data {
		b.Builder.Write(d)
	}
}

// IsEmpty returns true if the builder's length is 0.
func (b *MessageBuilder) IsEmpty() bool {
	return b.Builder.Len() == 0
}

// GetFilledMessage returns a formatted string with aligned left and right parts.
func GetFilledMessage(left, right, sep, fill string, maxLeft, maxRight, maxLen int) string {
	dataLen := len(left) + len(right) + len(sep)
	if dataLen > maxLen {
		panic(fmt.Sprintf("invalid state: provided data length %d > %d max length", dataLen, maxLen))
	}

	if len(right) > maxRight {
		panic(fmt.Sprintf("invalid state: right data length %d > %d max right", len(right), maxRight))
	}

	if len(left) > maxLeft {
		panic(fmt.Sprintf("invalid state: left data length %d > %d max left", len(left), maxLeft))
	}

	sepPos := maxLen - maxRight - len(sep)
	if len(left) > sepPos {
		panic(fmt.Sprintf("invalid state: left data length %d > %d space for left", len(left), sepPos))
	}

	numberOfLines := sepPos - maxLeft
	numberOfSpaces := sepPos - numberOfLines - len(left) + 1
	out := left + strings.Repeat("  ", lang.If(numberOfSpaces >= 0, numberOfSpaces, 0)) +
		strings.Repeat(fill, lang.If(numberOfLines >= 0, numberOfLines, 0)) + sep + right

	return out
}

func newDefaultMessageProvider() MessageProvider {
	return &defaultMessageProvider{}
}

type defaultMessageProvider struct{}

func (defaultMessageProvider) Messages(languageCode string) Messages {
	switch languageCode {
	case "ru":
		return &ruMessages{}
	default:
		return &enMessages{}
	}
}

type ruMessages struct{}

func (ruMessages) CloseBtn() string {
	return "Закрыть"
}

func (ruMessages) GeneralError() string {
	return "Произошла внутренняя ошибка"
}

func (ruMessages) PrepareMessage(msg string, _ User, _ State, _ int, _ bool) string {
	return msg
}

type enMessages struct{}

func (enMessages) CloseBtn() string {
	return "Close"
}

func (enMessages) GeneralError() string {
	return "There is an internal error"
}

func (enMessages) PrepareMessage(msg string, _ User, _ State, _ int, _ bool) string {
	return msg
}

// Case-insensitive regex patterns to detect and remove malicious sequences
// This pattern matches "javascript:" or "data:" with optional whitespace
var maliciousPattern = regexp.MustCompile(`(?i)(?:javascript|data)\s*:`)

// sanitizeText sanitizes text inputs to prevent injection attacks
func sanitizeText(text string, maxLength ...int) string {
	if text == "" {
		return ""
	}

	// Remove ASCII control characters (0-31 and 127)
	var cleaned strings.Builder
	for _, r := range text {
		if r >= 32 && r != 127 {
			cleaned.WriteRune(r)
		}
	}
	text = cleaned.String()

	// Normalize to lowercase for pattern matching
	lowerText := strings.ToLower(text)

	// Remove malicious patterns from the original text by finding matches in lowercase
	// and removing corresponding parts from the original text
	matches := maliciousPattern.FindAllStringIndex(lowerText, -1)
	if len(matches) > 0 {
		// Remove matches in reverse order to maintain correct indices
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			text = text[:match[0]] + text[match[1]:]
		}
	}

	// Trim whitespace
	text = strings.TrimSpace(text)

	// HTML escape to prevent XSS (done after pattern removal)
	text = html.EscapeString(text)

	if len(maxLength) > 0 {
		if utf8.RuneCountInString(text) > maxLength[0] {
			text = string([]rune(text)[:maxLength[0]])
		}
	}

	return text
}

// String builders benchmark
// https://gist.github.com/dtjm/c6ebc86abe7515c988ec

// go1.17
// goos: darwin
// goarch: amd64
// pkg: ***
// cpu: Intel(R) Core(TM) i7-4980HQ CPU @ 2.80GHz
// BenchmarkBufferWithReset-8       	29284802	       51.32 ns/op	       0 B/op	       0 allocs/op
// BenchmarkConcatOneLine-8         	20360762	       94.10 ns/op	       0 B/op	       0 allocs/op
// BenchmarkStringBuilderWithReset-8   	15469124	       93.51 ns/op	      24 B/op	       2 allocs/op
// BenchmarkBuffer-8                	16493529	       102.2 ns/op	      64 B/op	       1 allocs/op
// BenchmarkConcat-8                	 6670018	       257.7 ns/op	      32 B/op	       4 allocs/op
// BenchmarkSprintf-8               	 3249290	       393.9 ns/op	      96 B/op	       6 allocs/op

// So the best is 'concat one line', means you just use '+' to concat strings, e.g. return a + " " + b + " " + c
// Buffer with reset is fast only in cycle, because it allocates space on the first run and then reuses it
// strings.Builder is better than bytes.Buffer in case of building strings, so we use it here
