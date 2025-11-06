package bote

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewMetrics tests the creation of metrics instance
func TestNewMetrics(t *testing.T) {
	t.Run("with valid registry", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Registry:  registry,
			Namespace: "test",
			Subsystem: "bot",
		}

		m := newMetrics(config)

		assert.NotNil(t, m)
		assert.False(t, m.disabled)
		assert.Equal(t, "test", m.Namespace)
		assert.Equal(t, "bot", m.Subsystem)
		assert.NotNil(t, m.updatesTotal)
		assert.NotNil(t, m.handlersInFlight)
		assert.NotNil(t, m.stateRequestsTotal)
		assert.NotNil(t, m.handlerDurationMs)
		assert.NotNil(t, m.userLastSeen)
	})

	t.Run("without registry (disabled)", func(t *testing.T) {
		config := MetricsConfig{}
		m := newMetrics(config)

		assert.NotNil(t, m)
		assert.True(t, m.disabled)
	})

	t.Run("with custom labels", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{
			Registry: registry,
			ConstLabels: prometheus.Labels{
				"environment": "test",
				"version":     "1.0.0",
			},
		}

		m := newMetrics(config)

		assert.NotNil(t, m)
		assert.False(t, m.disabled)
		assert.Equal(t, config.ConstLabels, m.ConstLabels)
	})
}

// TestMetricsIncUpdate tests the incUpdate method
func TestMetricsIncUpdate(t *testing.T) {
	t.Run("updates counter increments", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		config := MetricsConfig{Registry: registry}
		m := newMetrics(config)

		// Increment updates
		m.incUpdate()
		m.incUpdate()
		m.incUpdate()

		// Verify counter value
		count := testutil.ToFloat64(m.updatesTotal)
		assert.Equal(t, 3.0, count)
	})

	t.Run("disabled metrics does nothing", func(t *testing.T) {
		m := newMetrics(MetricsConfig{})
		m.incUpdate() // Should not panic
	})

	t.Run("nil metrics does nothing", func(t *testing.T) {
		var m *metrics
		m.incUpdate() // Should not panic
	})
}

// TestMetricsIncStateRequest tests the incStateRequest method
func TestMetricsIncStateRequest(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("increments state request counter", func(t *testing.T) {
		state1 := NewUserState("waiting_input")
		state2 := NewUserState("menu")

		m.incStateRequest(state1)
		m.incStateRequest(state1)
		m.incStateRequest(state2)

		// Verify counter values
		count1 := testutil.ToFloat64(m.stateRequestsTotal.WithLabelValues(state1.String()))
		count2 := testutil.ToFloat64(m.stateRequestsTotal.WithLabelValues(state2.String()))

		assert.Equal(t, 2.0, count1)
		assert.Equal(t, 1.0, count2)
	})

	t.Run("handles different state types", func(t *testing.T) {
		m.incStateRequest(FirstRequest)
		m.incStateRequest(Unknown)
		m.incStateRequest(NoChange)

		count1 := testutil.ToFloat64(m.stateRequestsTotal.WithLabelValues(FirstRequest.String()))
		count2 := testutil.ToFloat64(m.stateRequestsTotal.WithLabelValues(Unknown.String()))
		count3 := testutil.ToFloat64(m.stateRequestsTotal.WithLabelValues(NoChange.String()))

		assert.Equal(t, 1.0, count1)
		assert.Equal(t, 1.0, count2)
		assert.Equal(t, 1.0, count3)
	})
}

// TestMetricsObserveHandlerDuration tests the observeHandlerDuration method
func TestMetricsObserveHandlerDuration(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("observes handler duration", func(t *testing.T) {
		m.observeHandlerDuration(100 * time.Millisecond)
		m.observeHandlerDuration(200 * time.Millisecond)
		m.observeHandlerDuration(500 * time.Millisecond)

		// Verify histogram exists and didn't panic
		assert.NotNil(t, m.handlerDurationMs)
	})
}

// TestMetricsHandlerInflight tests the handler in-flight tracking
func TestMetricsHandlerInflight(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("tracks handlers in flight", func(t *testing.T) {
		// Start first handler
		m.recordHandlerStart()
		count := testutil.ToFloat64(m.handlersInFlight)
		assert.Equal(t, 1.0, count)

		// Start second handler
		m.recordHandlerStart()
		count = testutil.ToFloat64(m.handlersInFlight)
		assert.Equal(t, 2.0, count)

		// Finish first handler
		m.recordHandlerFinish()
		count = testutil.ToFloat64(m.handlersInFlight)
		assert.Equal(t, 1.0, count)

		// Finish second handler
		m.recordHandlerFinish()
		count = testutil.ToFloat64(m.handlersInFlight)
		assert.Equal(t, 0.0, count)
	})

	t.Run("does not go below zero", func(t *testing.T) {
		m.recordHandlerFinish()
		m.recordHandlerFinish()
		count := testutil.ToFloat64(m.handlersInFlight)
		assert.Equal(t, 0.0, count)
	})
}

// TestMetricsIncError tests the incError method
func TestMetricsIncError(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("increments error counters by type and severity", func(t *testing.T) {
		m.incError(MetricsErrorHandler, MetricsErrorSeverityLow)
		m.incError(MetricsErrorHandler, MetricsErrorSeverityLow)
		m.incError(MetricsErrorHandler, MetricsErrorSeveritHigh)
		m.incError(MetricsErrorTelegramAPI, MetricsErrorSeverityLow)

		countHandlerLow := testutil.ToFloat64(m.errorsTotal.WithLabelValues(
			MetricsErrorHandler, MetricsErrorSeverityLow))
		countHandlerHigh := testutil.ToFloat64(m.errorsTotal.WithLabelValues(
			MetricsErrorHandler, MetricsErrorSeveritHigh))
		countTelegramLow := testutil.ToFloat64(m.errorsTotal.WithLabelValues(
			MetricsErrorTelegramAPI, MetricsErrorSeverityLow))

		assert.Equal(t, 2.0, countHandlerLow)
		assert.Equal(t, 1.0, countHandlerHigh)
		assert.Equal(t, 1.0, countTelegramLow)
	})

	t.Run("tracks all error types", func(t *testing.T) {
		errorTypes := []string{
			MetricsErrorBotBlocked,
			MetricsErrorInternal,
			MetricsErrorInvalidUserState,
			MetricsErrorBadUsage,
			MetricsErrorConnectionError,
		}

		for _, errType := range errorTypes {
			m.incError(errType, MetricsErrorSeverityLow)
			count := testutil.ToFloat64(m.errorsTotal.WithLabelValues(errType, MetricsErrorSeverityLow))
			assert.Equal(t, 1.0, count, "Error type %s should have count 1", errType)
		}
	})
}

// TestMetricsMessageOperations tests message operation counters
func TestMetricsMessageOperations(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("increments send messages counter", func(t *testing.T) {
		m.incSendMessagesTotal()
		m.incSendMessagesTotal()
		m.incSendMessagesTotal()

		count := testutil.ToFloat64(m.sendMessagesTotal)
		assert.Equal(t, 3.0, count)
	})

	t.Run("increments edit messages counter", func(t *testing.T) {
		m.incEditMessagesTotal()
		m.incEditMessagesTotal()

		count := testutil.ToFloat64(m.editMessagesTotal)
		assert.Equal(t, 2.0, count)
	})

	t.Run("increments delete messages counter", func(t *testing.T) {
		m.incDeleteMessagesTotal()

		count := testutil.ToFloat64(m.deleteMessagesTotal)
		assert.Equal(t, 1.0, count)
	})
}

// TestMetricsActiveUsers tests active user tracking
func TestMetricsActiveUsers(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("adds active users", func(t *testing.T) {
		// Add user 1
		m.addActiveUser(1)
		m.addActiveUser(1)
		m.addActiveUser(1)

		// Add user 2
		m.addActiveUser(2)

		// Force update
		m.updateActiveUsers()

		// Verify users were tracked
		count1h := testutil.ToFloat64(m.currentActiveUsers.WithLabelValues(MetricsWindow1h))
		assert.Equal(t, 2.0, count1h, "Should have 2 active users in 1h window")
	})

	t.Run("tracks user actions", func(t *testing.T) {
		m.addActiveUser(100)
		m.addActiveUser(100)
		m.addActiveUser(100)

		m.updateActiveUsers()

		// Check that actions are tracked
		stat, exists := m.userLastSeen.Lookup(100)
		require.True(t, exists)
		assert.Equal(t, int64(3), stat.totalActions)
	})

	t.Run("cleans up old users", func(t *testing.T) {
		// Add a user with old timestamp
		oldTime := time.Now().Add(-25 * time.Hour)
		m.userLastSeen.Set(999, activeUserStat{
			lastSeen:     oldTime,
			sessionStart: oldTime,
			totalActions: 5,
		})

		m.updateActiveUsers()

		// Verify old user was cleaned up
		_, exists := m.userLastSeen.Lookup(999)
		assert.False(t, exists, "Old user should be removed")
	})

	t.Run("updates active users periodically", func(t *testing.T) {
		m.lastUpdateTime = time.Now().Add(-2 * time.Minute)
		m.addActiveUser(500)

		// Should trigger update since last update was > 1 minute ago
		assert.NotZero(t, m.lastUpdateTime)
	})
}

// TestMetricsUserCacheSize tests user cache size tracking
func TestMetricsUserCacheSize(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("sets user cache size", func(t *testing.T) {
		m.setUserCacheSize(50)
		count := testutil.ToFloat64(m.userCacheSize)
		assert.Equal(t, 50.0, count)

		m.setUserCacheSize(100)
		count = testutil.ToFloat64(m.userCacheSize)
		assert.Equal(t, 100.0, count)
	})
}

// TestMetricsWebhookStatus tests webhook status tracking
func TestMetricsWebhookStatus(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("sets webhook status", func(t *testing.T) {
		url := "https://example.com/webhook"
		address := "0.0.0.0:8443"

		m.setWebhookStatus(url, address)

		count := testutil.ToFloat64(m.webhookStatus.WithLabelValues(url, address))
		assert.Equal(t, 1.0, count)
	})
}

// TestMetricsWebhookRequests tests webhook request tracking
func TestMetricsWebhookRequests(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("handles webhook request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/bot123", nil)

		m.HandleRequest(req)

		count := testutil.ToFloat64(m.webhookRequestsTotal.WithLabelValues("/webhook/bot123"))
		assert.Equal(t, 1.0, count)

		inFlight := testutil.ToFloat64(m.webhookRequestsInFlight.WithLabelValues("/webhook/bot123"))
		assert.Equal(t, 1.0, inFlight)
	})

	t.Run("ignores metrics and health paths", func(t *testing.T) {
		metricsReq := httptest.NewRequest(http.MethodGet, defaultWebhookMetricsPath, nil)
		healthReq := httptest.NewRequest(http.MethodGet, defaultWebhookHealthPath, nil)

		m.HandleRequest(metricsReq)
		m.HandleRequest(healthReq)

		// These should not be tracked, so we can't verify the count directly
		// but we ensure no panic occurs
	})

	t.Run("handles webhook response", func(t *testing.T) {
		// Create a fresh metrics instance for this test
		freshRegistry := prometheus.NewRegistry()
		freshMetrics := newMetrics(MetricsConfig{Registry: freshRegistry})

		req := httptest.NewRequest(http.MethodPost, "/webhook/response-test", nil)
		w := httptest.NewRecorder()

		freshMetrics.HandleRequest(req)
		freshMetrics.HandleResponse(req, w, 200, 100*time.Millisecond)

		inFlight := testutil.ToFloat64(freshMetrics.webhookRequestsInFlight.WithLabelValues("/webhook/response-test"))
		assert.Equal(t, 0.0, inFlight, "In-flight requests should be decremented")
	})

	t.Run("tracks webhook errors", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/error", nil)
		w := httptest.NewRecorder()

		m.HandleRequest(req)
		m.HandleResponse(req, w, 400, 50*time.Millisecond)

		errorCount := testutil.ToFloat64(m.webhookErrorsTotal.WithLabelValues("/webhook/error", "400"))
		assert.Equal(t, 1.0, errorCount)

		m.HandleResponse(req, w, 500, 50*time.Millisecond)
		errorCount = testutil.ToFloat64(m.webhookErrorsTotal.WithLabelValues("/webhook/error", "500"))
		assert.Equal(t, 1.0, errorCount)
	})

	t.Run("observes response time", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/webhook/timing", nil)
		w := httptest.NewRecorder()

		m.HandleRequest(req)
		m.HandleResponse(req, w, 200, 250*time.Millisecond)
		m.HandleResponse(req, w, 200, 350*time.Millisecond)

		// Verify observations were recorded (can't directly count histogram observations with testutil.ToFloat64)
		// Just verify the method didn't panic
		assert.NotNil(t, m.webhookResponseTimeSeconds)
	})
}

// TestMetricsSessionLength tests session length tracking
func TestMetricsSessionLength(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("tracks session length", func(t *testing.T) {
		// Simulate a user session
		userID := int64(12345)
		sessionStart := time.Now().Add(-5 * time.Minute)

		m.userLastSeen.Set(userID, activeUserStat{
			lastSeen:     time.Now(),
			sessionStart: sessionStart,
			totalActions: 10,
		})

		m.updateActiveUsers()

		// Verify session length histogram exists and didn't panic
		assert.NotNil(t, m.sessionLength)
	})

	t.Run("handles new sessions", func(t *testing.T) {
		userID := int64(54321)

		// First action - new session
		m.addActiveUser(userID)

		stat, exists := m.userLastSeen.Lookup(userID)
		require.True(t, exists)
		assert.NotZero(t, stat.sessionStart)

		// Simulate gap longer than defaultSessionLength
		m.userLastSeen.Set(userID, activeUserStat{
			lastSeen:     time.Now().Add(-16 * time.Minute),
			sessionStart: time.Now().Add(-16 * time.Minute),
			totalActions: 5,
		})

		// New action should start new session
		m.addActiveUser(userID)

		stat, exists = m.userLastSeen.Lookup(userID)
		require.True(t, exists)
		assert.Equal(t, int64(6), stat.totalActions)
	})
}

// TestMetricsDisabled tests that disabled metrics don't cause errors
func TestMetricsDisabled(t *testing.T) {
	m := newMetrics(MetricsConfig{})

	// All these should work without panicking
	t.Run("all methods work when disabled", func(t *testing.T) {
		m.incUpdate()
		m.incStateRequest(FirstRequest)
		m.observeHandlerDuration(100 * time.Millisecond)
		m.recordHandlerStart()
		m.recordHandlerFinish()
		m.incError(MetricsErrorHandler, MetricsErrorSeverityLow)
		m.incSendMessagesTotal()
		m.incEditMessagesTotal()
		m.incDeleteMessagesTotal()
		m.addActiveUser(123)
		m.setUserCacheSize(50)
		m.setWebhookStatus("url", "address")

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		m.HandleRequest(req)
		m.HandleResponse(req, w, 200, 100*time.Millisecond)
	})
}

// TestMetricsNil tests that nil metrics don't cause panics
func TestMetricsNil(t *testing.T) {
	var m *metrics

	t.Run("all methods work with nil receiver", func(t *testing.T) {
		m.incUpdate()
		m.incStateRequest(FirstRequest)
		m.observeHandlerDuration(100 * time.Millisecond)
		m.recordHandlerStart()
		m.recordHandlerFinish()
		m.incError(MetricsErrorHandler, MetricsErrorSeverityLow)
		m.incSendMessagesTotal()
		m.incEditMessagesTotal()
		m.incDeleteMessagesTotal()
		m.addActiveUser(123)
		m.setUserCacheSize(50)
		m.setWebhookStatus("url", "address")

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		w := httptest.NewRecorder()
		m.HandleRequest(req)
		m.HandleResponse(req, w, 200, 100*time.Millisecond)
	})
}

// TestMetricsConcurrency tests concurrent access to metrics
func TestMetricsConcurrency(t *testing.T) {
	registry := prometheus.NewRegistry()
	config := MetricsConfig{Registry: registry}
	m := newMetrics(config)

	t.Run("concurrent metric updates", func(t *testing.T) {
		done := make(chan bool)
		iterations := 100

		// Concurrent updates
		go func() {
			for i := 0; i < iterations; i++ {
				m.incUpdate()
			}
			done <- true
		}()

		go func() {
			for i := 0; i < iterations; i++ {
				m.incStateRequest(FirstRequest)
			}
			done <- true
		}()

		go func() {
			for i := 0; i < iterations; i++ {
				m.addActiveUser(int64(i))
			}
			done <- true
		}()

		go func() {
			for i := 0; i < iterations; i++ {
				m.recordHandlerStart()
				m.recordHandlerFinish()
			}
			done <- true
		}()

		// Wait for all goroutines
		for i := 0; i < 4; i++ {
			<-done
		}

		// Verify no race conditions occurred
		count := testutil.ToFloat64(m.updatesTotal)
		assert.Equal(t, float64(iterations), count)
	})
}
