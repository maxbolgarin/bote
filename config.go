package bote

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

// Config contains bote configurations.
//
// You can use environment variables to fill it:
// BOTE_LP_TIMEOUT - long polling timeout
// BOTE_PARSE_MODE - default parse mode
// BOTE_DEFAULT_LANGUAGE_CODE - default language code
// BOTE_NO_PREVIEW - disable link preview
// BOTE_DEBUG - enable debug mode
type Config struct {
	// LPTimeout is the long polling timeout.
	// Default is 15 seconds.
	// You can use environment variable BOTE_LP_TIMEOUT.
	LPTimeout time.Duration `yaml:"lp_timeout" json:"lp_timeout"  env:"BOTE_LP_TIMEOUT"`

	// ParseMode is the default parse mode for the bot.
	// Default is HTML.
	// You can use environment variable BOTE_PARSE_MODE.
	// It can be one of the following:
	// - "HTML"
	// - "Markdown"
	// - "MarkdownV2"
	ParseMode tele.ParseMode `yaml:"mode" json:"mode" env:"BOTE_PARSE_MODE"`

	// DefaultLanguageCode is the default language code for the bot in ISO 639-1 format.
	// Default is "en".
	DefaultLanguageCode string `yaml:"default_language_code" json:"default_language_code" env:"BOTE_DEFAULT_LANGUAGE_CODE"`

	// NoPreview is a flag that disables link preview.
	// You can use environment variable BOTE_NO_PREVIEW.
	NoPreview bool `yaml:"no_preview" json:"no_preview" env:"BOTE_NO_PREVIEW"`

	// Debug is a flag that enables debug mode.
	// You can use environment variable BOTE_DEBUG.
	Debug bool `yaml:"debug" json:"debug" env:"BOTE_DEBUG"`
}

func (cfg *Config) Read(fileName ...string) error {
	if len(fileName) > 0 {
		return cleanenv.ReadConfig(fileName[0], cfg)
	}
	return cleanenv.ReadEnv(cfg)
}

func (cfg *Config) prepareAndValidate() error {
	cfg.ParseMode = lang.Check(cfg.ParseMode, tele.ModeHTML)
	cfg.LPTimeout = lang.Check(cfg.LPTimeout, 15*time.Second)
	cfg.DefaultLanguageCode = lang.Check(cfg.DefaultLanguageCode, "en")

	return validation.ValidateStruct(cfg,
		validation.Field(&cfg.LPTimeout, validation.Required, validation.Min(1*time.Second)),
		validation.Field(&cfg.ParseMode, validation.Required),
		validation.Field(&cfg.DefaultLanguageCode, validation.Required, validation.Length(2, 2)),
	)
}
