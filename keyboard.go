package bote

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"github.com/maxbolgarin/abstract"
	tele "gopkg.in/telebot.v4"
)

var (
	EmptyBtn      tele.Btn
	EmptyKeyboard = Inline(1)
)

func (b *Bote) NewBtn(callback HandlerFunc, name string, dataList ...string) tele.Btn {
	btn := tele.Btn{
		Text:   name,
		Unique: getBtnUnique(name),
		Data:   createBtnData(dataList...),
	}
	b.Handle(&btn, callback)
	return btn
}

type Keyboard struct {
	buttons    [][]tele.Btn
	currentRow []tele.Btn
	rowLength  int

	maxLength        int
	maxCoulumnLength int
	currentLength    int
}

func NewKeyboard(columns int) *Keyboard {
	if columns < 1 {
		panic("number of columns of the keyboard cannot be less than 1")
	}

	return &Keyboard{
		buttons:    make([][]tele.Btn, 0),
		currentRow: make([]tele.Btn, 0, columns),
		rowLength:  columns,
	}
}

func NewKeyboardWithLength(columns, maxLength int) *Keyboard {
	if columns < 1 {
		panic("number of columns of the keyboard cannot be less than 1")
	}

	return &Keyboard{
		buttons:          make([][]tele.Btn, 0),
		currentRow:       make([]tele.Btn, 0, columns),
		rowLength:        columns,
		maxLength:        maxLength,
		maxCoulumnLength: maxLength / columns,
	}
}

func (k *Keyboard) AppendRow(row ...tele.Btn) {
	if len(k.currentRow) > 0 {
		k.StartNewRow()
	}
	k.buttons = append(k.buttons, row)
}

func (k *Keyboard) Append(btns ...tele.Btn) {
	for _, btn := range btns {
		if k.maxLength > 0 {
			runes := utf8.RuneCountInString(btn.Text)
			lengthWithButton := k.currentLength + runes
			if len(k.currentRow) > 0 && (lengthWithButton > k.maxLength ||
				k.currentLength > k.maxCoulumnLength || runes > k.maxCoulumnLength) {
				k.StartNewRow()
			}
			k.currentLength += runes
		}
		k.currentRow = append(k.currentRow, btn)
		if len(k.currentRow) == k.rowLength {
			k.StartNewRow()
		}
	}
}

func (k *Keyboard) StartNewRow() {
	if len(k.currentRow) == 0 {
		return
	}
	k.buttons = append(k.buttons, k.currentRow)
	k.currentRow = make([]tele.Btn, 0, k.rowLength)
	if k.maxLength > 0 {
		k.currentLength = 0
	}
}

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

func Inline(columns int, btns ...tele.Btn) *tele.ReplyMarkup {
	keyboard := NewKeyboard(columns)
	if len(btns) > 0 {
		keyboard.Append(btns...)
	}
	return keyboard.CreateInlineMarkup()
}

func Single(btn tele.Btn) *tele.ReplyMarkup {
	keyboard := NewKeyboard(1)
	keyboard.Append(btn)
	return keyboard.CreateInlineMarkup()
}

func InlineLines(lines ...[]tele.Btn) *tele.ReplyMarkup {
	if len(lines) == 0 {
		return nil
	}

	keyboard := NewKeyboard(len(lines[0]))
	for _, line := range lines {
		keyboard.AppendRow(line...)
	}

	return keyboard.CreateInlineMarkup()
}

func RemoveKeyboard() *tele.ReplyMarkup {
	selector := tele.ReplyMarkup{
		RemoveKeyboard: true,
	}
	return &selector
}

func createBtnData(dataList ...string) string {
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
		rnd     = abstract.GetRandomString(maxBytesInUnique)
	)
	if len(nameHex) < nameBytesInUnique {
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
