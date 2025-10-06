package bote

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/maxbolgarin/erro"
	"github.com/maxbolgarin/lang"
	"github.com/maxbolgarin/servex/v2"
	tele "gopkg.in/telebot.v4"
)

// webhookPoller implements tele.Poller interface for webhook-based updates.
type webhookPoller struct {
	srv     *servex.Server
	cfg     WebhookConfig
	log     Logger
	metrics *webhookMetrics

	bot     *tele.Bot
	updates chan tele.Update
	stopCh  chan struct{}

	shutdownOnce sync.Once
}

// newWebhookPoller creates a new webhook poller with the given configuration.
func newWebhookPoller(config WebhookConfig, logger Logger) (*webhookPoller, error) {
	if err := prepareCertificate(config, logger); err != nil {
		return nil, erro.Wrap(err, "prepare certificate")
	}

	metrics := &webhookMetrics{}

	servexOpts := []servex.Option{
		servex.WithNoRequestLog(),
		servex.WithReadTimeout(config.ReadTimeout),
		servex.WithIdleTimeout(config.IdleTimeout),
		servex.WithHealthEndpoint(),
		servex.WithMetrics(metrics),
		servex.WithLogger(logger),
	}

	if config.Security.CertFile != "" && config.Security.KeyFile != "" {
		servexOpts = append(servexOpts,
			servex.WithCertificateFromFile(config.Security.CertFile, config.Security.KeyFile),
		)
	}
	if len(config.Security.AllowedIPs) > 0 {
		servexOpts = append(servexOpts,
			servex.WithAllowedIPs(config.Security.AllowedIPs...),
			servex.WithFilterTrustedProxies("127.0.0.1"),
		)
	}
	if lang.Deref(config.Security.SecurityHeaders) {
		servexOpts = append(servexOpts,
			servex.WithSecurityHeaders(),
		)
	}
	if lang.Deref(config.RateLimit.Enabled) {
		servexOpts = append(servexOpts,
			servex.WithRPS(config.RateLimit.RequestsPerSecond),
			servex.WithBurstSize(config.RateLimit.BurstSize),
		)
	}

	srv, err := servex.NewServer(servexOpts...)
	if err != nil {
		return nil, erro.Wrap(err, "create servex server")
	}

	wp := &webhookPoller{
		cfg:     config,
		srv:     srv,
		log:     logger,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}

	wp.srv.POST(config.urlParsed.Path, wp.handleWebhook)

	return wp, nil
}

// Poll implements tele.Poller interface.
func (wp *webhookPoller) Poll(bot *tele.Bot, updates chan tele.Update, stop chan struct{}) {
	wp.bot = bot
	wp.updates = updates

	var start func(string) error
	if wp.cfg.Security.StartHTTPS {
		start = wp.srv.StartHTTPS
	} else {
		start = wp.srv.StartHTTP
	}

	if err := start(wp.cfg.Listen); err != nil {
		wp.log.Error("failed to start server", "error", err.Error(), "listen", wp.cfg.Listen)
		close(wp.stopCh)
		return
	}

	// Set webhook on Telegram
	if err := wp.setWebhook(); err != nil {
		wp.log.Error("failed to set webhook", "error", err.Error())
		close(wp.stopCh)
		return
	}

	wp.log.Info("webhook poller started", "url", wp.cfg.URL, "listen", wp.cfg.Listen)

	select {
	case <-stop:
		wp.log.Info("webhook poller stopping")
	case <-wp.stopCh:
		wp.log.Info("webhook poller stopped")
	}

	wp.log.Info("webhook poller stopping")
}

// setWebhook configures the webhook on Telegram's side.
func (wp *webhookPoller) setWebhook() error {
	webhookURL := wp.cfg.URL

	// Prepare webhook parameters
	params := map[string]interface{}{
		"url":                  webhookURL,
		"max_connections":      wp.cfg.MaxConnections,
		"drop_pending_updates": wp.cfg.DropPendingUpdates,
	}

	if wp.cfg.Security.SecretToken != "" {
		params["secret_token"] = wp.cfg.Security.SecretToken
	}

	if len(wp.cfg.AllowedUpdates) > 0 {
		params["allowed_updates"] = wp.cfg.AllowedUpdates
	}

	// Upload certificate if self-signed
	if wp.cfg.Security.LoadCertInTelegram && wp.cfg.Security.CertFile != "" {
		certData, err := os.ReadFile(wp.cfg.Security.CertFile)
		if err != nil {
			return erro.Wrap(err, "read certificate file")
		}
		params["certificate"] = &tele.Document{
			File:     tele.FromReader(strings.NewReader(string(certData))),
			FileName: "cert.pem",
		}
	}

	// Use telebot's method to set webhook
	_, err := wp.bot.Raw("setWebhook", params)
	if err != nil {
		return erro.Wrap(err, "set webhook")
	}

	wp.log.Debug("webhook set successfully",
		"url", webhookURL,
		"max_connections", wp.cfg.MaxConnections,
		"drop_pending_updates", wp.cfg.DropPendingUpdates,
		"secret_token", lang.If(wp.cfg.Security.SecretToken != "", "set", "not set"),
		"allowed_updates", wp.cfg.AllowedUpdates,
		"allowed_ips", wp.cfg.Security.AllowedIPs,
		"is_loaded_cert", wp.cfg.Security.LoadCertInTelegram,
	)

	return nil
}

// handleWebhook handles incoming webhook requests.
func (wp *webhookPoller) handleWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := wp.srv.NewContext(w, r)

	if err := wp.validateRequest(r); err != nil {
		ctx.BadRequest(err, "request validation failed")
		return
	}

	update, err := servex.ReadJSON[tele.Update](r)
	if err != nil {
		ctx.BadRequest(err, "failed to read update")
		return
	}

	// Send update to channel (non-blocking)
	select {
	case wp.updates <- update:
		ctx.Response(http.StatusOK)

	default:
		wp.log.Warn("update channel full, dropping update")

		ctx.ServiceUnavailable(nil, "update channel full, dropping update")
	}
}

// validateRequest validates the Telegram webhook request.
func (wp *webhookPoller) validateRequest(r *http.Request) error {
	if lang.Deref(wp.cfg.Security.CheckTLSInRequest) && r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		return erro.New("HTTPS required")
	}

	if wp.cfg.Security.SecretToken == "" {
		return nil
	}

	signature := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if signature == "" {
		return erro.New("missing signature header")
	}

	if subtle.ConstantTimeCompare([]byte(signature), []byte(wp.cfg.Security.SecretToken)) != 1 {
		return erro.New("invalid signature")
	}

	return nil
}

// shutdown gracefully shuts down the webhook poller.
func (wp *webhookPoller) shutdown(ctx context.Context) error {
	errList := erro.NewList()

	wp.shutdownOnce.Do(func() {
		// Delete webhook from Telegram
		if err := wp.deleteWebhook(); err != nil {
			errList.Add(err)
		}

		// Stop HTTP server
		if err := wp.srv.Shutdown(ctx); err != nil {
			errList.Add(err)
		}

		wp.log.Debug("webhook poller shutdown complete")
	})

	return errList.Err()
}

// deleteWebhook removes the webhook from Telegram.
func (wp *webhookPoller) deleteWebhook() error {
	if wp.bot == nil {
		return nil
	}

	_, err := wp.bot.Raw("deleteWebhook", map[string]interface{}{
		"drop_pending_updates": wp.cfg.DropPendingUpdates,
	})
	if err != nil {
		return erro.Wrap(err, "delete webhook")
	}

	wp.log.Debug("webhook deleted successfully")
	return nil
}

// prepareCertificate prepares the certificate for the webhook.
// It validates the certificate if it exists, and generates a self-signed certificate if it doesn't.
func prepareCertificate(config WebhookConfig, logger Logger) error {
	genSelfSignedCert := lang.Deref(config.Security.GenerateSelfSignedCert)

	if !genSelfSignedCert && (config.Security.CertFile == "" || config.Security.KeyFile == "") {
		return nil
	}

	err := validateCertificate(config.Security.CertFile, config.Security.KeyFile, logger)
	if err != nil && !genSelfSignedCert {
		return erro.Wrap(err, "validate certificate")
	}

	if err == nil {
		if genSelfSignedCert {
			logger.Info("certificate is valid, skipping self-signed certificate generation",
				"cert_file", config.Security.CertFile, "key_file", config.Security.KeyFile)
		}
		return nil
	}

	// There is an error with current certificate and generation is enabled

	certFile, keyFile, err := generateSelfSignedCert(
		config.Security.CertFile,
		config.Security.KeyFile,
		config.urlParsed.Host,
		logger,
	)
	if err != nil {
		return erro.Wrap(err, "generate self-signed certificate")
	}

	config.Security.CertFile = certFile
	config.Security.KeyFile = keyFile

	return nil
}

// GenerateSelfSignedCert generates a self-signed certificate for webhook use.
// This is useful for development and testing environments.
func generateSelfSignedCert(certFile, keyFile, domain string, logger Logger) (string, string, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", erro.Wrap(err, "generate private key")
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Bote Bot"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	// Add domain to certificate
	if domain != "" {
		template.DNSNames = append(template.DNSNames, domain)

		// Extract hostname from domain if it's a URL
		if strings.HasPrefix(domain, "https://") || strings.HasPrefix(domain, "http://") {
			if u, err := url.Parse(domain); err == nil && u.Host != "" {
				template.DNSNames = append(template.DNSNames, u.Host)
			}
		}
	}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return "", "", erro.Wrap(err, "create certificate")
	}

	certFile = lang.Check(certFile, "./cert.pem")
	keyFile = lang.Check(keyFile, "./key.pem")

	// Create directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(certFile), 0755); err != nil {
		return "", "", erro.Wrap(err, "create cert directory")
	}
	if err := os.MkdirAll(filepath.Dir(keyFile), 0755); err != nil {
		return "", "", erro.Wrap(err, "create key directory")
	}

	// Save certificate
	certOut, err := os.Create(certFile)
	if err != nil {
		return "", "", erro.Wrap(err, "create cert file")
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return "", "", erro.Wrap(err, "encode certificate")
	}

	// Save private key
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return "", "", erro.Wrap(err, "create key file")
	}
	defer keyOut.Close()

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", erro.Wrap(err, "marshal private key")
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}); err != nil {
		return "", "", erro.Wrap(err, "encode private key")
	}

	logger.Info("self-signed certificate generated",
		"cert_file", certFile,
		"key_file", keyFile,
		"domain", domain,
	)

	return certFile, keyFile, nil
}

func validateCertificate(certFile, keyFile string, logger Logger) error {
	// Check if files exist
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		return erro.New("certificate file does not exist", "file", certFile)
	}
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		return erro.New("key file does not exist", "file", keyFile)
	}

	// Try to load the certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return erro.Wrap(err, "load certificate pair")
	}

	// Parse the certificate to get more info
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return erro.Wrap(err, "parse certificate")
	}

	// Check if certificate is expired
	now := time.Now()
	if now.Before(x509Cert.NotBefore) {
		return erro.New("certificate is not yet valid", "not_before", x509Cert.NotBefore)
	}
	if now.After(x509Cert.NotAfter) {
		return erro.New("certificate has expired", "not_after", x509Cert.NotAfter)
	}

	// Warn if certificate expires soon (within 30 days)
	if time.Until(x509Cert.NotAfter) < 30*24*time.Hour {
		logger.Warn("certificate expires soon",
			"expires_at", x509Cert.NotAfter,
			"days_remaining", int(time.Until(x509Cert.NotAfter).Hours()/24))
	}

	logger.Debug("certificate validation successful",
		"cert_file", certFile,
		"expires_at", x509Cert.NotAfter,
		"dns_names", x509Cert.DNSNames,
	)

	return nil
}

// TODO: b
type webhookMetrics struct {
	requestsTotal int
	errorsTotal   int
}

func (m *webhookMetrics) HandleRequest(r *http.Request) {
	m.requestsTotal++
}

func (m *webhookMetrics) HandleResponse(r *http.Request, w http.ResponseWriter, statusCode int, duration time.Duration) {
	m.errorsTotal++
}

// WebhookInfo contains information about the current webhook configuration.
type webhookInfo struct {
	URL                          string   `json:"url"`
	HasCustomCertificate         bool     `json:"has_custom_certificate"`
	PendingUpdateCount           int      `json:"pending_update_count"`
	IPAddress                    string   `json:"ip_address,omitempty"`
	LastErrorDate                int64    `json:"last_error_date,omitempty"`
	LastErrorMessage             string   `json:"last_error_message,omitempty"`
	LastSynchronizationErrorDate int64    `json:"last_synchronization_error_date,omitempty"`
	MaxConnections               int      `json:"max_connections,omitempty"`
	AllowedUpdates               []string `json:"allowed_updates,omitempty"`
}

// GetWebhookInfo retrieves current webhook information from Telegram.
func (wp *webhookPoller) GetWebhookInfo() (*webhookInfo, error) {
	if wp.bot == nil {
		return nil, erro.New("bot not initialized")
	}

	resp, err := wp.bot.Raw("getWebhookInfo", nil)
	if err != nil {
		return nil, erro.Wrap(err, "get webhook info")
	}

	var result struct {
		Ok     bool        `json:"ok"`
		Result webhookInfo `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, erro.Wrap(err, "parse webhook info response")
	}

	if !result.Ok {
		return nil, erro.New("telegram API returned error")
	}

	return &result.Result, nil
}
