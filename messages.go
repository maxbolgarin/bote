package bote

import (
	"fmt"
	"strings"

	"github.com/maxbolgarin/lang"
)

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

// Builder is a wrapper for strings.Builder with additional methods.
// You should not copy it. Empty
// Empty value of Builder is ready to use.
type Builder struct {
	strings.Builder
}

// NewBuilder creates a new Builder instance.
func NewBuilder() *Builder {
	return &Builder{}
}

// Writef writes a formatted string to the builder using fmt.Sprintf.
func (b *Builder) Writef(format string, args ...any) {
	b.WriteString(fmt.Sprintf(format, args...))
}

// Writeln writes a string to the builder and adds a newline at the end.
func (b *Builder) Writeln(s string) {
	b.WriteString(s + "\n")
}

// WriteIf writes either msgIf or msgElse depending on the value of first argument.
func (b *Builder) WriteIf(toWrite bool, msgIf, msgElse string) {
	if toWrite {
		b.WriteString(msgIf)
	} else {
		b.WriteString(msgElse)
	}
}

// IsEmpty returns true if the builder's length is 0.
func (b *Builder) IsEmpty() bool {
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
