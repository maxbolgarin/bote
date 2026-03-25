package bote

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigPrepareAndValidate_LongPollingDefaults(t *testing.T) {
	cfg := Config{}
	if err := cfg.prepareAndValidate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Mode != PollingModeLong {
		t.Fatalf("expected mode long, got %q", cfg.Mode)
	}
	if cfg.LongPolling.Timeout == 0 {
		t.Fatalf("expected default timeout to be set")
	}
	if cfg.Bot.ParseMode == "" {
		t.Fatalf("expected default parse mode to be set")
	}
	if cfg.Webhook.Listen == "" || cfg.Webhook.ReadTimeout == 0 || cfg.Webhook.IdleTimeout == 0 {
		t.Fatalf("expected webhook defaults to be set")
	}
	if cfg.Webhook.RateLimit.RequestsPerSecond == 0 || cfg.Webhook.RateLimit.BurstSize == 0 || cfg.Webhook.RateLimit.Enabled == nil {
		t.Fatalf("expected webhook ratelimit defaults to be set")
	}
	if cfg.Bot.DeleteMessages == nil || cfg.Log.Enable == nil || cfg.Log.LogUpdates == nil {
		t.Fatalf("expected bool pointer defaults to be set")
	}
}

func TestConfigPrepareAndValidate_WebhookValidation(t *testing.T) {
	cfg := Config{Mode: PollingModeWebhook}
	if err := cfg.prepareAndValidate(); err == nil {
		t.Fatalf("expected error when webhook URL is empty")
	}

	cfg = Config{Mode: PollingModeWebhook, Webhook: WebhookConfig{URL: "http://example.com/path"}}
	if err := cfg.prepareAndValidate(); err == nil {
		t.Fatalf("expected error when webhook URL is not https")
	}

	cfg = Config{Mode: PollingModeWebhook, Webhook: WebhookConfig{URL: "https://example.com/path"}}
	if err := cfg.prepareAndValidate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithOptionsHelpers(t *testing.T) {
	var opts Options
	WithLongPolling(100 * time.Millisecond)(&opts)
	if opts.Config.Mode != PollingModeLong || opts.Config.LongPolling.Timeout != 100*time.Millisecond {
		t.Fatalf("WithLongPolling not applied")
	}
	WithAllowedUpdates("message", "callback")(&opts)
	if len(opts.Config.LongPolling.AllowedUpdates) != 2 || len(opts.Config.Webhook.AllowedUpdates) != 2 {
		t.Fatalf("WithAllowedUpdates not applied to both modes")
	}
	WithWebhook("https://example.com/hook", ":8080")(&opts)
	if opts.Config.Mode != PollingModeWebhook || opts.Config.Webhook.URL != "https://example.com/hook" || opts.Config.Webhook.Listen != ":8080" {
		t.Fatalf("WithWebhook not applied")
	}
	WithWebhookRateLimit(5, 2)(&opts)
	if opts.Config.Webhook.RateLimit.Enabled == nil || *opts.Config.Webhook.RateLimit.Enabled == false {
		t.Fatalf("WithWebhookRateLimit should enable ratelimit")
	}
}

func TestGetLogLevel(t *testing.T) {
	if got := getLogLevel(LogLevelDebug); got.String() == "" {
		t.Fatalf("expected valid log level for debug")
	}
	if got := getLogLevel("unknown"); got.String() == "" { // should fallback to info
		t.Fatalf("expected fallback log level for unknown")
	}
}

func TestSecretTokenGeneration(t *testing.T) {
	cfg := Config{
		Mode:    PollingModeWebhook,
		Webhook: WebhookConfig{URL: "https://example.com/hook"},
	}
	if err := cfg.prepareAndValidate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	token := cfg.Webhook.Security.SecretToken
	if token == "" {
		t.Fatalf("secret token should be generated")
	}
	if len(token) != 32 {
		t.Fatalf("expected 32 hex chars (16 bytes), got %d", len(token))
	}

	// Generate a second token and ensure they're different
	cfg2 := Config{
		Mode:    PollingModeWebhook,
		Webhook: WebhookConfig{URL: "https://example.com/hook"},
	}
	if err := cfg2.prepareAndValidate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg2.Webhook.Security.SecretToken == token {
		t.Fatalf("two generated tokens should differ")
	}
}

func TestStrictPrivacyRequiresUserDB(t *testing.T) {
	opts := Options{
		Config: Config{
			Mode: PollingModeCustom,
			Bot: BotConfig{
				Privacy: PrivacyConfig{
					Mode: PrivacyModeStrict,
				},
			},
		},
		UserDB:  nil,
		Offline: true,
		Poller:  &mockPoller{},
	}

	_, err := prepareOpts(opts)
	if err == nil {
		t.Fatalf("expected error for strict privacy with nil UserDB")
	}
	if !strings.Contains(err.Error(), "strict privacy") {
		t.Fatalf("error should mention strict privacy, got: %v", err)
	}
}

func TestOptionBuilders_Config(t *testing.T) {
	t.Run("WithConfig", func(t *testing.T) {
		var opts Options
		cfg := Config{Mode: PollingModeWebhook}
		WithConfig(cfg)(&opts)
		assert.Equal(t, PollingModeWebhook, opts.Config.Mode)
	})

	t.Run("WithMode", func(t *testing.T) {
		var opts Options
		WithMode(PollingModeWebhook)(&opts)
		assert.Equal(t, PollingModeWebhook, opts.Config.Mode)
	})

	t.Run("WithMetricsConfig", func(t *testing.T) {
		var opts Options
		mc := MetricsConfig{Namespace: "myns", Subsystem: "mysub"}
		WithMetricsConfig(mc)(&opts)
		assert.Equal(t, "myns", opts.Metrics.Namespace)
		assert.Equal(t, "mysub", opts.Metrics.Subsystem)
	})

	t.Run("WithLongPollingConfig", func(t *testing.T) {
		var opts Options
		cfg := LongPollingConfig{Timeout: 5 * time.Second}
		WithLongPollingConfig(cfg)(&opts)
		assert.Equal(t, 5*time.Second, opts.Config.LongPolling.Timeout)
	})

	t.Run("WithWebhookConfig", func(t *testing.T) {
		var opts Options
		cfg := WebhookConfig{URL: "https://example.com/wh"}
		WithWebhookConfig(cfg)(&opts)
		assert.Equal(t, "https://example.com/wh", opts.Config.Webhook.URL)
	})

	t.Run("WithBotConfig", func(t *testing.T) {
		var opts Options
		cfg := BotConfig{DefaultLanguage: "en"}
		WithBotConfig(cfg)(&opts)
		assert.Equal(t, Language("en"), opts.Config.Bot.DefaultLanguage)
	})
}

func TestOptionBuilders_Webhook(t *testing.T) {
	t.Run("WithWebhookServer", func(t *testing.T) {
		var opts Options
		WithWebhookServer(":9090", true)(&opts)
		assert.Equal(t, ":9090", opts.Config.Webhook.Listen)
		assert.True(t, opts.Config.Webhook.Security.StartHTTPS)
	})

	t.Run("WithWebhookServer_noHTTPS", func(t *testing.T) {
		var opts Options
		WithWebhookServer(":9090", false)(&opts)
		assert.Equal(t, ":9090", opts.Config.Webhook.Listen)
		assert.False(t, opts.Config.Webhook.Security.StartHTTPS)
	})

	t.Run("WithWebhookSecretToken", func(t *testing.T) {
		var opts Options
		WithWebhookSecretToken("my-secret")(&opts)
		assert.Equal(t, "my-secret", opts.Config.Webhook.Security.SecretToken)
	})

	t.Run("WithWebhookAllowedIPs", func(t *testing.T) {
		var opts Options
		WithWebhookAllowedIPs("1.2.3.4", "5.6.7.8")(&opts)
		assert.Contains(t, opts.Config.Webhook.Security.AllowedIPs, "1.2.3.4")
		assert.Contains(t, opts.Config.Webhook.Security.AllowedIPs, "5.6.7.8")
	})

	t.Run("WithWebhookAllowedTelegramIPs", func(t *testing.T) {
		var opts Options
		WithWebhookAllowedTelegramIPs()(&opts)
		assert.NotEmpty(t, opts.Config.Webhook.Security.AllowedIPs)
		assert.NotNil(t, opts.Config.Webhook.Security.AllowTelegramIPs)
		assert.False(t, *opts.Config.Webhook.Security.AllowTelegramIPs)
	})

	t.Run("WithWebhookSecurityHeaders", func(t *testing.T) {
		var opts Options
		WithWebhookSecurityHeaders()(&opts)
		assert.NotNil(t, opts.Config.Webhook.Security.SecurityHeaders)
		assert.True(t, *opts.Config.Webhook.Security.SecurityHeaders)
	})

	t.Run("WithWebhookCertificate", func(t *testing.T) {
		var opts Options
		WithWebhookCertificate("/path/cert.pem", "/path/key.pem", true, true)(&opts)
		assert.Equal(t, PollingModeWebhook, opts.Config.Mode)
		assert.Equal(t, "/path/cert.pem", opts.Config.Webhook.Security.CertFile)
		assert.Equal(t, "/path/key.pem", opts.Config.Webhook.Security.KeyFile)
		assert.True(t, opts.Config.Webhook.Security.LoadCertInTelegram)
		assert.True(t, opts.Config.Webhook.Security.StartHTTPS)
	})

	t.Run("WithWebhookGenerateCertificate_noDir", func(t *testing.T) {
		var opts Options
		WithWebhookGenerateCertificate()(&opts)
		assert.NotNil(t, opts.Config.Webhook.Security.GenerateSelfSignedCert)
		assert.True(t, *opts.Config.Webhook.Security.GenerateSelfSignedCert)
		assert.Contains(t, opts.Config.Webhook.Security.CertFile, "cert.pem")
		assert.Contains(t, opts.Config.Webhook.Security.KeyFile, "key.pem")
	})

	t.Run("WithWebhookGenerateCertificate_withDir", func(t *testing.T) {
		var opts Options
		WithWebhookGenerateCertificate("/tmp/tls")(&opts)
		assert.Equal(t, "/tmp/tls/cert.pem", opts.Config.Webhook.Security.CertFile)
		assert.Equal(t, "/tmp/tls/key.pem", opts.Config.Webhook.Security.KeyFile)
	})

	t.Run("WithWebhookMetrics", func(t *testing.T) {
		var opts Options
		mc := MetricsConfig{Namespace: "webhookns"}
		WithWebhookMetrics(mc, "/mymetrics")(&opts)
		assert.True(t, opts.Config.Webhook.EnableMetrics)
		assert.Equal(t, "/mymetrics", opts.Config.Webhook.MetricsPath)
		assert.Equal(t, "webhookns", opts.Metrics.Namespace)
	})

	t.Run("WithWebhookMetrics_noPath", func(t *testing.T) {
		var opts Options
		mc := MetricsConfig{Subsystem: "sub"}
		WithWebhookMetrics(mc)(&opts)
		assert.True(t, opts.Config.Webhook.EnableMetrics)
		assert.Equal(t, "", opts.Config.Webhook.MetricsPath) // empty before prepareAndValidate
	})
}

func TestOptionBuilders_Privacy(t *testing.T) {
	t.Run("WithLowPrivacyMode", func(t *testing.T) {
		var opts Options
		WithLowPrivacyMode()(&opts)
		assert.Equal(t, PrivacyModeLow, opts.Config.Bot.Privacy.Mode)
	})

	t.Run("WithStrictPrivacyMode_nilPointers", func(t *testing.T) {
		var opts Options
		WithStrictPrivacyMode(nil, nil, nil, nil)(&opts)
		assert.Equal(t, PrivacyModeStrict, opts.Config.Bot.Privacy.Mode)
		assert.Nil(t, opts.Config.Bot.Privacy.EncryptionKey)
		assert.Nil(t, opts.Config.Bot.Privacy.HMACKey)
	})

	t.Run("WithStrictPrivacyMode_withValues", func(t *testing.T) {
		var opts Options
		encKey := "enc-key"
		hmacKey := "hmac-key"
		var encVer int64 = 1
		var hmacVer int64 = 2
		WithStrictPrivacyMode(&encKey, &encVer, &hmacKey, &hmacVer)(&opts)
		assert.Equal(t, PrivacyModeStrict, opts.Config.Bot.Privacy.Mode)
		assert.Equal(t, &encKey, opts.Config.Bot.Privacy.EncryptionKey)
		assert.Equal(t, &hmacKey, opts.Config.Bot.Privacy.HMACKey)
		assert.Equal(t, &encVer, opts.Config.Bot.Privacy.EncryptionKeyVersion)
		assert.Equal(t, &hmacVer, opts.Config.Bot.Privacy.HMACKeyVersion)
	})

	t.Run("WithStrictPrivacyModeKeyProvider", func(t *testing.T) {
		var opts Options
		provider := &simpleKeysProvider{}
		WithStrictPrivacyModeKeyProvider(provider)(&opts)
		assert.Equal(t, PrivacyModeStrict, opts.Config.Bot.Privacy.Mode)
		assert.Equal(t, provider, opts.KeysProvider)
	})
}

func TestOptionBuilders_BotSettings(t *testing.T) {
	t.Run("WithDefaultLanguage", func(t *testing.T) {
		var opts Options
		WithDefaultLanguage("ru")(&opts)
		assert.Equal(t, Language("ru"), opts.Config.Bot.DefaultLanguage)
	})

	t.Run("WithCustomPoller", func(t *testing.T) {
		var opts Options
		poller := &mockPoller{}
		WithCustomPoller(poller)(&opts)
		assert.Equal(t, PollingModeCustom, opts.Config.Mode)
		assert.Equal(t, poller, opts.Poller)
	})

	t.Run("WithUserDB", func(t *testing.T) {
		var opts Options
		// nil is a valid assignment — just verifies the field is set
		WithUserDB(nil)(&opts)
		assert.Nil(t, opts.UserDB)
	})

	t.Run("WithMsgsProvider", func(t *testing.T) {
		var opts Options
		WithMsgsProvider(nil)(&opts)
		assert.Nil(t, opts.Msgs)
	})
}

func TestOptionBuilders_Logging(t *testing.T) {
	t.Run("WithLogLevel", func(t *testing.T) {
		var opts Options
		WithLogLevel(LogLevelDebug)(&opts)
		assert.Equal(t, LogLevelDebug, opts.Config.Log.Level)
	})

	t.Run("WithLogLevel_warn", func(t *testing.T) {
		var opts Options
		WithLogLevel(LogLevelWarn)(&opts)
		assert.Equal(t, LogLevelWarn, opts.Config.Log.Level)
	})

	t.Run("WithLogLevel_error", func(t *testing.T) {
		var opts Options
		WithLogLevel(LogLevelError)(&opts)
		assert.Equal(t, LogLevelError, opts.Config.Log.Level)
	})

	t.Run("WithDebugIncomingUpdates", func(t *testing.T) {
		var opts Options
		WithDebugIncomingUpdates()(&opts)
		assert.True(t, opts.Config.Log.DebugIncomingUpdates)
	})

	t.Run("WithLogger", func(t *testing.T) {
		var opts Options
		WithLogger(nil, LogLevelInfo)(&opts)
		assert.NotNil(t, opts.Config.Log.Enable)
		assert.True(t, *opts.Config.Log.Enable)
		assert.Equal(t, LogLevelInfo, opts.Config.Log.Level)
	})

	t.Run("WithUpdateLogger", func(t *testing.T) {
		var opts Options
		WithUpdateLogger(nil)(&opts)
		assert.NotNil(t, opts.Config.Log.LogUpdates)
		assert.True(t, *opts.Config.Log.LogUpdates)
	})
}

func TestOptionBuilders_Offline(t *testing.T) {
	t.Run("WithOffline_noPoller", func(t *testing.T) {
		var opts Options
		WithOffline()(&opts)
		assert.True(t, opts.Offline)
		assert.Nil(t, opts.Poller)
	})

	t.Run("WithOffline_withPoller", func(t *testing.T) {
		var opts Options
		poller := &mockPoller{}
		WithOffline(poller)(&opts)
		assert.True(t, opts.Offline)
		assert.Equal(t, PollingModeCustom, opts.Config.Mode)
		assert.Equal(t, poller, opts.Poller)
	})
}

func TestOptionBuilders_SendHelpers(t *testing.T) {
	assert.NotNil(t, HTML())
	assert.NotNil(t, Markdown())
	assert.NotNil(t, MarkdownV2())
	assert.NotNil(t, Silent())
	assert.NotNil(t, Protected())
	assert.NotNil(t, ForceReply())
	assert.NotNil(t, OneTimeKeyboard())
	assert.NotNil(t, NoPreview())
	assert.NotNil(t, AllowWithoutReply())
}
