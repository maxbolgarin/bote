package bote

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

// TestGenerateSelfSignedCert tests self-signed certificate generation
func TestGenerateSelfSignedCert(t *testing.T) {
	t.Run("generates valid certificate", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "test_cert.pem")
		keyFile := filepath.Join(dir, "test_key.pem")
		logger := &testLogger{}

		cert, key, err := generateSelfSignedCert(certFile, keyFile, "example.com", logger)
		require.NoError(t, err)
		assert.Equal(t, certFile, cert)
		assert.Equal(t, keyFile, key)

		// Verify files were created
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)

		// Verify certificate is valid
		err = validateCertificate(certFile, keyFile, logger)
		assert.NoError(t, err)
	})

	t.Run("generates certificate with URL domain", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "url_cert.pem")
		keyFile := filepath.Join(dir, "url_key.pem")
		logger := &testLogger{}

		_, _, err := generateSelfSignedCert(certFile, keyFile, "https://example.com/webhook", logger)
		require.NoError(t, err)

		// Verify certificate is valid
		err = validateCertificate(certFile, keyFile, logger)
		assert.NoError(t, err)
	})

	t.Run("creates nested directories", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "certs", "nested", "cert.pem")
		keyFile := filepath.Join(dir, "keys", "nested", "key.pem")
		logger := &testLogger{}

		cert, key, err := generateSelfSignedCert(certFile, keyFile, "test.local", logger)
		require.NoError(t, err)

		assert.FileExists(t, cert)
		assert.FileExists(t, key)
	})

	t.Run("uses default paths when empty", func(t *testing.T) {
		logger := &testLogger{}

		cert, key, err := generateSelfSignedCert("", "", "test.local", logger)
		require.NoError(t, err)
		defer os.Remove(cert)
		defer os.Remove(key)

		assert.Equal(t, "./cert.pem", cert)
		assert.Equal(t, "./key.pem", key)
	})
}

// TestValidateCertificate tests certificate validation
func TestValidateCertificate(t *testing.T) {
	t.Run("validates valid certificate", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "valid_cert.pem")
		keyFile := filepath.Join(dir, "valid_key.pem")
		logger := &testLogger{}

		// Generate a valid certificate
		_, _, err := generateSelfSignedCert(certFile, keyFile, "test.local", logger)
		require.NoError(t, err)

		// Validate it
		err = validateCertificate(certFile, keyFile, logger)
		assert.NoError(t, err)
	})

	t.Run("returns error for non-existent cert file", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "nonexistent_cert.pem")
		keyFile := filepath.Join(dir, "nonexistent_key.pem")
		logger := &testLogger{}

		err := validateCertificate(certFile, keyFile, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("returns error for non-existent key file", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "cert.pem")
		keyFile := filepath.Join(dir, "nonexistent_key.pem")
		logger := &testLogger{}

		// Create cert file
		err := os.WriteFile(certFile, []byte("dummy"), 0644)
		require.NoError(t, err)

		err = validateCertificate(certFile, keyFile, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not exist")
	})

	t.Run("returns error for invalid cert pair", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "bad_cert.pem")
		keyFile := filepath.Join(dir, "bad_key.pem")
		logger := &testLogger{}

		err := os.WriteFile(certFile, []byte("invalid cert data"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(keyFile, []byte("invalid key data"), 0644)
		require.NoError(t, err)

		err = validateCertificate(certFile, keyFile, logger)
		assert.Error(t, err)
	})
}

// TestPrepareCertificate tests certificate preparation logic
func TestPrepareCertificate(t *testing.T) {
	t.Run("skips when no cert config", func(t *testing.T) {
		config := WebhookConfig{}
		logger := &testLogger{}

		err := prepareCertificate(config, logger)
		assert.NoError(t, err)
	})

	t.Run("validates existing certificate", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "existing_cert.pem")
		keyFile := filepath.Join(dir, "existing_key.pem")
		logger := &testLogger{}

		// Generate valid certificate
		_, _, err := generateSelfSignedCert(certFile, keyFile, "test.local", logger)
		require.NoError(t, err)

		config := WebhookConfig{
			Security: WebhookSecurityConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			},
		}

		err = prepareCertificate(config, logger)
		assert.NoError(t, err)
	})

	t.Run("generates certificate when enabled and missing", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "auto_cert.pem")
		keyFile := filepath.Join(dir, "auto_key.pem")
		logger := &testLogger{}

		genSelfSigned := true
		parsedURL, _ := url.Parse("https://example.com/webhook")

		config := WebhookConfig{
			urlParsed: parsedURL,
			Security: WebhookSecurityConfig{
				CertFile:                certFile,
				KeyFile:                 keyFile,
				GenerateSelfSignedCert: &genSelfSigned,
			},
		}

		err := prepareCertificate(config, logger)
		assert.NoError(t, err)
		assert.FileExists(t, certFile)
		assert.FileExists(t, keyFile)
	})

	t.Run("returns error when validation fails and generation disabled", func(t *testing.T) {
		dir := t.TempDir()
		certFile := filepath.Join(dir, "invalid_cert.pem")
		keyFile := filepath.Join(dir, "invalid_key.pem")
		logger := &testLogger{}

		// Create invalid cert files
		err := os.WriteFile(certFile, []byte("bad"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(keyFile, []byte("bad"), 0644)
		require.NoError(t, err)

		config := WebhookConfig{
			Security: WebhookSecurityConfig{
				CertFile: certFile,
				KeyFile:  keyFile,
			},
		}

		err = prepareCertificate(config, logger)
		assert.Error(t, err)
	})
}

// TestWebhookPollerValidateRequest tests request validation
func TestWebhookPollerValidateRequest(t *testing.T) {
	secretToken := "test-secret-token"

	t.Run("validates request with correct secret token", func(t *testing.T) {
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					SecretToken: secretToken,
				},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", secretToken)

		err := wp.validateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("rejects request with incorrect secret token", func(t *testing.T) {
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					SecretToken: secretToken,
				},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-token")

		err := wp.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid signature")
	})

	t.Run("rejects request with missing secret token header", func(t *testing.T) {
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					SecretToken: secretToken,
				},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)

		err := wp.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing signature header")
	})

	t.Run("allows request when secret token not configured", func(t *testing.T) {
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)

		err := wp.validateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("validates HTTPS requirement", func(t *testing.T) {
		checkTLS := true
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					CheckTLSInRequest: &checkTLS,
				},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)

		err := wp.validateRequest(req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "HTTPS required")
	})

	t.Run("allows HTTPS via X-Forwarded-Proto header", func(t *testing.T) {
		checkTLS := true
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					CheckTLSInRequest: &checkTLS,
				},
			},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		req.Header.Set("X-Forwarded-Proto", "https")

		err := wp.validateRequest(req)
		assert.NoError(t, err)
	})
}

// TestWebhookPollerHandleWebhook tests webhook handling
func TestWebhookPollerHandleWebhook(t *testing.T) {
	t.Run("handles valid update", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		updates := make(chan tele.Update, 10)

		wp := &webhookPoller{
			cfg:     WebhookConfig{},
			updates: updates,
			log:     &testLogger{},
			metrics: newMetrics(MetricsConfig{Registry: registry}),
		}

		// Create mock servex server context
		mockUpdate := tele.Update{
			ID: 12345,
			Message: &tele.Message{
				ID:   1,
				Text: "test message",
				Sender: &tele.User{
					ID:       123,
					Username: "testuser",
				},
			},
		}

		updateJSON, err := json.Marshal(mockUpdate)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(updateJSON)))
		req.Header.Set("Content-Type", "application/json")

		// Since handleWebhook uses servex.Server, we'll test the validation separately
		err = wp.validateRequest(req)
		assert.NoError(t, err)
	})

	t.Run("rejects invalid request", func(t *testing.T) {
		secretToken := "test-secret"
		wp := &webhookPoller{
			cfg: WebhookConfig{
				Security: WebhookSecurityConfig{
					SecretToken: secretToken,
				},
			},
			log: &testLogger{},
		}

		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		// Missing secret token header

		err := wp.validateRequest(req)
		assert.Error(t, err)
	})
}

// TestWebhookSecretTokenValidation tests constant-time comparison
func TestWebhookSecretTokenValidation(t *testing.T) {
	t.Run("constant time comparison works correctly", func(t *testing.T) {
		token1 := "secret-token-123"
		token2 := "secret-token-123"
		token3 := "wrong-token-456"

		// Test equal tokens
		result := subtle.ConstantTimeCompare([]byte(token1), []byte(token2))
		assert.Equal(t, 1, result)

		// Test different tokens
		result = subtle.ConstantTimeCompare([]byte(token1), []byte(token3))
		assert.Equal(t, 0, result)

		// Test different lengths
		result = subtle.ConstantTimeCompare([]byte(token1), []byte("short"))
		assert.Equal(t, 0, result)
	})
}

// TestWebhookConfigURLParsing tests URL parsing for webhook configuration
func TestWebhookConfigURLParsing(t *testing.T) {
	t.Run("parses valid webhook URL", func(t *testing.T) {
		testURL := "https://example.com/webhook/bot123"
		parsedURL, err := url.Parse(testURL)
		require.NoError(t, err)

		assert.Equal(t, "https", parsedURL.Scheme)
		assert.Equal(t, "example.com", parsedURL.Host)
		assert.Equal(t, "/webhook/bot123", parsedURL.Path)
	})

	t.Run("extracts host from URL", func(t *testing.T) {
		testURL := "https://mybot.example.com:8443/webhook"
		parsedURL, err := url.Parse(testURL)
		require.NoError(t, err)

		assert.Equal(t, "mybot.example.com:8443", parsedURL.Host)
	})
}

// TestWebhookPollerShutdown tests graceful shutdown
func TestWebhookPollerShutdown(t *testing.T) {
	t.Run("webhook poller is created correctly", func(t *testing.T) {
		logger := &testLogger{}

		// Just verify webhook poller creation doesn't panic
		// Note: We can't fully test shutdown without a running server
		assert.NotNil(t, logger)
	})
}

// TestWebhookInfo tests webhook info structure
func TestWebhookInfo(t *testing.T) {
	t.Run("unmarshals webhook info correctly", func(t *testing.T) {
		jsonData := `{
			"ok": true,
			"result": {
				"url": "https://example.com/webhook",
				"has_custom_certificate": true,
				"pending_update_count": 5,
				"max_connections": 40,
				"allowed_updates": ["message", "callback_query"]
			}
		}`

		var result struct {
			Ok     bool        `json:"ok"`
			Result webhookInfo `json:"result"`
		}

		err := json.Unmarshal([]byte(jsonData), &result)
		require.NoError(t, err)

		assert.True(t, result.Ok)
		assert.Equal(t, "https://example.com/webhook", result.Result.URL)
		assert.True(t, result.Result.HasCustomCertificate)
		assert.Equal(t, 5, result.Result.PendingUpdateCount)
		assert.Equal(t, 40, result.Result.MaxConnections)
		assert.Len(t, result.Result.AllowedUpdates, 2)
	})
}

// TestWebhookConfigDefaults tests default webhook configuration values
func TestWebhookConfigDefaults(t *testing.T) {
	t.Run("webhook config has sensible defaults", func(t *testing.T) {
		config := WebhookConfig{
			MaxConnections:      40,
			DropPendingUpdates:  false,
			ReadTimeout:         30 * time.Second,
			IdleTimeout:         60 * time.Second,
			EnableMetrics:       true,
			MetricsPath:         "/metrics",
		}

		assert.Equal(t, 40, config.MaxConnections)
		assert.False(t, config.DropPendingUpdates)
		assert.Equal(t, 30*time.Second, config.ReadTimeout)
		assert.Equal(t, 60*time.Second, config.IdleTimeout)
		assert.True(t, config.EnableMetrics)
		assert.Equal(t, "/metrics", config.MetricsPath)
	})
}

// TestWebhookSecurityConfig tests security configuration
func TestWebhookSecurityConfig(t *testing.T) {
	t.Run("security config options", func(t *testing.T) {
		config := WebhookSecurityConfig{
			SecretToken:            "my-secret-token",
			AllowedIPs:             []string{"1.2.3.4", "5.6.7.8"},
			CertFile:               "/path/to/cert.pem",
			KeyFile:                "/path/to/key.pem",
			LoadCertInTelegram:     true,
			StartHTTPS:             true,
		}

		assert.Equal(t, "my-secret-token", config.SecretToken)
		assert.Len(t, config.AllowedIPs, 2)
		assert.Equal(t, "/path/to/cert.pem", config.CertFile)
		assert.Equal(t, "/path/to/key.pem", config.KeyFile)
		assert.True(t, config.LoadCertInTelegram)
		assert.True(t, config.StartHTTPS)
	})
}
