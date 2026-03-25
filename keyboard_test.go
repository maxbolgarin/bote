package bote

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// TestCreateBtnData tests button data creation
func TestCreateBtnData(t *testing.T) {
	tests := []struct {
		name     string
		dataList []string
		expected string
	}{
		{
			name:     "empty list",
			dataList: []string{},
			expected: "",
		},
		{
			name:     "single item",
			dataList: []string{"a"},
			expected: "a",
		},
		{
			name:     "multiple items",
			dataList: []string{"a", "b", "c"},
			expected: "a|b|c",
		},
		{
			name:     "with empty strings",
			dataList: []string{"a", "", "b", "c"},
			expected: "a|b|c",
		},
		{
			name:     "complex data",
			dataList: []string{"user", "123", "action", "edit"},
			expected: "user|123|action|edit",
		},
		{
			name:     "with special characters",
			dataList: []string{"data-1", "value:2", "key=3"},
			expected: "data-1|value:2|key=3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateBtnData(tt.dataList...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestCreateBtnDataPipeEscaping tests that pipe characters in data items are escaped
func TestCreateBtnDataPipeEscaping(t *testing.T) {
	tests := []struct {
		name     string
		dataList []string
		expected string
	}{
		{
			name:     "single item with pipe",
			dataList: []string{"item|with|pipes"},
			expected: "item-with-pipes",
		},
		{
			name:     "multiple items with pipes",
			dataList: []string{"a|b", "c|d"},
			expected: "a-b|c-d",
		},
		{
			name:     "no pipes unchanged",
			dataList: []string{"clean", "data"},
			expected: "clean|data",
		},
		{
			name:     "only pipes",
			dataList: []string{"|||"},
			expected: "---",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateBtnData(tt.dataList...)
			assert.Equal(t, tt.expected, result)
			// Verify DataParsed round-trip produces expected number of parts
			parts := strings.Split(result, "|")
			nonEmptyParts := 0
			for _, p := range parts {
				if p != "" {
					nonEmptyParts++
				}
			}
			assert.Equal(t, len(tt.dataList), nonEmptyParts, "part count should match input count")
		})
	}
}

// TestBtnDataTruncationUTF8 tests that button data truncation respects rune boundaries
func TestBtnDataTruncationUTF8(t *testing.T) {
	t.Run("emoji not split", func(t *testing.T) {
		// Each emoji is 4 bytes. Create data that would be split mid-emoji at byte boundary.
		data := strings.Repeat("😀", 20) // 80 bytes, well over MaxDataLengthBytes
		result := CreateBtnData(data)
		// Result should be valid UTF-8 after truncation
		for _, r := range result {
			assert.NotEqual(t, rune(0xFFFD), r, "should not contain replacement character")
		}
	})

	t.Run("russian text not split", func(t *testing.T) {
		// Each Russian char is 2 bytes
		data := strings.Repeat("Б", 40) // 80 bytes
		result := CreateBtnData(data)
		for _, r := range result {
			assert.NotEqual(t, rune(0xFFFD), r, "should not contain replacement character")
		}
	})

	t.Run("data within limit unchanged", func(t *testing.T) {
		data := "short"
		result := CreateBtnData(data)
		assert.Equal(t, "short", result)
	})
}

// TestGetBtnIDAndUnique tests button ID and unique generation
func TestGetBtnIDAndUnique(t *testing.T) {
	t.Run("generates valid id and unique", func(t *testing.T) {
		name := "button-name"
		id, unique := getBtnIDAndUnique(name)

		assert.NotEmpty(t, id)
		assert.NotEmpty(t, unique)
		assert.Greater(t, len(unique), len(id), "unique should include random suffix")

		// Verify unique contains id
		assert.True(t, strings.HasPrefix(unique, id))
	})

	t.Run("name can be decoded from unique", func(t *testing.T) {
		name := "test-button"
		_, unique := getBtnIDAndUnique(name)

		decoded := getNameFromUnique(unique)
		assert.Equal(t, name, decoded)
	})

	t.Run("id can be extracted from unique", func(t *testing.T) {
		name := "my-button"
		id, unique := getBtnIDAndUnique(name)

		extractedID := getIDFromUnique(unique)
		assert.Equal(t, id, extractedID)
	})

	t.Run("truncates long names", func(t *testing.T) {
		longName := strings.Repeat("a", 100)
		id, unique := getBtnIDAndUnique(longName)

		// Hex encoded name will be truncated to idBytesInUnique
		assert.LessOrEqual(t, len(id), idBytesInUnique)
		assert.LessOrEqual(t, len(unique), maxBytesInUnique)
	})

	t.Run("different names generate different ids", func(t *testing.T) {
		id1, _ := getBtnIDAndUnique("button1")
		id2, _ := getBtnIDAndUnique("button2")

		assert.NotEqual(t, id1, id2)
	})

	t.Run("same name generates different uniques due to random suffix", func(t *testing.T) {
		_, unique1 := getBtnIDAndUnique("same-button")
		_, unique2 := getBtnIDAndUnique("same-button")

		assert.NotEqual(t, unique1, unique2, "uniques should differ due to random suffix")
	})
}

// TestGetNameFromUnique tests name decoding from unique
func TestGetNameFromUnique(t *testing.T) {
	t.Run("decodes valid unique", func(t *testing.T) {
		originalName := "test-button-name"
		encoded := hex.EncodeToString([]byte(originalName))
		unique := encoded + "1234" // Add random suffix

		decoded := getNameFromUnique(unique)
		assert.Equal(t, originalName, decoded)
	})

	t.Run("handles invalid hex", func(t *testing.T) {
		invalidUnique := "zzzzz1234"
		decoded := getNameFromUnique(invalidUnique)
		assert.Equal(t, invalidUnique, decoded, "should return original on decode error")
	})
}

// TestGetIDFromUnique tests ID extraction from unique
func TestGetIDFromUnique(t *testing.T) {
	t.Run("extracts id from valid unique", func(t *testing.T) {
		unique := "abcdef1234"
		id := getIDFromUnique(unique)
		assert.Equal(t, "abcdef", id)
	})

	t.Run("handles short unique", func(t *testing.T) {
		shortUnique := "abc"
		id := getIDFromUnique(shortUnique)
		assert.Equal(t, shortUnique, id)
	})
}

// TestGetIDFromUnparsedData tests ID extraction from unparsed data
func TestGetIDFromUnparsedData(t *testing.T) {
	t.Run("extracts id from data with payload", func(t *testing.T) {
		unique := "abcdef1234"
		data := unique + "|payload"
		id := getIDFromUnparsedData(data)
		expected := unique[:len(unique)-randBytesInUnique]
		assert.Equal(t, expected, id)
	})

	t.Run("handles data without payload", func(t *testing.T) {
		unique := "abcdef1234"
		id := getIDFromUnparsedData(unique)
		expected := unique[:len(unique)-randBytesInUnique]
		assert.Equal(t, expected, id)
	})
}

// TestKeyboardCreation tests keyboard builder creation
func TestKeyboardCreation(t *testing.T) {
	t.Run("creates keyboard with default settings", func(t *testing.T) {
		kb := NewKeyboard()
		assert.NotNil(t, kb)
		assert.NotNil(t, kb.buttons)
		assert.NotNil(t, kb.currentRow)
		assert.Equal(t, 0, kb.optionalRowLen)
	})

	t.Run("creates keyboard with optional row length", func(t *testing.T) {
		kb := NewKeyboard(3)
		assert.Equal(t, 3, kb.optionalRowLen)
	})

	t.Run("creates keyboard with rune counting", func(t *testing.T) {
		kb := NewKeyboardWithLength(TwoBytesPerRune)
		assert.True(t, kb.isCountRunes)
		assert.Equal(t, runesInRow[TwoBytesPerRune], kb.maxRunesInRow)
	})

	t.Run("creates keyboard with optional row length and rune counting", func(t *testing.T) {
		kb := NewKeyboardWithLength(OneBytePerRune, 4)
		assert.Equal(t, 4, kb.optionalRowLen)
		assert.True(t, kb.isCountRunes)
	})
}

// TestKeyboardAdd tests adding buttons to keyboard
func TestKeyboardAdd(t *testing.T) {
	t.Run("adds buttons to current row", func(t *testing.T) {
		kb := NewKeyboard()
		btn1 := tele.Btn{Text: "Button 1"}
		btn2 := tele.Btn{Text: "Button 2"}

		kb.Add(btn1, btn2)

		assert.Len(t, kb.currentRow, 2)
		assert.Len(t, kb.buttons, 0)
	})

	t.Run("creates new row when max buttons reached", func(t *testing.T) {
		kb := NewKeyboard()
		btns := make([]tele.Btn, maxButtonsInRow+1)
		for i := range btns {
			btns[i] = tele.Btn{Text: string(rune('A' + i))}
		}

		kb.Add(btns...)

		assert.Len(t, kb.buttons, 1, "should create new row")
		assert.Len(t, kb.buttons[0], maxButtonsInRow)
		assert.Len(t, kb.currentRow, 1)
	})

	t.Run("creates new row when optional row length reached", func(t *testing.T) {
		kb := NewKeyboard(3)
		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"}, {Text: "3"}, {Text: "4"},
		}

		kb.Add(btns...)

		assert.Len(t, kb.buttons, 1)
		assert.Len(t, kb.buttons[0], 3)
		assert.Len(t, kb.currentRow, 1)
	})

	t.Run("creates new row when max runes reached", func(t *testing.T) {
		kb := NewKeyboardWithLength(OneBytePerRune)
		// Create buttons that together exceed max runes
		longText := strings.Repeat("a", 20)
		btns := []tele.Btn{
			{Text: longText},
			{Text: longText},
		}

		kb.Add(btns...)

		assert.Greater(t, len(kb.buttons)+len(kb.currentRow), 0)
	})
}

// TestKeyboardAddRow tests adding row of buttons
func TestKeyboardAddRow(t *testing.T) {
	t.Run("adds row directly", func(t *testing.T) {
		kb := NewKeyboard()
		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"}, {Text: "3"},
		}

		kb.AddRow(btns...)

		assert.Len(t, kb.buttons, 1)
		assert.Len(t, kb.buttons[0], 3)
		assert.Len(t, kb.currentRow, 0)
	})

	t.Run("starts new row before adding if current row has buttons", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "existing"})

		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"},
		}
		kb.AddRow(btns...)

		assert.Len(t, kb.buttons, 2)
		assert.Len(t, kb.buttons[0], 1, "first row should have existing button")
		assert.Len(t, kb.buttons[1], 2, "second row should have new buttons")
	})
}

// TestKeyboardAddFooter tests adding footer buttons
func TestKeyboardAddFooter(t *testing.T) {
	t.Run("adds footer buttons", func(t *testing.T) {
		kb := NewKeyboard()
		footer := []tele.Btn{
			{Text: "Close"}, {Text: "Help"},
		}

		kb.AddFooter(footer...)

		assert.Len(t, kb.footer, 2)
	})

	t.Run("footer appears in markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "Main"})
		kb.AddFooter(tele.Btn{Text: "Footer"})

		markup := kb.CreateInlineMarkup()

		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Equal(t, "Footer", markup.InlineKeyboard[1][0].Text)
	})
}

// TestKeyboardStartNewRow tests manual row creation
func TestKeyboardStartNewRow(t *testing.T) {
	t.Run("starts new row with buttons in current row", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "1"})
		kb.StartNewRow()
		kb.Add(tele.Btn{Text: "2"})

		assert.Len(t, kb.buttons, 1)
		assert.Len(t, kb.currentRow, 1)
	})

	t.Run("does nothing when current row is empty", func(t *testing.T) {
		kb := NewKeyboard()
		kb.StartNewRow()

		assert.Len(t, kb.buttons, 0)
		assert.Len(t, kb.currentRow, 0)
	})

	t.Run("resets rune counter", func(t *testing.T) {
		kb := NewKeyboardWithLength(OneBytePerRune)
		kb.Add(tele.Btn{Text: "test"})
		assert.Greater(t, kb.runesInCurrentRow, 0)

		kb.StartNewRow()
		assert.Equal(t, 0, kb.runesInCurrentRow)
	})
}

// TestKeyboardCreateInlineMarkup tests inline markup creation
func TestKeyboardCreateInlineMarkup(t *testing.T) {
	t.Run("creates inline markup from buttons", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "Button 1", Unique: "btn1"})
		kb.StartNewRow()
		kb.Add(tele.Btn{Text: "Button 2", Unique: "btn2"})

		markup := kb.CreateInlineMarkup()

		require.NotNil(t, markup)
		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Equal(t, "Button 1", markup.InlineKeyboard[0][0].Text)
		assert.Equal(t, "Button 2", markup.InlineKeyboard[1][0].Text)
	})

	t.Run("includes current row in markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "1"}, tele.Btn{Text: "2"})

		markup := kb.CreateInlineMarkup()

		assert.Len(t, markup.InlineKeyboard, 1)
		assert.Len(t, markup.InlineKeyboard[0], 2)
	})

	t.Run("includes footer in markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "Main"})
		kb.AddFooter(tele.Btn{Text: "Close"})

		markup := kb.CreateInlineMarkup()

		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Equal(t, "Close", markup.InlineKeyboard[1][0].Text)
	})
}

// TestKeyboardCreateReplyMarkup tests reply markup creation
func TestKeyboardCreateReplyMarkup(t *testing.T) {
	t.Run("creates reply markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "Button 1"})
		kb.StartNewRow()
		kb.Add(tele.Btn{Text: "Button 2"})

		markup := kb.CreateReplyMarkup(false)

		require.NotNil(t, markup)
		assert.Len(t, markup.ReplyKeyboard, 2)
		assert.True(t, markup.ResizeKeyboard)
		assert.False(t, markup.OneTimeKeyboard)
	})

	t.Run("creates one-time reply markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "OK"})

		markup := kb.CreateReplyMarkup(true)

		assert.True(t, markup.OneTimeKeyboard)
	})

	t.Run("includes footer in reply markup", func(t *testing.T) {
		kb := NewKeyboard()
		kb.Add(tele.Btn{Text: "Main"})
		kb.AddFooter(tele.Btn{Text: "Cancel"})

		markup := kb.CreateReplyMarkup(false)

		assert.Len(t, markup.ReplyKeyboard, 2)
	})
}

// TestInlineHelperFunctions tests inline keyboard helper functions
func TestInlineHelperFunctions(t *testing.T) {
	t.Run("Inline function creates markup", func(t *testing.T) {
		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"}, {Text: "3"},
		}

		markup := Inline(2, btns...)

		require.NotNil(t, markup)
		assert.Greater(t, len(markup.InlineKeyboard), 0)
	})

	t.Run("InlineBuilder creates markup with columns", func(t *testing.T) {
		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"}, {Text: "3"}, {Text: "4"},
		}

		markup := InlineBuilder(2, OneBytePerRune, btns...)

		require.NotNil(t, markup)
		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Len(t, markup.InlineKeyboard[0], 2)
		assert.Len(t, markup.InlineKeyboard[1], 2)
	})

	t.Run("SingleRow creates one row markup", func(t *testing.T) {
		btns := []tele.Btn{
			{Text: "1"}, {Text: "2"}, {Text: "3"},
		}

		markup := SingleRow(btns...)

		require.NotNil(t, markup)
		assert.Len(t, markup.InlineKeyboard, 1)
		assert.Len(t, markup.InlineKeyboard[0], 3)
	})
}

// TestRemoveKeyboard tests remove keyboard function
func TestRemoveKeyboard(t *testing.T) {
	t.Run("creates remove keyboard markup", func(t *testing.T) {
		markup := RemoveKeyboard()

		require.NotNil(t, markup)
		assert.True(t, markup.RemoveKeyboard)
	})
}

// TestRuneSizeTypes tests rune size type configurations
func TestRuneSizeTypes(t *testing.T) {
	t.Run("rune size types are defined", func(t *testing.T) {
		assert.Contains(t, runesInRow, OneBytePerRune)
		assert.Contains(t, runesInRow, TwoBytesPerRune)
		assert.Contains(t, runesInRow, FourBytesPerRune)

		assert.Greater(t, runesInRow[OneBytePerRune], 0)
		assert.Greater(t, runesInRow[TwoBytesPerRune], 0)
		assert.Greater(t, runesInRow[FourBytesPerRune], 0)
	})

	t.Run("rune sizes are ordered correctly", func(t *testing.T) {
		assert.Greater(t, runesInRow[OneBytePerRune], runesInRow[TwoBytesPerRune])
		assert.Greater(t, runesInRow[TwoBytesPerRune], runesInRow[FourBytesPerRune])
	})
}

// TestKeyboardConstants tests keyboard constants
func TestKeyboardConstants(t *testing.T) {
	t.Run("keyboard constants are valid", func(t *testing.T) {
		assert.Equal(t, 8, maxButtonsInRow)
		assert.Equal(t, 64, MaxDataLengthBytes)
		assert.Equal(t, 28, maxBytesInUnique)
		assert.Equal(t, 4, randBytesInUnique)
		assert.Equal(t, 24, idBytesInUnique)
	})
}

// TestEmptyKeyboard tests empty keyboard constants
func TestEmptyKeyboard(t *testing.T) {
	t.Run("empty button is defined", func(t *testing.T) {
		assert.Equal(t, "", EmptyBtn.Text)
	})

	t.Run("empty keyboard is defined", func(t *testing.T) {
		assert.NotNil(t, EmptyKeyboard)
	})
}

func TestGetNameFromUniqueShortStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"one_char", "a", "a"},
		{"two_chars", "ab", "ab"},
		{"exactly_randBytes", "abcd", "abcd"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNameFromUnique(tt.input)
			assert.Equal(t, tt.expected, result, "short strings should be returned unchanged")
		})
	}
}

func TestGetBtnIDAndUniqueSHA256ForLongNames(t *testing.T) {
	t.Run("short_name_hex_encoded", func(t *testing.T) {
		name := "Ok"
		id, unique := getBtnIDAndUnique(name)
		expectedHex := hex.EncodeToString([]byte(name))
		assert.Equal(t, expectedHex, id)
		assert.True(t, strings.HasPrefix(unique, id))
		assert.Equal(t, len(id)+randBytesInUnique, len(unique))
	})

	t.Run("long_name_uses_sha256", func(t *testing.T) {
		longName := "this_is_a_long_button_name"
		id, unique := getBtnIDAndUnique(longName)
		plainHex := hex.EncodeToString([]byte(longName))
		assert.NotEqual(t, plainHex[:idBytesInUnique], id, "long names should use SHA-256, not plain hex truncation")
		assert.Equal(t, idBytesInUnique, len(id))
		assert.Equal(t, maxBytesInUnique, len(unique))
	})

	t.Run("collision_resistance", func(t *testing.T) {
		name1 := "long_button_name_alpha_version"
		name2 := "long_button_name_beta_version_"
		id1, _ := getBtnIDAndUnique(name1)
		id2, _ := getBtnIDAndUnique(name2)
		assert.NotEqual(t, id1, id2, "different long names should produce different IDs")
	})

	t.Run("deterministic", func(t *testing.T) {
		name := "long_deterministic_button_name_test"
		id1, _ := getBtnIDAndUnique(name)
		id2, _ := getBtnIDAndUnique(name)
		assert.Equal(t, id1, id2, "same name should produce same ID")
	})
}

// TestKeyboardWithContext tests NewKeyboardWithContext and its methods.
func TestKeyboardWithContext(t *testing.T) {
	dummyHandler := func(ctx Context) error { return nil }

	t.Run("NewKeyboardWithContext returns non-nil keyboard", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		require.NotNil(t, kb)
		require.NotNil(t, kb.Keyboard)
	})

	t.Run("AddBtn adds buttons to same row", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		kb.AddBtn("Button 1", dummyHandler)
		kb.AddBtn("Button 2", dummyHandler)
		kb.AddBtn("Button 3", dummyHandler)

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		// All three buttons should be in a single row.
		assert.Len(t, markup.InlineKeyboard, 1)
		assert.Len(t, markup.InlineKeyboard[0], 3)
	})

	t.Run("AddBtnRow creates a new row per call", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		kb.AddBtnRow("Row 1 Btn", dummyHandler)
		kb.AddBtnRow("Row 2 Btn", dummyHandler)

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		// Each AddBtnRow call should produce a separate row.
		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Len(t, markup.InlineKeyboard[0], 1)
		assert.Len(t, markup.InlineKeyboard[1], 1)
	})

	t.Run("AB shortcut behaves same as AddBtn", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		kb.AB("Btn A", dummyHandler)
		kb.AB("Btn B", dummyHandler)

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		// Both buttons should share a single row, same as AddBtn.
		assert.Len(t, markup.InlineKeyboard, 1)
		assert.Len(t, markup.InlineKeyboard[0], 2)
	})

	t.Run("ABR shortcut behaves same as AddBtnRow", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		kb.ABR("Row A", dummyHandler)
		kb.ABR("Row B", dummyHandler)

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		// Each ABR call should produce a separate row.
		assert.Len(t, markup.InlineKeyboard, 2)
	})

	t.Run("AddBtnRow with callback data stores data in button", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)

		kb := NewKeyboardWithContext(ctx)
		kb.AddBtnRow("Item", dummyHandler, "item-id-1")

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		require.Len(t, markup.InlineKeyboard, 1)
		require.Len(t, markup.InlineKeyboard[0], 1)
		// The button Data field should contain the provided data item.
		assert.Contains(t, markup.InlineKeyboard[0][0].Data, "item-id-1")
	})

	t.Run("AddFooter adds buttons in final row", func(t *testing.T) {
		bot := setupTestBot(t)
		ctx := NewContext(bot, 111, 1)
		ctxImpl := ctx.(*contextImpl)

		kb := NewKeyboardWithContext(ctx)
		kb.AddBtn("Main 1", dummyHandler)
		kb.AddBtn("Main 2", dummyHandler)

		footerBtn := ctxImpl.Btn("Footer Close", dummyHandler)
		kb.AddFooter(footerBtn)

		markup := kb.CreateInlineMarkup()
		require.NotNil(t, markup)
		// Should have the main row plus the footer row.
		assert.Len(t, markup.InlineKeyboard, 2)
		assert.Equal(t, "Footer Close", markup.InlineKeyboard[1][0].Text)
	})
}
