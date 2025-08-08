package bote

import (
	"testing"
	"time"
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
