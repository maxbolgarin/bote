package bote

import (
	"log/slog"
	"net/url"
	"os"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/erro"
	"github.com/maxbolgarin/lang"
	"github.com/prometheus/client_golang/prometheus"

	tele "gopkg.in/telebot.v4"
)

const (
	// MaxTextLenInLogs is the maximum length of the text in message logs.
	MaxTextLenInLogs = 64

	startCommand = "/start"

	defaultLongPollingTimeout = 15 * time.Second

	defaultWebhookListenAddress  = "0.0.0.0:8080"
	defaultWebhookReadTimeout    = 30 * time.Second
	defaultWebhookIdleTimeout    = 120 * time.Second
	defaultWebhookMaxConnections = 40

	defaultWebhookRateLimitEnabled = true
	defaultWebhookRateLimitRPS     = 30
	defaultWebhookRateLimitBurst   = 10

	defaultWebhookHealthPath  = "/health"
	defaultWebhookMetricsPath = "/metrics"

	defaultBotParseMode       = tele.ModeHTML
	defaultBotDefaultLanguage = LanguageDefault
	defaultBotDeleteMessages  = true
	defaultUserCacheCapacity  = 10000
	defaultUserCacheTTL       = 24 * time.Hour

	defaultLogEnable  = true
	defaultLogUpdates = true
	defaultLogLevel   = "info"

	defaultUpdatesChannelCapacity = 1000

	longDurationThreshold = 500 * time.Millisecond
)

// https://core.telegram.org/bots/webhooks
var telegramIPRanges = []string{
	"149.154.160.0/20",
	"91.108.4.0/22",
}

// EmptyHandler is a handler that does nothing.
var EmptyHandler = func(Context) error { return nil }

type (
	// HandlerFunc represents a function that is used to handle user actions in bot.
	HandlerFunc func(Context) error

	// MiddlewareFunc represents a function that called on every bot update.
	MiddlewareFunc func(*tele.Update, User) bool

	// MiddlewareFuncTele represents a function that called on every bot update in telebot format.
	MiddlewareFuncTele func(*tele.Update) bool

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
		// It will log updates even if EnableLogging == false.
		// You should set LogUpdates == false to disable updates logging.
		UpdateLogger UpdateLogger

		// Poller is a poller for the bot. It uses default poller by default.
		// You should implement it in your application if you want to use custom poller (e.g. for testing).
		// Note: If WebhookConfig is provided, this will be ignored and webhook poller will be used.
		Poller tele.Poller

		// Metrics is a configuration for prometheus metrics.
		// It registers metrics in the provided registry.
		// It do not register metris if Registry is nil.
		Metrics MetricsConfig

		// Offline is a flag that enables offline mode.
		// It is used to create a bot without network for testing purposes.
		Offline bool

		metrics *metrics
	}

	MetricsConfig struct {
		Registry    *prometheus.Registry
		Namespace   string
		Subsystem   string
		ConstLabels prometheus.Labels
	}

	// PrivacyMode is a privacy mode for the bot.
	// It is used to make compliance with GDPR (privacy by design).
	// Possible values:
	// - "no" - no privacy mode (all data is stored)
	// - "low" - low privacy mode (UserID + Username)
	// - "strict" - strict mode (only UserID is stored)
	PrivacyMode string

	// UpdateType is a type of update that is using in update logging.
	UpdateType string

	// PollingMode is the polling mode to use.
	PollingMode string
)

const (
	PollingModeLong    PollingMode = "long"
	PollingModeWebhook PollingMode = "webhook"
	PollingModeCustom  PollingMode = "custom"
)

const (
	MessageUpdate  UpdateType = "message"
	CallbackUpdate UpdateType = "callback"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

const (
	PrivacyModeNo     PrivacyMode = "no"
	PrivacyModeLow    PrivacyMode = "low"
	PrivacyModeStrict PrivacyMode = "strict"
)

// Config contains bote configuration.
type Config struct {
	// Mode is the polling mode to use.
	// Default: long.
	// Possible values:
	// - "long" - long polling
	// - "webhook" - webhook
	// - "custom" - custom poller (you should provide poller using [WithPoller] option)
	// Environment variable: BOTE_MODE.
	Mode PollingMode `yaml:"mode" json:"mode" env:"BOTE_MODE"`

	// LongPolling contains long polling configuration.
	LongPolling LongPollingConfig `yaml:"long_polling" json:"long_polling"`

	// Webhook contains webhook configuration.
	Webhook WebhookConfig `yaml:"webhook" json:"webhook"`

	// Bot contains bot configuration.
	Bot BotConfig `yaml:"bot" json:"bot"`

	// Log contains log configuration.
	Log LogConfig `yaml:"log" json:"log"`
}

type BotConfig struct {
	// ParseMode is the default parse mode for the bot.
	// Default: HTML.
	// Environment variable: BOTE_PARSE_MODE.
	// It can be one of the following:
	// - "HTML"
	// - "Markdown"
	// - "MarkdownV2"
	ParseMode tele.ParseMode `yaml:"mode" json:"mode" env:"BOTE_PARSE_MODE"`

	// PrivacyMode is the default privacy mode for the bot.
	// Default: "no".
	// Possible values:
	// - "no" - no privacy mode (all data is stored)
	// - "low" - low privacy mode (UserID + Username)
	// - "strict" - strict mode (only UserID is stored)
	// Environment variable: BOTE_PRIVACY_MODE.
	PrivacyMode PrivacyMode `yaml:"privacy_mode" json:"privacy_mode" env:"BOTE_PRIVACY_MODE"`

	// DefaultLanguage is the default language code for the bot in ISO 639-1 format.
	// Default: "en".
	// Environment variable: BOTE_DEFAULT_LANGUAGE.
	DefaultLanguage Language `yaml:"default_language" json:"default_language" env:"BOTE_DEFAULT_LANGUAGE"`

	// NoPreview is a flag that disables link preview in bot messages.
	// Default: false.
	// Environment variable: BOTE_NO_PREVIEW.
	NoPreview bool `yaml:"no_preview" json:"no_preview" env:"BOTE_NO_PREVIEW"`

	// DeleteMessages is a flag that enables deleting every user message.
	// Default: true.
	// Environment variable: BOTE_DELETE_MESSAGES.
	DeleteMessages *bool `yaml:"delete_messages" json:"delete_messages" env:"BOTE_DELETE_MESSAGES"`

	// UserCacheCapacity is the capacity of the user cache. Cache will evict users with least activity.
	// Default: 10000.
	// Environment variable: BOTE_USER_CACHE_CAPACITY.
	UserCacheCapacity int `yaml:"user_cache_capacity" json:"user_cache_capacity" env:"BOTE_USER_CACHE_CAPACITY"`

	// UserCacheTTL is the TTL of the user cache.
	// Default: 24 hours.
	// Environment variable: BOTE_USER_CACHE_TTL.
	UserCacheTTL time.Duration `yaml:"user_cache_ttl" json:"user_cache_ttl" env:"BOTE_USER_CACHE_TTL"`
}

type LogConfig struct {
	// Enable is a flag that enables logging of bot activity (except updates logging).
	// Use default slog to stderr if another logger is not provided using [WithLogger] option.
	// Default: true.
	// Environment variable: BOTE_ENABLE_LOGGING.
	Enable *bool `yaml:"enable" json:"enable" env:"BOTE_LOG_ENABLE"`

	// LogUpdates is a flag that enables logging of bot updates.
	// Use default slog to stderr if another logger is not provided using [WithUpdateLogger] option.
	// Default: true.
	// Environment variable: BOTE_LOG_UPDATES.
	LogUpdates *bool `yaml:"log_updates" json:"log_updates" env:"BOTE_LOG_UPDATES"`

	// Level is the log level. Logger will log messages with level greater than or equal to this level.
	// Default: info.
	// Possible values:
	// - "debug"
	// - "info"
	// - "warn"
	// - "error"
	// Environment variable: BOTE_LOG_LEVEL.
	Level string `yaml:"level" json:"level" env:"BOTE_LOG_LEVEL"`

	// Privacy is a flag that makes logs more privacy-friendly.
	// When true, it will not log username, messages, pressed buttons, etc. Only IDs and states will be logged.
	// Default: false.
	// Environment variable: BOTE_LOG_PRIVACY.
	Privacy bool `yaml:"privacy" json:"privacy" env:"BOTE_LOG_PRIVACY"`

	// DebugIncomingUpdates is a flag that enables logging of incoming updates.
	// It is not for production use.
	// Default: false.
	// Environment variable: BOTE_LOG_DEBUG_INCOMING_UPDATES.
	DebugIncomingUpdates bool `yaml:"debug_incoming_updates" json:"debug_incoming_updates" env:"BOTE_LOG_DEBUG_INCOMING_UPDATES"`
}

// LongPollingConfig contains configuration for long polling-based bot operation.
type LongPollingConfig struct {
	// Timeout is the long polling timeout.
	// Default: 15 seconds.
	// Environment variable: BOTE_LP_TIMEOUT.
	Timeout time.Duration `yaml:"timeout" json:"timeout"  env:"BOTE_LP_TIMEOUT"`

	// Limit is the maximum number of updates to be returned in a single request.
	// Default: 0, no limit.
	// Environment variable: BOTE_LP_LIMIT.
	Limit int `yaml:"limit" json:"limit" env:"BOTE_LP_LIMIT"`

	// LastUpdateID is the last update ID to be returned.
	// It sets an offset for the starting update to return.
	// It is used to continue long polling from the last update ID.
	// Default: 0.
	// Environment variable: BOTE_LP_LAST_UPDATE_ID.
	LastUpdateID int `yaml:"last_update_id" json:"last_update_id" env:"BOTE_LP_LAST_UPDATE_ID"`

	// AllowedUpdates is a list of update types the bot wants to receive.
	// Empty list means all update types.
	// Possible values:
	//		message
	// 		edited_message
	// 		channel_post
	// 		edited_channel_post
	// 		inline_query
	// 		chosen_inline_result
	// 		callback_query
	// 		shipping_query
	// 		pre_checkout_query
	// 		poll
	// 		poll_answer
	//
	// Environment variable: BOTE_ALLOWED_UPDATES (comma-separated).
	AllowedUpdates []string `yaml:"allowed_updates" json:"allowed_updates" env:"BOTE_ALLOWED_UPDATES" envSeparator:","`
}

// WebhookConfig contains configuration for webhook-based bot operation.
type WebhookConfig struct {
	// URL is the webhook endpoint URL that Telegram will use to send updates.
	// Must be HTTPS and accessible from the internet.
	// Example: "https://yourdomain.com/bot/webhook"
	URL string `yaml:"url" json:"url" env:"BOTE_WEBHOOK_URL"`

	// Listen is the address to bind the webhook server to.
	// Default: ":8443"
	// Environment variable: BOTE_WEBHOOK_LISTEN.
	Listen string `yaml:"listen" json:"listen" env:"BOTE_WEBHOOK_LISTEN"`

	// ReadTimeout is the maximum duration for reading the entire request.
	// Default: 30 seconds.
	// Environment variable: BOTE_WEBHOOK_READ_TIMEOUT.
	ReadTimeout time.Duration `yaml:"read_timeout" json:"read_timeout" env:"BOTE_WEBHOOK_READ_TIMEOUT"`

	// IdleTimeout is the maximum amount of time to wait for the next request.
	// Default: 120 seconds.
	// Environment variable: BOTE_WEBHOOK_IDLE_TIMEOUT.
	IdleTimeout time.Duration `yaml:"idle_timeout" json:"idle_timeout" env:"BOTE_WEBHOOK_IDLE_TIMEOUT"`

	// MaxConnections is the maximum allowed number of simultaneous HTTPS connections to the webhook.
	// Valid range: 1-100. Default: 40.
	// Environment variable: BOTE_WEBHOOK_MAX_CONNECTIONS.
	MaxConnections int `yaml:"max_connections" json:"max_connections" env:"BOTE_WEBHOOK_MAX_CONNECTIONS"`

	// DropPendingUpdates specifies whether to drop pending updates when setting webhook.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_DROP_PENDING_UPDATES.
	DropPendingUpdates bool `yaml:"drop_pending_updates" json:"drop_pending_updates" env:"BOTE_WEBHOOK_DROP_PENDING_UPDATES"`

	// MetricsPath is the path to serve metrics.
	// Default: "/metrics".
	// Environment variable: BOTE_WEBHOOK_METRICS_PATH.
	MetricsPath string `yaml:"metrics_path" json:"metrics_path" env:"BOTE_WEBHOOK_METRICS_PATH"`

	// EnableMetrics is a flag that enables metrics serving.
	// It uses provided registry to register metrics or a default one if registry is nil.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_ENABLE_METRICS.
	EnableMetrics bool `yaml:"enable_metrics" json:"enable_metrics" env:"BOTE_WEBHOOK_ENABLE_METRICS"`

	// Security contains security configuration.
	Security WebhookSecurityConfig `yaml:"security" json:"security"`

	// RateLimit contains rate limiting configuration.
	RateLimit WebhookRateLimitConfig `yaml:"rate_limit" json:"rate_limit"`

	// AllowedUpdates is a list of update types the bot wants to receive.
	// Empty list means all update types.
	// Possible values:
	//		message
	// 		edited_message
	// 		channel_post
	// 		edited_channel_post
	// 		inline_query
	// 		chosen_inline_result
	// 		callback_query
	// 		shipping_query
	// 		pre_checkout_query
	// 		poll
	// 		poll_answer
	//
	// Environment variable: BOTE_ALLOWED_UPDATES (comma-separated).
	AllowedUpdates []string `yaml:"allowed_updates" json:"allowed_updates" env:"BOTE_ALLOWED_UPDATES" envSeparator:","`

	urlParsed *url.URL
}

type WebhookSecurityConfig struct {
	// SecretToken is used for webhook request verification.
	// Highly recommended for security. Will be generated automatically if not provided.
	// Environment variable: BOTE_WEBHOOK_SECRET_TOKEN.
	SecretToken string `yaml:"secret_token" json:"secret_token" env:"BOTE_WEBHOOK_SECRET_TOKEN"`

	// StartHTTPS indicates if using HTTPS in the current server.
	// If true, it will start current server as HTTPS server with provided certificate and key.
	// If false, it will start current server as HTTP server.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_START_HTTPS.
	StartHTTPS bool `yaml:"start_https" json:"start_https" env:"BOTE_WEBHOOK_START_HTTPS"`

	// LoadCertInTelegram indicates that current certificate (public part) should be loaded to Telegram.
	// It is required for Telegram webhook if you use self-signed certificate.
	// If true, will upload certificate to Telegram and providing certificate is required.
	// If false, it is expected that you use trusted certificate.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_LOAD_CERT_IN_TELEGRAM.
	LoadCertInTelegram bool `yaml:"load_cert_in_telegram" json:"load_cert_in_telegram" env:"BOTE_WEBHOOK_LOAD_CERT_IN_TELEGRAM"`

	// CertFile is the path to the TLS certificate file.
	// Required if StartHTTPS is true or LoadCertInTelegram is true.
	// If it is empty it is expected that there is a LB in front of the server that termmintaes TLS.
	// HTTPS is strictly required for Telegram webhook.
	// Environment variable: BOTE_WEBHOOK_CERT_FILE.
	CertFile string `yaml:"cert_file" json:"cert_file" env:"BOTE_WEBHOOK_CERT_FILE"`

	// KeyFile is the path to the TLS private key file.
	// Required if StartHTTPS is true or LoadCertInTelegram is true.
	// If it is empty it is expected that there is a LB in front of the server that termmintaes TLS.
	// HTTPS is strictly required for Telegram webhook.
	// Environment variable: BOTE_WEBHOOK_KEY_FILE.
	KeyFile string `yaml:"key_file" json:"key_file" env:"BOTE_WEBHOOK_KEY_FILE"`

	// CheckTLSInRequest indicates that current server should check TLS in request.
	// If true, it will check TLS in request and return 400 if it is not TLS.
	// It will try to find TLS config in request or https in X-Forwarded-Proto header.
	// If false, it will not check TLS in request.
	// Default: true.
	// Environment variable: BOTE_WEBHOOK_CHECK_TLS_IN_REQUEST.
	CheckTLSInRequest *bool `yaml:"check_tls_in_request" json:"check_tls_in_request" env:"BOTE_WEBHOOK_CHECK_TLS_IN_REQUEST"`

	// SecurityHeaders is a flag that enables security headers in response.
	// Default: true.
	// It will set the following headers:
	// - X-Frame-Options: DENY
	// - X-Content-Type-Options: nosniff
	// - X-XSS-Protection: 1; mode=block
	// - Referrer-Policy: strict-origin-when-cross-origin
	// Environment variable: BOTE_WEBHOOK_SECURITY_HEADERS.
	SecurityHeaders *bool `yaml:"security_headers" json:"security_headers" env:"BOTE_WEBHOOK_SECURITY_HEADERS"`

	// GenerateSelfSignedCert is a flag that enables generation of self-signed certificate and upload it to Telegram.
	// It will generate certificate and key files and upload them to Telegram.
	// It will set StartHTTPS to true and LoadCertInTelegram to true.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_GENERATE_SELF_SIGNED_CERT.
	GenerateSelfSignedCert *bool `yaml:"generate_self_signed_cert" json:"generate_self_signed_cert" env:"BOTE_WEBHOOK_GENERATE_SELF_SIGNED_CERT"`

	// AllowedTelegramIPs is a flag that adds Telegram IPs to allowed IPs.
	// Default: false.
	// Environment variable: BOTE_WEBHOOK_ALLOW_TELEGRAM_IPS.
	AllowTelegramIPs *bool `yaml:"allow_telegram_ips" json:"allow_telegram_ips" env:"BOTE_WEBHOOK_ALLOW_TELEGRAM_IPS"`

	// AllowedIPs contains allowed IP addresses/CIDR blocks.
	// Only requests from these IPs will be accepted.
	// Default: [] to allow all IPs.
	// Environment variable: BOTE_WEBHOOK_ALLOWED_IPS (comma-separated).
	AllowedIPs []string `yaml:"allowed_ips" json:"allowed_ips" env:"BOTE_WEBHOOK_ALLOWED_IPS" envSeparator:","`
}

// WebhookRateLimitConfig contains rate limiting configuration.
type WebhookRateLimitConfig struct {
	// Enabled enables rate limiting. Default: true.
	// Environment variable: BOTE_WEBHOOK_RATE_LIMIT_ENABLED.
	Enabled *bool `yaml:"enabled" json:"enabled" env:"BOTE_WEBHOOK_RATE_LIMIT_ENABLED"`

	// RequestsPerSecond is the maximum requests per second allowed.
	// Default: 30 (Telegram's recommended limit).
	// Environment variable: BOTE_WEBHOOK_RATE_LIMIT_RPS.
	RequestsPerSecond int `yaml:"requests_per_second" json:"requests_per_second" env:"BOTE_WEBHOOK_RATE_LIMIT_RPS"`

	// BurstSize is the burst size for rate limiting.
	// Default: 10.
	// Environment variable: BOTE_WEBHOOK_RATE_LIMIT_BURST.
	BurstSize int `yaml:"burst_size" json:"burst_size" env:"BOTE_WEBHOOK_RATE_LIMIT_BURST"`
}

// WithConfig returns an option that sets the bot configuration.
func WithConfig(cfg Config) func(opts *Options) {
	return func(opts *Options) {
		opts.Config = cfg
	}
}

// WithMode returns an option that sets the polling mode.
func WithMode(mode PollingMode) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Mode = mode
	}
}

func WithMetricsConfig(metrics MetricsConfig) func(opts *Options) {
	return func(opts *Options) {
		opts.Metrics = metrics
	}
}

// WithLongPolling returns an option that sets the long polling configuration.
func WithLongPollingConfig(cfg LongPollingConfig) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.LongPolling = cfg
	}
}

// WithLongPolling returns an option that sets the long polling configuration.
func WithLongPolling(timeout ...time.Duration) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Mode = PollingModeLong
		opts.Config.LongPolling.Timeout = lang.First(timeout)
	}
}

// WithAllowedUpdates returns an option that sets the allowed updates.
func WithAllowedUpdates(updates ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.LongPolling.AllowedUpdates = updates
		opts.Config.Webhook.AllowedUpdates = updates
	}
}

// WithWebhookConfig returns an option that sets the webhook configuration.
func WithWebhookConfig(cfg WebhookConfig) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook = cfg
	}
}

// WithWebhook returns an option that creates a webhook configuration from URL and basic settings.
// This is a convenience function for simple webhook setups.
func WithWebhook(url string, listen ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Mode = PollingModeWebhook
		opts.Config.Webhook.URL = url
		opts.Config.Webhook.Listen = lang.First(listen)
	}
}

// WithWebhookServer returns an option that sets the webhook server listen address.
func WithWebhookServer(listen string, isHTTPS bool) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Listen = listen
		opts.Config.Webhook.Security.StartHTTPS = isHTTPS
	}
}

// WithWebhookRateLimit returns an option that sets the webhook rate limit.
func WithWebhookRateLimit(rps int, burst int) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.RateLimit.Enabled = lang.Ptr(true)
		opts.Config.Webhook.RateLimit.RequestsPerSecond = rps
		opts.Config.Webhook.RateLimit.BurstSize = burst
	}
}

// WithWebhookSecretToken returns an option that sets the webhook secret token.
func WithWebhookSecretToken(secretToken string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Security.SecretToken = secretToken
	}
}

// WithWebhookAllowedIPs returns an option that sets the webhook allowed IPs.
func WithWebhookAllowedIPs(allowedIPs ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Security.AllowedIPs = append(opts.Config.Webhook.Security.AllowedIPs, allowedIPs...)
	}
}

// WithWebhookAllowedTelegramIPs returns an option that sets the webhook allowed Telegram IPs.
func WithWebhookAllowedTelegramIPs() func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Security.AllowedIPs = append(opts.Config.Webhook.Security.AllowedIPs, telegramIPRanges...)
		opts.Config.Webhook.Security.AllowTelegramIPs = lang.Ptr(false)
	}
}

// WithWebhookSecurityHeaders returns an option that sets the webhook security headers.
func WithWebhookSecurityHeaders() func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Security.SecurityHeaders = lang.Ptr(true)
	}
}

// WithWebhookTLS returns an option that creates a webhook configuration with TLS certificates.
func WithWebhookCertificate(certFile, keyFile string, loadCertificateInTelegram bool, startHTTPS bool) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Mode = PollingModeWebhook
		opts.Config.Webhook.Security.CertFile = certFile
		opts.Config.Webhook.Security.KeyFile = keyFile
		opts.Config.Webhook.Security.LoadCertInTelegram = loadCertificateInTelegram
		opts.Config.Webhook.Security.StartHTTPS = startHTTPS
	}
}

// WithWebhookGenerateCertificate returns an option that generates self-signed certificate and uploads it to Telegram.
func WithWebhookGenerateCertificate(directory ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.Security.GenerateSelfSignedCert = lang.Ptr(true)
		opts.Config.Webhook.Security.CertFile = lang.Check(lang.First(directory), ".") + "/cert.pem"
		opts.Config.Webhook.Security.KeyFile = lang.Check(lang.First(directory), ".") + "/key.pem"
	}
}

// WithWebhookGenerateCertificate returns an option that generates self-signed certificate and uploads it to Telegram.
func WithWebhookMetrics(metrics MetricsConfig, metricsPath ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.EnableMetrics = true
		opts.Config.Webhook.MetricsPath = lang.First(metricsPath)
		opts.Metrics = metrics
	}
}

// WithWebhookGenerateCertificate returns an option that generates self-signed certificate and uploads it to Telegram.
func WithWebhookDefaultMetrics(metricsPath ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Webhook.EnableMetrics = true
		opts.Config.Webhook.MetricsPath = lang.First(metricsPath)
		opts.Metrics = MetricsConfig{
			Registry: prometheus.NewRegistry(),
		}
	}
}

// WithCustomPoller returns an option that sets the custom poller.
func WithCustomPoller(poller tele.Poller) func(opts *Options) {
	return func(opts *Options) {
		opts.Poller = poller
		opts.Config.Mode = PollingModeCustom
	}
}

// WithBotConfig returns an option that sets the bot configuration.
func WithBotConfig(cfg BotConfig) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Bot = cfg
	}
}

// WithDefaultLanguage returns an option that sets the default language.
func WithDefaultLanguage(lang Language) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Bot.DefaultLanguage = lang
	}
}

// WithPrivacyMode returns an option that sets the privacy mode.
func WithPrivacyMode(mode PrivacyMode) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Bot.PrivacyMode = mode
	}
}

// WithUserDB returns an option that sets the user storage.
func WithUserDB(db UsersStorage) func(opts *Options) {
	return func(opts *Options) {
		opts.UserDB = db
	}
}

// WithMsgsProvider returns an option that sets the message provider.
func WithMsgsProvider(msgs MessageProvider) func(opts *Options) {
	return func(opts *Options) {
		opts.Msgs = msgs
	}
}

// WithLogger returns an option that sets the logger.
func WithLogger(logger Logger, level ...string) func(opts *Options) {
	return func(opts *Options) {
		opts.Logger = logger
		opts.Config.Log.Enable = lang.Ptr(true)
		opts.Config.Log.Level = lang.First(level)
	}
}

// WithUpdateLogger returns an option that sets the update logger.
func WithUpdateLogger(logger UpdateLogger) func(opts *Options) {
	return func(opts *Options) {
		opts.UpdateLogger = logger
		opts.Config.Log.LogUpdates = lang.Ptr(true)
	}
}

// WithDebug returns an option that sets the debug mode.
func WithLogLevel(level string) func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Log.Level = level
	}
}

// WithDebugIncomingUpdates returns an option that sets the debug incoming updates.
func WithDebugIncomingUpdates() func(opts *Options) {
	return func(opts *Options) {
		opts.Config.Log.DebugIncomingUpdates = true
	}
}

// WithOffline returns an option that sets the offline mode.
// If poller is provided, it will be used instead of the default poller.
// It is used to create a bot without network for testing purposes.
func WithOffline(poller ...tele.Poller) func(opts *Options) {
	return func(opts *Options) {
		if len(poller) > 0 {
			opts.Poller = poller[0]
			opts.Config.Mode = PollingModeCustom
		}
		opts.Offline = true
	}
}

func (cfg *Config) prepareAndValidate() error {
	err := env.Parse(cfg)
	if err != nil {
		return err
	}

	if cfg.Mode == PollingModeWebhook {
		if cfg.Webhook.URL == "" {
			return erro.New("webhook URL is required")
		}
		cfg.Webhook.urlParsed, err = url.Parse(cfg.Webhook.URL)
		if err != nil {
			return erro.Wrap(err, "parse webhook URL")
		}
		if cfg.Webhook.urlParsed.Scheme != "https" {
			return erro.New("webhook URL must use HTTPS")
		}
		generateSelfSignedCert := cfg.Webhook.Security.GenerateSelfSignedCert != nil && *cfg.Webhook.Security.GenerateSelfSignedCert

		if cfg.Webhook.Security.StartHTTPS {
			if cfg.Webhook.Security.CertFile == "" && !generateSelfSignedCert {
				return erro.New("certificate file is required if start HTTPS is true")
			}
			if cfg.Webhook.Security.KeyFile == "" && !generateSelfSignedCert {
				return erro.New("key file is required if start HTTPS is true")
			}
		}
		if cfg.Webhook.Security.LoadCertInTelegram {
			if cfg.Webhook.Security.CertFile == "" && !generateSelfSignedCert {
				return erro.New("certificate file is required if load certificate in Telegram is true")
			}
		}

		if generateSelfSignedCert {
			cfg.Webhook.Security.LoadCertInTelegram = true
			cfg.Webhook.Security.StartHTTPS = true
		}
	}

	cfg.Mode = lang.Check(cfg.Mode, PollingModeLong)
	cfg.LongPolling.Timeout = lang.Check(cfg.LongPolling.Timeout, defaultLongPollingTimeout)

	cfg.Webhook.Listen = lang.Check(cfg.Webhook.Listen, defaultWebhookListenAddress)
	cfg.Webhook.ReadTimeout = lang.Check(cfg.Webhook.ReadTimeout, defaultWebhookReadTimeout)
	cfg.Webhook.IdleTimeout = lang.Check(cfg.Webhook.IdleTimeout, defaultWebhookIdleTimeout)
	if cfg.Webhook.MaxConnections < 1 || cfg.Webhook.MaxConnections > 100 {
		cfg.Webhook.MaxConnections = defaultWebhookMaxConnections
	}

	cfg.Webhook.Security.CheckTLSInRequest = lang.Ptr(lang.CheckPtr(cfg.Webhook.Security.CheckTLSInRequest, true))
	if cfg.Webhook.Security.AllowTelegramIPs != nil && *cfg.Webhook.Security.AllowTelegramIPs {
		cfg.Webhook.Security.AllowedIPs = append(cfg.Webhook.Security.AllowedIPs, telegramIPRanges...)
	}

	if cfg.Webhook.Security.SecretToken == "" {
		cfg.Webhook.Security.SecretToken = abstract.GetRandomString(32)
	}

	cfg.Webhook.RateLimit.Enabled = lang.Ptr(lang.CheckPtr(cfg.Webhook.RateLimit.Enabled, defaultWebhookRateLimitEnabled))
	cfg.Webhook.RateLimit.RequestsPerSecond = lang.Check(cfg.Webhook.RateLimit.RequestsPerSecond, defaultWebhookRateLimitRPS)
	cfg.Webhook.RateLimit.BurstSize = lang.Check(cfg.Webhook.RateLimit.BurstSize, defaultWebhookRateLimitBurst)

	cfg.Webhook.MetricsPath = lang.Check(cfg.Webhook.MetricsPath, defaultWebhookMetricsPath)

	cfg.Bot.ParseMode = lang.Check(cfg.Bot.ParseMode, defaultBotParseMode)
	cfg.Bot.PrivacyMode = lang.Check(cfg.Bot.PrivacyMode, PrivacyModeNo)
	cfg.Bot.DefaultLanguage = lang.Check(cfg.Bot.DefaultLanguage, defaultBotDefaultLanguage)
	cfg.Bot.DeleteMessages = lang.Ptr(lang.CheckPtr(cfg.Bot.DeleteMessages, defaultBotDeleteMessages))
	cfg.Bot.UserCacheCapacity = lang.Check(cfg.Bot.UserCacheCapacity, defaultUserCacheCapacity)
	cfg.Bot.UserCacheTTL = lang.Check(cfg.Bot.UserCacheTTL, defaultUserCacheTTL)

	cfg.Log.Enable = lang.Ptr(lang.CheckPtr(cfg.Log.Enable, defaultLogEnable))
	cfg.Log.LogUpdates = lang.Ptr(lang.CheckPtr(cfg.Log.LogUpdates, defaultLogUpdates))
	cfg.Log.Level = lang.Check(cfg.Log.Level, defaultLogLevel)

	return nil
}

func (t UpdateType) String() string {
	return string(t)
}

func prepareOpts(opts Options) (Options, error) {
	err := opts.Config.prepareAndValidate()
	if err != nil {
		return opts, erro.Wrap(err, "prepare and validate config")
	}
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}
	if opts.UpdateLogger == nil {
		opts.UpdateLogger = &updateLogger{opts.Logger}
	}
	opts.metrics = newMetrics(opts.Metrics)
	opts.Logger = &leveledLogger{
		log:   opts.Logger,
		level: getLogLevel(opts.Config.Log.Level),
	}
	if !*opts.Config.Log.Enable {
		opts.Logger = noopLogger{}
	}
	if !*opts.Config.Log.LogUpdates {
		opts.UpdateLogger = &updateLogger{noopLogger{}}
	}
	if opts.UserDB == nil {
		opts.UserDB, err = newInMemoryUserStorage(opts.Config.Bot.UserCacheCapacity, opts.Config.Bot.UserCacheTTL)
		if err != nil {
			return opts, erro.Wrap(err, "new user storage")
		}
	}
	if opts.Msgs == nil {
		opts.Msgs = newDefaultMessageProvider()
	}

	if opts.Config.Mode == PollingModeLong {
		opts.Poller = &tele.LongPoller{
			Timeout:      opts.Config.LongPolling.Timeout,
			Limit:        opts.Config.LongPolling.Limit,
			LastUpdateID: opts.Config.LongPolling.LastUpdateID,
		}
	}

	if opts.Config.Mode == PollingModeWebhook {
		webhookPoller, err := newWebhookPoller(opts.Config.Webhook, opts.metrics, opts.Logger)
		if err != nil {
			return opts, erro.Wrap(err, "create webhook poller")
		}
		opts.Poller = webhookPoller
	}

	return opts, nil
}

func getLogLevel(level string) slog.Level {
	switch level {
	case LogLevelDebug:
		return slog.LevelDebug
	case LogLevelInfo:
		return slog.LevelInfo
	case LogLevelWarn:
		return slog.LevelWarn
	case LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

type leveledLogger struct {
	log   Logger
	level slog.Level
}

func (l *leveledLogger) Debug(msg string, args ...any) {
	if l.level <= slog.LevelDebug {
		l.log.Debug(msg, args...)
	}
}

func (l *leveledLogger) Info(msg string, args ...any) {
	if l.level <= slog.LevelInfo {
		l.log.Info(msg, args...)
	}
}

func (l *leveledLogger) Warn(msg string, args ...any) {
	if l.level <= slog.LevelWarn {
		l.log.Warn(msg, args...)
	}
}

func (l *leveledLogger) Error(msg string, args ...any) {
	if l.level <= slog.LevelError {
		l.log.Error(msg, args...)
	}
}

func HTML() any {
	return tele.ModeHTML
}

func Markdown() any {
	return tele.ModeMarkdown
}

func MarkdownV2() any {
	return tele.ModeMarkdownV2
}

func Silent() any {
	return tele.Silent
}

func Protected() any {
	return tele.Protected
}

func ForceReply() any {
	return tele.ForceReply
}

func OneTimeKeyboard() any {
	return tele.OneTimeKeyboard
}

func NoPreview() any {
	return tele.NoPreview
}

func AllowWithoutReply() any {
	return tele.AllowWithoutReply
}
