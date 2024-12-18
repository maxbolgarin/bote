package bote

import (
	"fmt"
	"strings"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/lang"
)

// MessageProvider is an interface for providing messages based on the user language code.
type MessageProvider interface {
	// Messages returns messages for a specific language.
	Messages(languageCode string) Messages
}

// Messages is a collection of messages for a specific language.
type Messages interface {
	// GeneralError is a general error message that sends when an unhandled error occurs.
	GeneralError() string

	// FatalError is a fatal error message that sends when bot cannot work further and must be restarted.
	// It generally offers the user to press /start to restart the bot.
	FatalError() string

	// PrepareMainMessage calls before every Send, SendMain, Edit or EditMain for the main message.
	PrepareMainMessage(main string, u User, newState State) string

	// PrepareHistoryMessage calls before every EditHistory for the history message.
	PrepareHistoryMessage(main string, u User, newState State, msgID int) string
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

// Max possible length of entity ID (telegram bot constraint)
const entityIDLength = 12

func NewID() string {
	return abstract.GetRandomString(entityIDLength)
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
func (b *MessageBuilder) Write(msg string) {
	b.WriteString(msg)
}

// Writef writes a formatted string to the builder using fmt.Sprintf.
func (b *MessageBuilder) Writef(format string, args ...any) {
	b.WriteString(fmt.Sprintf(format, args...))
}

// Writeln writes a string to the builder and adds a newline at the end.
func (b *MessageBuilder) Writeln(s string) {
	b.WriteString(s + "\n")
}

// WriteIf writes either msgIf or msgElse depending on the value of first argument.
func (b *MessageBuilder) WriteIf(toWrite bool, msgIf, msgElse string) {
	if toWrite {
		b.WriteString(msgIf)
	} else {
		b.WriteString(msgElse)
	}
}

// WriteBytes writes a byte slice to the builder.
// It is an alias for [strings.Builder.Write].
func (b *MessageBuilder) WriteBytes(data []byte) {
	b.Builder.Write(data)
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

func (d defaultMessageProvider) Messages(languageCode string) Messages {
	switch languageCode {
	case "ru":
		return &ruMessages{}
	default:
		return &enMessages{}
	}
}

type ruMessages struct{}

func (d ruMessages) GeneralError() string {
	return "Произошла внутренняя ошибка"
}

func (d ruMessages) FatalError() string {
	return "Произошла внутренняя ошибка!\nНажмите /start, чтобы восстановить бота"
}

func (d ruMessages) PrepareMainMessage(main string, u User, newState State) string {
	return main
}

func (d ruMessages) PrepareHistoryMessage(main string, u User, newState State, msgID int) string {
	return main
}

type enMessages struct{}

func (d enMessages) GeneralError() string {
	return "There is an error"
}

func (d enMessages) FatalError() string {
	return "There is an internal error!\nPress /start to recover"
}

func (d enMessages) PrepareMainMessage(main string, u User, newState State) string {
	return main
}

func (d enMessages) PrepareHistoryMessage(main string, u User, newState State, msgID int) string {
	return main
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
