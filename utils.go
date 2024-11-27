package bote

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/hex"
	"strings"

	"github.com/maxbolgarin/errm"
	"golang.org/x/exp/utf8string"
	tele "gopkg.in/telebot.v4"
)

const (
	MaxBytes      = 32
	BytesInLetter = 2

	// MaxBytesInUnique is the maximum number of bytes that can be used in button unique value
	MaxBytesInUnique = 38

	// RandBytesInUnique is the number of random bytes in unique button value
	RandBytesInUnique = 12

	// NameBytesInUnique is the maximum length of name in unique button value
	NameBytesInUnique = MaxBytesInUnique - RandBytesInUnique
)

func ParseCallbackData(data string) []string {
	return strings.Split(data, "|")
}

func ParseDoubleCallbackData(data string) (string, string) {
	spl := strings.Split(data, "|")
	if len(spl) < 2 {
		return spl[0], ""
	}
	return spl[0], spl[1]
}

func ParseLastItemFromData(data string) string {
	spl := strings.Split(data, "|")
	return spl[len(spl)-1]
}

func RemoveKeyboard() *tele.ReplyMarkup {
	selector := tele.ReplyMarkup{
		RemoveKeyboard: true,
	}
	return &selector
}

func GetBtnUnique(name string) string {
	var (
		nameHex = hex.EncodeToString([]byte(name))
		rnd     = GetRandID(MaxBytesInUnique)
	)
	if len(nameHex) < NameBytesInUnique {
		nameHex = nameHex[:NameBytesInUnique]
	}
	return nameHex + rnd
}

func ParseBtnUnique(unique string) string {
	notRand := unique[:len(unique)-RandBytesInUnique]
	raw, err := hex.DecodeString(notRand)
	if err != nil {
		return unique
	}
	return string(raw)
}

func GetRandID(length int) string {
	var result string
	secret := make([]byte, length)
	gen, err := rand.Read(secret)
	if err != nil || gen != length {
		// error reading random, return empty string
		return result
	}
	var encoder = base32.StdEncoding.WithPadding(base32.NoPadding)
	result = encoder.EncodeToString(secret)
	if len(result) > length {
		return result[:length]
	}
	return result
}

// PrepareNumber returns first numeric symbols from string, so for `332fdqa` -> `332`.
func PrepareNumber(s string, isDecimal bool) string {
	for i := range s {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if isDecimal && s[i] == '.' {
			continue
		}
		return s[:i]
	}
	return s
}

func IsNotFoundEditMsgErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "message to edit not found")
}

func IsInvalidArgument(err error) bool {
	if err == nil {
		return false
	}
	return errm.Is(err, EmptyUserIDErr) || errm.Is(err, EmptyMsgIDErr)
}

func IsBlockedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "bot was blocked by the user")
}

func CutForButton(s string) string {
	// 3 is for emoji = space; 3 for dots
	if len(s) <= MaxBytes-3 {
		return s
	}
	numberOfLetters := (MaxBytes - 6) / BytesInLetter
	return utfSlice(s, numberOfLetters)
}

func utfSlice(s string, i int) string {
	ss := utf8string.NewString(s)
	rc := ss.RuneCount()
	if i > ss.RuneCount() {
		i = rc
	}
	return ss.Slice(0, i)
}

func isNumeric(s string, ignore ...rune) bool {
loop:
	for _, v := range s {
		for _, a := range ignore {
			if v == a {
				continue loop
			}
		}
		if v < '0' || v > '9' {
			return false
		}
	}
	return true
}

func ignoreError(err error, toIgnore ...error) error {
	for _, e := range toIgnore {
		if errm.Is(err, e) {
			return nil
		}
	}
	return nil
}
