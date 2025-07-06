package bote

import (
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/maxbolgarin/errm"
	"github.com/maxbolgarin/lang"
	tele "gopkg.in/telebot.v4"
)

const (
	// MaxTextLenInLogs is the maximum length of the text in message logs.
	MaxTextLenInLogs = 64

	startCommand = "/start"
)

// EmptyHandler is a handler that does nothing.
var EmptyHandler = func(Context) error { return nil }

type (
	// HandlerFunc represents a function that is used to handle user actions in bot.
	HandlerFunc func(Context) error

	// MiddlewareFunc represents a function that called on every bot update.
	MiddlewareFunc func(*tele.Update, User) bool

	// Logger is an interface for logging messages.
	Logger interface {
		Debug(string, ...any)
		Info(string, ...any)
		Warn(string, ...any)
		Error(string, ...any)
	}

	// UpdateLogger is an interface for logging updates.
	UpdateLogger interface {
		Log(UpdateType, ...any)
	}

	// Options contains bote additional options.
	Options struct {
		// Config contains bote configuration. It is optional and has default values for all fields.
		// You also can set values using environment variables.
		Config Config

		// UserDB is a storage for users. It uses in-memory storage by default.
		// You should implement it in your application if you want to persist users between applicataion restarts.
		UserDB UsersStorage

		// Msgs is a message provider. It uses default messages by default.
		// You should implement it in your application if you want to use custom messages.
		Msgs MessageProvider

		// Logger is a logger. It uses slog logger by default if EnableLogging == true (by default).
		Logger Logger

		// UpdateLogger is a logger for updates. It uses Logger and logs in debug level by default.
		// It will log updates even if Debug==false. It will log updates even if EnableLogging == false.
		// You should set LogUpdates == false to disable updates logging.
		UpdateLogger UpdateLogger

		// Poller is a poller for the bot. It uses default poller by default.
		// You should implement it in your application if you want to use custom poller (e.g. for testing).
		Poller tele.Poller
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

	// UserCacheCapacity is the capacity of the user cache. Cache will evict users with least activity.
	// Default: 10000.
	// Environment variable: BOTE_USER_CACHE_CAPACITY.
	UserCacheCapacity int `yaml:"user_cache_capacity" json:"user_cache_capacity" env:"BOTE_USER_CACHE_CAPACITY"`

	// UserCacheTTL is the TTL of the user cache.
	// Default: 24 hours.
	// Environment variable: BOTE_USER_CACHE_TTL.
	UserCacheTTL time.Duration `yaml:"user_cache_ttl" json:"user_cache_ttl" env:"BOTE_USER_CACHE_TTL"`

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

	// TestMode is a flag that enables test mode. It set log level to debug and bot to offline.
	// Default: false.
	// You can use environment variable BOTE_TEST_MODE.
	TestMode bool `yaml:"test_mode" json:"test_mode" env:"BOTE_TEST_MODE"`

	// WebhookURL is the URL for the webhook.
	// Environment variable: BOTE_WEBHOOK_URL.
	WebhookURL string `yaml:"webhook_url" json:"webhook_url" env:"BOTE_WEBHOOK_URL"`

	// ListenAddress is the address for the webhook listener.
	// Default: ":8443".
	// Environment variable: BOTE_LISTEN_ADDRESS.
	ListenAddress string `yaml:"listen_address" json:"listen_address" env:"BOTE_LISTEN_ADDRESS"`

	// TLSKeyFile is the path to the TLS key file.
	// Environment variable: BOTE_TLS_KEY_FILE.
	TLSKeyFile string `yaml:"tls_key_file" json:"tls_key_file" env:"BOTE_TLS_KEY_FILE"`

	// TLSCertFile is the path to the TLS cert file.
	// Environment variable: BOTE_TLS_CERT_FILE.
	TLSCertFile string `yaml:"tls_cert_file" json:"tls_cert_file" env:"BOTE_TLS_CERT_FILE"`
}

// WithConfig returns an option that sets the bot configuration.
func WithConfig(cfg Config) func(opts *Options) {
	return func(opts *Options) {
		opts.Config = cfg
	}
}

// WithUserDB returns an option that sets the user storage.
func WithUserDB(db UsersStorage) func(opts *Options) {
	return func(opts *Options) {
		opts.UserDB = db
	}
}

// WithMsgs returns an option that sets the message provider.
func WithMsgs(msgs MessageProvider) func(opts *Options) {
	return func(opts *Options) {
		opts.Msgs = msgs
	}
}

// WithLogger returns an option that sets the logger.
func WithLogger(logger Logger) func(opts *Options) {
	return func(opts *Options) {
		opts.Logger = logger
	}
}

// WithUpdateLogger returns an option that sets the update logger.
func WithUpdateLogger(logger UpdateLogger) func(opts *Options) {
	return func(opts *Options) {
		opts.UpdateLogger = logger
	}
}

// WithTestMode returns an option that sets the test mode.
// If poller is provided, it will be used instead of the default poller.
func WithTestMode(poller ...tele.Poller) func(opts *Options) {
	return func(opts *Options) {
		if len(poller) > 0 {
			opts.Poller = poller[0]
		}
		opts.Config.TestMode = true
	}
}

func (cfg *Config) prepareAndValidate() error {
	if err := env.Parse(cfg); err != nil {
		return err
	}

	cfg.ParseMode = lang.Check(cfg.ParseMode, tele.ModeHTML)
	cfg.LPTimeout = lang.Check(cfg.LPTimeout, 15*time.Second)
	cfg.DefaultLanguageCode = lang.Check(cfg.DefaultLanguageCode, "en")
	cfg.DeleteMessages = lang.Ptr(lang.CheckPtr(cfg.DeleteMessages, true))
	cfg.LogUpdates = lang.Ptr(lang.CheckPtr(cfg.LogUpdates, true))
	cfg.EnableLogging = lang.Ptr(lang.CheckPtr(cfg.EnableLogging, true))
	cfg.Debug = lang.Check(cfg.Debug, cfg.TestMode)
	cfg.UserCacheCapacity = lang.Check(cfg.UserCacheCapacity, 10000)
	cfg.UserCacheTTL = lang.Check(cfg.UserCacheTTL, 24*time.Hour)

	if cfg.WebhookURL != "" {
		if _, err := url.ParseRequestURI(cfg.WebhookURL); err != nil {
			return errm.Wrap(err, "invalid webhook url")
		}
		cfg.ListenAddress = lang.Check(cfg.ListenAddress, ":8443")
		if (cfg.TLSKeyFile == "") != (cfg.TLSCertFile == "") {
			return errm.New("tls key and cert files must be both provided or both empty")
		}
	}

	return nil
}

func (t UpdateType) String() string {
	return string(t)
}

func prepareOpts(opts Options) (Options, error) {
	err := opts.Config.prepareAndValidate()
	if err != nil {
		return opts, errm.Wrap(err, "prepare and validate config")
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: lang.If(opts.Config.Debug, slog.LevelDebug, slog.LevelInfo),
		}))
		if opts.UpdateLogger == nil && !opts.Config.Debug {
			opts.UpdateLogger = &updateLogger{slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))}
		}
	}
	if opts.UpdateLogger == nil {
		opts.UpdateLogger = &updateLogger{opts.Logger}
	}
	if !*opts.Config.EnableLogging {
		opts.Logger = noopLogger{}
	}

	if opts.UserDB == nil {
		opts.UserDB, err = newInMemoryUserStorage(opts.Config.UserCacheCapacity, opts.Config.UserCacheTTL)
		if err != nil {
			return opts, errm.Wrap(err, "new user storage")
		}
	}
	if opts.Msgs == nil {
		opts.Msgs = newDefaultMessageProvider()
	}

	return opts, nil
}
