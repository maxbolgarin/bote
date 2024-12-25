package bote

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

const (
	maxButtonsInRow = 8
)

// RuneSizeType sets type of UTF-8 runes in button text.
// For example, if you use English language, you should use OneBytePerRune.
// If you use Russian language, you should use TwoBytesPerRune.
// If you add a lot of emojis of special symbols, you should use FourBytesPerRune.
type RuneSizeType string

const (
	OneBytePerRune   RuneSizeType = "OneBytePerRune"
	TwoBytesPerRune  RuneSizeType = "TwoBytesPerRune"
	FourBytesPerRune RuneSizeType = "FourBytesPerRune"
)

var (
	EmptyBtn      tele.Btn
	EmptyKeyboard = Inline(maxButtonsInRow)

	// TODO: make length depends on number of buttons
	runesInRow = map[RuneSizeType]int{
		OneBytePerRune:   36,
		TwoBytesPerRune:  20,
		FourBytesPerRune: 16,
	}
)

// ButtonBuilder is an interface for creating buttons.
// Use this interface to provide [Bot] to handlers without "admin" methods like [Bot.AddMiddleware] or [Bot.Stop].
type ButtonBuilder interface {
	Btn(name string, callback HandlerFunc, dataList ...string) tele.Btn
}

// Btn creates button and registers handler for it. You can provide data for the button.
// Data items will be separated by '|' in a single data string.
// Button unique value is generated from hexing button name with 10 random bytes at the end.
func (b *Bot) Btn(name string, callback HandlerFunc, dataList ...string) tele.Btn {
	btn := tele.Btn{
		Text:   name,
		Unique: getBtnUnique(name),
		Data:   CreateBtnData(dataList...),
	}
	if callback != nil {
		b.Handle(&btn, callback)
	}
	return btn
}

// CreateBtnData creates data string from dataList, that should be passed as data to callback button.
// This method can be useful when creating [InitBundle] with providing [InitBundle.Data].
func CreateBtnData(dataList ...string) string {
	switch len(dataList) {
	case 0:
		return ""
	case 1:
		return dataList[0]
	}

	var b strings.Builder
	b.WriteString(dataList[0])
	for _, s := range dataList[1:] {
		if s == "" {
			continue
		}
		b.WriteString("|" + s)
	}
	return b.String()
}

// Keyboard is a ReplyMarkup (keyboard) builder.
type Keyboard struct {
	buttons    [][]tele.Btn
	currentRow []tele.Btn

	optionalRowLen int

	runesInCurrentRow int
	maxRunesInRow     int
	isCountRunes      bool
}

// NewKeyboard creates new keyboard builder.
func NewKeyboard(optionalRowLen ...int) *Keyboard {
	return &Keyboard{
		buttons:    make([][]tele.Btn, 0),
		currentRow: make([]tele.Btn, 0),

		optionalRowLen: lang.First(optionalRowLen),
	}
}

// NewKeyboardWithLength creates new keyboard builder with max runes in a row.
// It creates a new row in Add if number of runes is greater than max runes in row for selected rune type.
func NewKeyboardWithLength(runeType RuneSizeType, optionalRowLen ...int) *Keyboard {
	return &Keyboard{
		buttons:        make([][]tele.Btn, 0),
		currentRow:     make([]tele.Btn, 0, maxButtonsInRow),
		maxRunesInRow:  runesInRow[runeType],
		isCountRunes:   runesInRow[runeType] > 0,
		optionalRowLen: lang.First(optionalRowLen),
	}
}

// Add adds buttons to the current row.
// It creates a new row in Add if number of buttons is greater than max buttons in row.
// It creates a new row in Add if number of runes is greater than max runes in row for selected rune type.
func (k *Keyboard) Add(btns ...tele.Btn) *Keyboard {
	for _, btn := range btns {
		if len(k.currentRow) == maxButtonsInRow {
			k.StartNewRow()
		}
		if k.optionalRowLen > 0 && len(k.currentRow) == k.optionalRowLen {
			k.StartNewRow()
		}

		// Zero len for new row
		if len(k.currentRow) == 0 {
			k.currentRow = append(k.currentRow, btn)
			if k.isCountRunes {
				k.runesInCurrentRow += utf8.RuneCountInString(btn.Text)
			}
			continue
		}

		if k.isCountRunes {
			runesInBtn := utf8.RuneCountInString(btn.Text)
			if k.runesInCurrentRow+runesInBtn >= k.maxRunesInRow {
				k.StartNewRow()
			}
			k.runesInCurrentRow = runesInBtn
		}

		k.currentRow = append(k.currentRow, btn)
	}

	return k
}

// AddRow adds buttons to the current row.
// It creates a new row if there is buttons in the current row after Add.
func (k *Keyboard) AddRow(btns ...tele.Btn) *Keyboard {
	if len(k.currentRow) > 0 {
		k.StartNewRow()
	}
	k.buttons = append(k.buttons, btns)

	return k
}

// StartNewRow creates a new row.
func (k *Keyboard) StartNewRow() *Keyboard {
	if len(k.currentRow) == 0 {
		return k
	}
	k.buttons = append(k.buttons, k.currentRow)
	k.currentRow = make([]tele.Btn, 0, maxButtonsInRow)
	k.runesInCurrentRow = 0

	return k
}

// CreateInlineMarkup creates inline keyboard from the current keyboard builder.
func (k *Keyboard) CreateInlineMarkup() *tele.ReplyMarkup {
	if len(k.currentRow) > 0 {
		k.StartNewRow()
	}

	out := make([][]tele.InlineButton, 0, len(k.buttons))
	for _, row := range k.buttons {
		rOut := make([]tele.InlineButton, 0, len(row))
		for _, btn := range row {
			rOut = append(rOut, *btn.Inline())
		}
		out = append(out, rOut)
	}

	selector := tele.ReplyMarkup{
		InlineKeyboard: out,
	}

	return &selector
}

// CreateReplyMarkup creates reply keyboard from the current keyboard builder.
func (k *Keyboard) CreateReplyMarkup(oneTime bool) *tele.ReplyMarkup {
	if len(k.currentRow) > 0 {
		k.StartNewRow()
	}

	out := make([][]tele.ReplyButton, 0, len(k.buttons))
	for _, row := range k.buttons {
		rOut := make([]tele.ReplyButton, 0, len(row))
		for _, btn := range row {
			btn.Unique = "" // I should do this thing because of nil coming from Reply() func
			rOut = append(rOut, *btn.Reply())
		}
		out = append(out, rOut)
	}

	selector := tele.ReplyMarkup{
		ResizeKeyboard:  true,
		OneTimeKeyboard: oneTime,
		ReplyKeyboard:   out,
	}

	return &selector
}

// Inline creates inline keyboard from provided rows of buttons.
func Inline(rowLength int, btns ...tele.Btn) *tele.ReplyMarkup {
	keyboard := NewKeyboard(rowLength)
	for _, btn := range btns {
		keyboard.Add(btn)
	}
	return keyboard.CreateInlineMarkup()
}

// InlineBuilder creates inline keyboard from provided buttons and columns count.
func InlineBuilder(columns int, runesTypes RuneSizeType, btns ...tele.Btn) *tele.ReplyMarkup {
	keyboard := NewKeyboardWithLength(runesTypes)
	for i, btn := range btns {
		if i%columns == 0 && i != 0 {
			keyboard.StartNewRow()
		}
		keyboard.Add(btn)
	}
	return keyboard.CreateInlineMarkup()
}

// SingleRow creates inline keyboard from provided buttons with a single row.
func SingleRow(btn ...tele.Btn) *tele.ReplyMarkup {
	keyboard := NewKeyboard()
	keyboard.Add(btn...)
	return keyboard.CreateInlineMarkup()
}

// RemoveKeyboard creates remove keyboard request.
func RemoveKeyboard() *tele.ReplyMarkup {
	selector := tele.ReplyMarkup{
		RemoveKeyboard: true,
	}
	return &selector
}

const (
	// maxBytesInUnique is the maximum number of bytes that can be used in button unique value
	maxBytesInUnique = 38
	// randBytesInUnique is the number of random bytes in unique button value
	randBytesInUnique = 10
	// nameBytesInUnique is the maximum length of name in unique button value
	nameBytesInUnique = maxBytesInUnique - randBytesInUnique
)

func getBtnUnique(name string) string {
	var (
		nameHex = hex.EncodeToString([]byte(name))
		rnd     = abstract.GetRandomString(randBytesInUnique)
	)
	if len(nameHex) > nameBytesInUnique {
		nameHex = nameHex[:nameBytesInUnique]
	}
	return nameHex + rnd
}

func parseBtnUnique(unique string) string {
	notRand := unique[:len(unique)-randBytesInUnique]
	raw, err := hex.DecodeString(notRand)
	if err != nil {
		return unique
	}
	return string(raw)
}
