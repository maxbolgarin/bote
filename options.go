package bote

import (
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/ilyakaznacheev/cleanenv"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

const (
	// MaxTextLenInLogs is the maximum length of the text in message logs.
	MaxTextLenInLogs = 64
)

type (
	// HandlerFunc represents a function that is used to handle user actions in bot.
	HandlerFunc func(c Context) error

	// MiddlewareFunc represents a function that called on every bot update.
	MiddlewareFunc func(*tele.Update, User) bool

	// Logger is an interface for logging messages.
	Logger interface {
		Debug(msg string, args ...any)
		Info(msg string, args ...any)
		Warn(msg string, args ...any)
		Error(msg string, args ...any)
	}

	// UpdateLogger is an interface for logging updates.
	UpdateLogger interface {
		Log(t UpdateType, args ...any)
	}

	// Options contains bote additional options.
	Options struct {
		// Config contains bote configuration. It is optional and has default values for all fields.
		// You also can set values using environment variables.
		Config Config

		// UserDB is a storage for users. It uses in-memory storage by default.
		// You should implement it in your application if you want to persist users between applicataion restarts.
		UserDB UserStorage

		// Msgs is a message provider. It uses default messages by default.
		// You should implement it in your application if you want to use custom messages.
		Msgs MessageProvider

		// Logger is a logger. It uses slog logger by default if EnableLogging == true (by default).
		Logger Logger

		// UpdateLogger is a logger for updates. It uses Logger and logs in debug level by default.
		// It will log updates even if Debug==false. It will log updates even if EnableLogging == false.
		// You should set LogUpdates == false to disable updates logging.
		UpdateLogger UpdateLogger
	}

	// UpdateType is a type of update that is using in update logging.
	UpdateType string
)

const (
	MessageUpdate  UpdateType = "message"
	CallbackUpdate UpdateType = "callback"
)

// Config contains bote configuration.
type Config struct {
	// LPTimeout is the long polling timeout.
	// Default: 15 seconds.
	// Environment variable: BOTE_LP_TIMEOUT.
	LPTimeout time.Duration `yaml:"lp_timeout" json:"lp_timeout"  env:"BOTE_LP_TIMEOUT"`

	// ParseMode is the default parse mode for the bot.
	// Default: HTML.
	// Environment variable: BOTE_PARSE_MODE.
	// It can be one of the following:
	// - "HTML"
	// - "Markdown"
	// - "MarkdownV2"
	ParseMode tele.ParseMode `yaml:"mode" json:"mode" env:"BOTE_PARSE_MODE"`

	// DefaultLanguageCode is the default language code for the bot in ISO 639-1 format.
	// Default: "en".
	// Environment variable: BOTE_DEFAULT_LANGUAGE_CODE.
	DefaultLanguageCode string `yaml:"default_language_code" json:"default_language_code" env:"BOTE_DEFAULT_LANGUAGE_CODE"`

	// NoPreview is a flag that disables link preview in bot messages.
	// Default: false.
	// Environment variable: BOTE_NO_PREVIEW.
	NoPreview bool `yaml:"no_preview" json:"no_preview" env:"BOTE_NO_PREVIEW"`

	// DeleteMessages is a flag that enables deleting every user message.
	// Default: true.
	// Environment variable: BOTE_DELETE_MESSAGES.
	DeleteMessages *bool `yaml:"delete_messages" json:"delete_messages" env:"BOTE_DELETE_MESSAGES"`

	// LogUpdates is a flag that enables logging of bot updates.
	// Default: true.
	// Environment variable: BOTE_LOG_UPDATES.
	LogUpdates *bool `yaml:"log_updates" json:"log_updates" env:"BOTE_LOG_UPDATES"`

	// EnableLogging is a flag that enables logging of bot activity (except updates logging).
	// Default: true.
	// Environment variable: BOTE_ENABLE_LOGGING.
	EnableLogging *bool `yaml:"enable_logging" json:"enable_logging" env:"BOTE_ENABLE_LOGGING"`

	// Debug is a flag that enables debug mode. It set log level to debug.
	// Default: false.
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
	cfg.DeleteMessages = lang.Check(cfg.DeleteMessages, lang.Ptr(true))
	cfg.LogUpdates = lang.Check(cfg.LogUpdates, lang.Ptr(true))
	cfg.EnableLogging = lang.Check(cfg.EnableLogging, lang.Ptr(true))

	return validation.ValidateStruct(cfg,
		validation.Field(&cfg.LPTimeout, validation.Required, validation.Min(1*time.Second)),
		validation.Field(&cfg.ParseMode, validation.Required),
		validation.Field(&cfg.DefaultLanguageCode, validation.Required, validation.Length(2, 2)),
	)
}

func (t UpdateType) String() string {
	return string(t)
}
