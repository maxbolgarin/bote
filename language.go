package bote

import (
	"fmt"
	"strings"
)

type Language string

const (
	LanguageDefault Language = "en"
)

// Language constants based on IETF subtags
const (
	LanguageAfrikaans        Language = "af"
	LanguageAlbanian         Language = "sq"
	LanguageAmharic          Language = "am"
	LanguageArabic           Language = "ar"
	LanguageArmenian         Language = "hy"
	LanguageAssamese         Language = "as"
	LanguageAzerbaijani      Language = "az"
	LanguageBashkir          Language = "ba"
	LanguageBasque           Language = "eu"
	LanguageBelarusian       Language = "be"
	LanguageBengali          Language = "bn"
	LanguageBosnian          Language = "bs"
	LanguageBreton           Language = "br"
	LanguageBulgarian        Language = "bg"
	LanguageBurmese          Language = "my"
	LanguageCatalan          Language = "ca"
	LanguageCentralKurdish   Language = "ckb"
	LanguageChinese          Language = "zh"
	LanguageCorsican         Language = "co"
	LanguageCroatian         Language = "hr"
	LanguageCzech            Language = "cs"
	LanguageDanish           Language = "da"
	LanguageDari             Language = "prs"
	LanguageDivehi           Language = "dv"
	LanguageDutch            Language = "nl"
	LanguageEnglish          Language = "en"
	LanguageEstonian         Language = "et"
	LanguageFaroese          Language = "fo"
	LanguageFilipino         Language = "fil"
	LanguageFinnish          Language = "fi"
	LanguageFrench           Language = "fr"
	LanguageFrisian          Language = "fy"
	LanguageGalician         Language = "gl"
	LanguageGeorgian         Language = "ka"
	LanguageGerman           Language = "de"
	LanguageGilbertese       Language = "gil"
	LanguageGreek            Language = "el"
	LanguageGreenlandic      Language = "kl"
	LanguageGujarati         Language = "gu"
	LanguageHausa            Language = "ha"
	LanguageHebrew           Language = "he"
	LanguageHindi            Language = "hi"
	LanguageHungarian        Language = "hu"
	LanguageIcelandic        Language = "is"
	LanguageIgbo             Language = "ig"
	LanguageIndonesian       Language = "id"
	LanguageInuktitut        Language = "iu"
	LanguageIrish            Language = "ga"
	LanguageItalian          Language = "it"
	LanguageJapanese         Language = "ja"
	LanguageKiche            Language = "quc"
	LanguageKannada          Language = "kn"
	LanguageKazakh           Language = "kk"
	LanguageKhmer            Language = "km"
	LanguageKinyarwanda      Language = "rw"
	LanguageKiswahili        Language = "sw"
	LanguageKonkani          Language = "kok"
	LanguageKorean           Language = "ko"
	LanguageKurdish          Language = "ku"
	LanguageKyrgyz           Language = "ky"
	LanguageLao              Language = "lo"
	LanguageLatvian          Language = "lv"
	LanguageLithuanian       Language = "lt"
	LanguageLowerSorbian     Language = "dsb"
	LanguageLuxembourgish    Language = "lb"
	LanguageMacedonian       Language = "mk"
	LanguageMalay            Language = "ms"
	LanguageMalayalam        Language = "ml"
	LanguageMaltese          Language = "mt"
	LanguageMaori            Language = "mi"
	LanguageMapudungun       Language = "arn"
	LanguageMarathi          Language = "mr"
	LanguageMohawk           Language = "moh"
	LanguageMongolian        Language = "mn"
	LanguageMoroccanArabic   Language = "ary"
	LanguageNepali           Language = "ne"
	LanguageNorwegian        Language = "no"
	LanguageNorwegianBokmal  Language = "nb"
	LanguageNorwegianNynorsk Language = "nn"
	LanguageOccitan          Language = "oc"
	LanguageOdia             Language = "or"
	LanguagePapiamento       Language = "pap"
	LanguagePashto           Language = "ps"
	LanguagePersian          Language = "fa"
	LanguagePolish           Language = "pl"
	LanguagePortuguese       Language = "pt"
	LanguagePunjabi          Language = "pa"
	LanguageQuechua          Language = "qu"
	LanguageRomanian         Language = "ro"
	LanguageRomansh          Language = "rm"
	LanguageRussian          Language = "ru"
	LanguageSamiInari        Language = "smn"
	LanguageSamiLule         Language = "smj"
	LanguageSamiNorthern     Language = "se"
	LanguageSamiSkolt        Language = "sms"
	LanguageSamiSouthern     Language = "sma"
	LanguageSanskrit         Language = "sa"
	LanguageScottishGaelic   Language = "gd"
	LanguageSerbian          Language = "sr"
	LanguageSesotho          Language = "st"
	LanguageSinhala          Language = "si"
	LanguageSlovak           Language = "sk"
	LanguageSlovenian        Language = "sl"
	LanguageSpanish          Language = "es"
	LanguageSwedish          Language = "sv"
	LanguageSwissGerman      Language = "gsw"
	LanguageSyriac           Language = "syc"
	LanguageTajik            Language = "tg"
	LanguageTamazight        Language = "tzm"
	LanguageTamil            Language = "ta"
	LanguageTatar            Language = "tt"
	LanguageTelugu           Language = "te"
	LanguageThai             Language = "th"
	LanguageTibetan          Language = "bo"
	LanguageTswana           Language = "tn"
	LanguageTurkish          Language = "tr"
	LanguageTurkmen          Language = "tk"
	LanguageUkrainian        Language = "uk"
	LanguageUpperSorbian     Language = "hsb"
	LanguageUrdu             Language = "ur"
	LanguageUyghur           Language = "ug"
	LanguageUzbek            Language = "uz"
	LanguageVietnamese       Language = "vi"
	LanguageWelsh            Language = "cy"
	LanguageWolof            Language = "wo"
	LanguageXhosa            Language = "xh"
	LanguageYakut            Language = "sah"
	LanguageYi               Language = "ii"
	LanguageYoruba           Language = "yo"
	LanguageZulu             Language = "zu"
)

// languageMap provides fast lookup for language code to Language constant mapping
var languageMap = map[string]Language{
	"af":  LanguageAfrikaans,
	"sq":  LanguageAlbanian,
	"am":  LanguageAmharic,
	"ar":  LanguageArabic,
	"hy":  LanguageArmenian,
	"as":  LanguageAssamese,
	"az":  LanguageAzerbaijani,
	"ba":  LanguageBashkir,
	"eu":  LanguageBasque,
	"be":  LanguageBelarusian,
	"bn":  LanguageBengali,
	"bs":  LanguageBosnian,
	"br":  LanguageBreton,
	"bg":  LanguageBulgarian,
	"my":  LanguageBurmese,
	"ca":  LanguageCatalan,
	"ckb": LanguageCentralKurdish,
	"zh":  LanguageChinese,
	"co":  LanguageCorsican,
	"hr":  LanguageCroatian,
	"cs":  LanguageCzech,
	"da":  LanguageDanish,
	"prs": LanguageDari,
	"dv":  LanguageDivehi,
	"nl":  LanguageDutch,
	"en":  LanguageEnglish,
	"et":  LanguageEstonian,
	"fo":  LanguageFaroese,
	"fil": LanguageFilipino,
	"fi":  LanguageFinnish,
	"fr":  LanguageFrench,
	"fy":  LanguageFrisian,
	"gl":  LanguageGalician,
	"ka":  LanguageGeorgian,
	"de":  LanguageGerman,
	"gil": LanguageGilbertese,
	"el":  LanguageGreek,
	"kl":  LanguageGreenlandic,
	"gu":  LanguageGujarati,
	"ha":  LanguageHausa,
	"he":  LanguageHebrew,
	"hi":  LanguageHindi,
	"hu":  LanguageHungarian,
	"is":  LanguageIcelandic,
	"ig":  LanguageIgbo,
	"id":  LanguageIndonesian,
	"iu":  LanguageInuktitut,
	"ga":  LanguageIrish,
	"it":  LanguageItalian,
	"ja":  LanguageJapanese,
	"quc": LanguageKiche,
	"kn":  LanguageKannada,
	"kk":  LanguageKazakh,
	"km":  LanguageKhmer,
	"rw":  LanguageKinyarwanda,
	"sw":  LanguageKiswahili,
	"kok": LanguageKonkani,
	"ko":  LanguageKorean,
	"ku":  LanguageKurdish,
	"ky":  LanguageKyrgyz,
	"lo":  LanguageLao,
	"lv":  LanguageLatvian,
	"lt":  LanguageLithuanian,
	"dsb": LanguageLowerSorbian,
	"lb":  LanguageLuxembourgish,
	"mk":  LanguageMacedonian,
	"ms":  LanguageMalay,
	"ml":  LanguageMalayalam,
	"mt":  LanguageMaltese,
	"mi":  LanguageMaori,
	"arn": LanguageMapudungun,
	"mr":  LanguageMarathi,
	"moh": LanguageMohawk,
	"mn":  LanguageMongolian,
	"ary": LanguageMoroccanArabic,
	"ne":  LanguageNepali,
	"no":  LanguageNorwegian,
	"nb":  LanguageNorwegianBokmal,
	"nn":  LanguageNorwegianNynorsk,
	"oc":  LanguageOccitan,
	"or":  LanguageOdia,
	"pap": LanguagePapiamento,
	"ps":  LanguagePashto,
	"fa":  LanguagePersian,
	"pl":  LanguagePolish,
	"pt":  LanguagePortuguese,
	"pa":  LanguagePunjabi,
	"qu":  LanguageQuechua,
	"ro":  LanguageRomanian,
	"rm":  LanguageRomansh,
	"ru":  LanguageRussian,
	"smn": LanguageSamiInari,
	"smj": LanguageSamiLule,
	"se":  LanguageSamiNorthern,
	"sms": LanguageSamiSkolt,
	"sma": LanguageSamiSouthern,
	"sa":  LanguageSanskrit,
	"gd":  LanguageScottishGaelic,
	"sr":  LanguageSerbian,
	"st":  LanguageSesotho,
	"si":  LanguageSinhala,
	"sk":  LanguageSlovak,
	"sl":  LanguageSlovenian,
	"es":  LanguageSpanish,
	"sv":  LanguageSwedish,
	"gsw": LanguageSwissGerman,
	"syc": LanguageSyriac,
	"tg":  LanguageTajik,
	"tzm": LanguageTamazight,
	"ta":  LanguageTamil,
	"tt":  LanguageTatar,
	"te":  LanguageTelugu,
	"th":  LanguageThai,
	"bo":  LanguageTibetan,
	"tn":  LanguageTswana,
	"tr":  LanguageTurkish,
	"tk":  LanguageTurkmen,
	"uk":  LanguageUkrainian,
	"hsb": LanguageUpperSorbian,
	"ur":  LanguageUrdu,
	"ug":  LanguageUyghur,
	"uz":  LanguageUzbek,
	"vi":  LanguageVietnamese,
	"cy":  LanguageWelsh,
	"wo":  LanguageWolof,
	"xh":  LanguageXhosa,
	"sah": LanguageYakut,
	"ii":  LanguageYi,
	"yo":  LanguageYoruba,
	"zu":  LanguageZulu,
}

// String returns the string representation of the language.
func (l Language) String() string {
	return string(l)
}

// ParseLanguage parses a language code string and returns the corresponding [Language] constant.
// It accepts language codes in any case (e.g., "en", "EN", "En").
// Returns an error if the language code is not recognized.
func ParseLanguage(code string) (Language, error) {
	if code == "" {
		return "", fmt.Errorf("language code cannot be empty")
	}
	if len(code) > 3 {
		return "", fmt.Errorf("language code cannot be longer than 3 characters")
	}

	// Normalize to lowercase for case-insensitive lookup
	normalizedCode := strings.ToLower(strings.TrimSpace(code))

	if lang, exists := languageMap[normalizedCode]; exists {
		return lang, nil
	}

	return "", fmt.Errorf("unsupported language code: %q", code)
}

// MustLanguage parses a language code string and returns the corresponding [Language] constant.
// It panics if the language code is not recognized.
// Use this function when you are certain the language code is valid.
func MustLanguage(code string) Language {
	lang, err := ParseLanguage(code)
	if err != nil {
		panic(fmt.Sprintf("MustLanguage: %v", err))
	}
	return lang
}

// ParseLanguageOrDefault parses a language code string and returns the corresponding [Language] constant.
// If the language code is not recognized, it returns the default language.
func ParseLanguageOrDefault(code string) Language {
	lang, err := ParseLanguage(code)
	if err != nil {
		return LanguageDefault
	}
	return lang
}
