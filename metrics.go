// Package bote provides a comprehensive metrics system for Telegram bot monitoring.
// It includes metrics for updates, handlers, messages, users, webhooks, and errors.
package bote

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/maxbolgarin/abstract"
	"github.com/maxbolgarin/lang"
	"github.com/prometheus/client_golang/prometheus"
)

// Error type constants for metrics categorization
const (
	// Error types for different failure scenarios
	MetricsErrorBotBlocked       = "bot_blocked"        // Bot is blocked by user
	MetricsErrorHandler          = "handler"            // Handler execution error
	MetricsErrorInternal         = "internal"           // Internal bote package error
	MetricsErrorTelegramAPI      = "telegram_api"       // Telegram API error
	MetricsErrorInvalidUserState = "invalid_user_state" // Invalid user state error
	MetricsErrorBadUsage         = "bad_usage"          // Package usage error
	MetricsErrorConnectionError  = "connection_error"   // Connection error

	// Error severity levels
	MetricsErrorSeverityLow = "low"  // Low severity error
	MetricsErrorSeveritHigh = "high" // High severity error

	// Time window constants for active user metrics
	MetricsWindow1h  = "1h"  // 1 hour window
	MetricsWindow24h = "24h" // 24 hour window

	// Default subsystem name for metrics
	defaultSubsystem = "bote"

	defaultSessionLength = 15 * time.Minute
)

// Predefined histogram buckets for different metric types
var (
	// Standard histogram buckets for general duration metrics (1ms to 10s)
	MetricsHistogramBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	// Extended buckets for long-running handlers (0.5s to 10s)
	LongHandlerDurationBuckets = []float64{0.5, 1, 2, 4, 6, 8, 10}
	// Session length buckets for active users (1m to 24h)
	SessionLengthBucketsSeconds = []float64{10, 30, 60, 120, 300, 600, 900, 1800, 3600}
)

// metrics holds all Prometheus metrics for the bot system.
// It provides comprehensive monitoring for updates, handlers, messages, users, and webhooks.
type metrics struct {
	MetricsConfig // Configuration for metrics (namespace, subsystem, etc.)

	// Core bot metrics
	updatesTotal               prometheus.Counter       // Total number of updates received
	activeHandlers             prometheus.Gauge         // Number of currently active handlers
	stateRequestsTotal         *prometheus.CounterVec   // Total requests per state (labeled by state)
	handlerDurationSeconds     prometheus.Histogram     // All handlers execution duration
	longHandlerDurationSeconds *prometheus.HistogramVec // Long handlers duration by state

	// Message operation metrics
	sendMessagesTotal   prometheus.Counter // Total messages sent
	editMessagesTotal   prometheus.Counter // Total messages edited
	deleteMessagesTotal prometheus.Counter // Total messages deleted

	// Error tracking metrics
	errorsTotal *prometheus.CounterVec // Total errors by type and severity

	// User activity metrics
	totalActiveUsers         prometheus.Gauge     // Total number of created/initialized users
	currentActiveUsers       *prometheus.GaugeVec // Current active users by time window
	averageUsersActionsCount *prometheus.GaugeVec // Average number of actions per user
	sessionLength            prometheus.Histogram // Session length by time window
	userCacheSize            prometheus.Gauge     // Size of user cache

	// Webhook metrics
	webhookStatus        *prometheus.GaugeVec     // Webhook status by URL and address
	webhookRequestsTotal *prometheus.CounterVec   // Total webhook requests by path
	webhookErrorsTotal   *prometheus.CounterVec   // Total webhook errors by path and status
	webhookResponseTime  *prometheus.HistogramVec // Webhook response time by path
	webhookRequestsOnFly *prometheus.GaugeVec     // Current requests in flight by path

	// Internal state tracking
	activeHandlersCount int64                                    // Atomic counter for active handlers
	onFlyRequestsCount  int64                                    // Atomic counter for requests in flight
	userLastSeen        *abstract.SafeMap[int64, activeUserStat] // Thread-safe map of user last seen times
	lastUpdateTime      time.Time                                // Last time active users were updated

	disabled bool // Whether metrics collection is disabled
}

type activeUserStat struct {
	totalActions int64
	lastSeen     time.Time
	sessionStart time.Time
}

// newMetrics creates and initializes a new metrics instance with all Prometheus metrics.
// If config.Registry is nil, returns a disabled metrics instance.
func newMetrics(config MetricsConfig) *metrics {
	// Return disabled metrics if no registry is provided
	if config.Registry == nil {
		return &metrics{
			disabled: true,
		}
	}

	// Create metrics instance with thread-safe user tracking
	m := &metrics{
		MetricsConfig: config,
		userLastSeen:  abstract.NewSafeMap[int64, activeUserStat](),
	}

	// Initialize core bot metrics
	m.updatesTotal = m.newSimpleCounter("updates_total", "Total number of updates received")
	m.activeHandlers = m.newSimpleGauge("handlers_active", "Number of handlers that have at least one request")
	m.stateRequestsTotal = m.newCounter("state_requests_total", "Total number of requests for provided state", "state")
	m.handlerDurationSeconds = m.newSimpleHistogram("handler_duration_seconds", "All handlers execution duration in seconds", MetricsHistogramBuckets)
	m.longHandlerDurationSeconds = m.newHistogram("long_handler_duration_seconds", "Handler execution duration in seconds by state if it is longer than 1 second",
		LongHandlerDurationBuckets, "state")

	// Initialize message operation metrics
	m.sendMessagesTotal = m.newSimpleCounter("messages_send_total", "Total number of messages sent")
	m.editMessagesTotal = m.newSimpleCounter("messages_edit_total", "Total number of messages edited")
	m.deleteMessagesTotal = m.newSimpleCounter("messages_delete_total", "Total number of messages deleted")

	// Initialize error tracking metrics
	m.errorsTotal = m.newCounter("errors_total", "Total number of errors by type and severity", "type", "severity")

	// Initialize user activity metrics
	m.totalActiveUsers = m.newSimpleGauge("users_total_active", "Total number of created or initialized users")
	m.currentActiveUsers = m.newGauge("users_current_active", "Current number of active users by window", "window")
	m.averageUsersActionsCount = m.newGauge("users_average_actions_count", "Average number of actions per user", "window")
	m.sessionLength = m.newSimpleHistogram("users_session_length_seconds", "Session length in seconds", SessionLengthBucketsSeconds)
	m.userCacheSize = m.newSimpleGauge("users_cache_size", "Size of user cache")

	// Initialize webhook monitoring metrics
	m.webhookStatus = m.newGauge("webhook_status", "Webhook status", "url", "address")
	m.webhookRequestsTotal = m.newCounter("webhook_requests_total", "Total number of webhook requests", "path")
	m.webhookErrorsTotal = m.newCounter("webhook_errors_total", "Total number of webhook errors", "path", "status_code")
	m.webhookResponseTime = m.newHistogram("webhook_response_time_seconds", "Webhook response time in seconds", MetricsHistogramBuckets, "path")
	m.webhookRequestsOnFly = m.newGauge("webhook_requests_on_fly", "Number of requests on fly", "path")

	return m
}

// incUpdate increments the total updates counter.
// Called when a new update is received from Telegram.
func (m *metrics) incUpdate() {
	if m == nil || m.disabled {
		return
	}
	m.updatesTotal.Inc()
}

// incStateRequest increments the handler calls counter for the given state.
// Called when a handler is invoked for a specific state.
func (m *metrics) incStateRequest(state State) {
	if m == nil || m.disabled {
		return
	}
	m.stateRequestsTotal.WithLabelValues(state.String()).Inc()
}

// observeHandlerDuration records the handler execution duration.
// Called after each handler execution to track performance.
func (m *metrics) observeHandlerDuration(d time.Duration) {
	if m == nil || m.disabled {
		return
	}
	m.handlerDurationSeconds.Observe(d.Seconds())
}

// observeLongHandlerDuration records the long handler execution duration for the given state.
// Called for handlers that take longer than 1 second to execute.
func (m *metrics) observeLongHandlerDuration(state string, d time.Duration) {
	if m == nil || m.disabled {
		return
	}
	m.longHandlerDurationSeconds.WithLabelValues(state).Observe(d.Seconds())
}

// incActiveHandlers increments the active handlers gauge.
// Called when a handler starts execution to track concurrent handlers.
func (m *metrics) incActiveHandlers() {
	if m == nil || m.disabled {
		return
	}
	count := atomic.AddInt64(&m.activeHandlersCount, 1)
	m.activeHandlers.Set(float64(count))
}

// decActiveHandlers decrements the active handlers gauge.
// Called when a handler finishes execution to track concurrent handlers.
func (m *metrics) decActiveHandlers() {
	if m == nil || m.disabled {
		return
	}
	count := atomic.AddInt64(&m.activeHandlersCount, -1)
	if count < 0 {
		count = 0
	}
	m.activeHandlers.Set(float64(count))
}

// incError increments the error counter for the given error type and severity.
// Called when an error occurs to track error rates and types.
func (m *metrics) incError(errorType, severity string) {
	if m == nil || m.disabled {
		return
	}
	m.errorsTotal.WithLabelValues(errorType, severity).Inc()
}

// incSendMessagesTotal increments the send messages total counter.
// Called when a message is sent to track message sending activity.
func (m *metrics) incSendMessagesTotal() {
	if m == nil || m.disabled {
		return
	}
	m.sendMessagesTotal.Inc()
}

// incEditMessagesTotal increments the edit messages total counter.
// Called when a message is edited to track message editing activity.
func (m *metrics) incEditMessagesTotal() {
	if m == nil || m.disabled {
		return
	}
	m.editMessagesTotal.Inc()
}

// incDeleteMessagesTotal increments the delete messages total counter.
// Called when a message is deleted to track message deletion activity.
func (m *metrics) incDeleteMessagesTotal() {
	if m == nil || m.disabled {
		return
	}
	m.deleteMessagesTotal.Inc()
}

// incNewUser increments the total active users counter.
// Called when a new user is created or initialized.
func (m *metrics) incNewUser() {
	if m == nil || m.disabled {
		return
	}
	m.totalActiveUsers.Inc()
}

// addActiveUser records user activity and updates active user metrics.
// Called when a user interacts with the bot to track user engagement.
func (m *metrics) addActiveUser(userID int64) {
	if m == nil || m.disabled {
		return
	}
	m.userLastSeen.Change(userID, func(userID int64, stat activeUserStat) activeUserStat {
		sessionStart := stat.sessionStart
		if stat.lastSeen.IsZero() || sessionStart.IsZero() || time.Since(stat.lastSeen) > defaultSessionLength {
			sessionStart = time.Now()
		}
		return activeUserStat{
			totalActions: stat.totalActions + 1,
			lastSeen:     time.Now(),
			sessionStart: sessionStart,
		}
	})

	// Update active users every minute to prevent high load
	if time.Since(m.lastUpdateTime) > time.Minute {
		m.updateActiveUsers()
	}
}

// incUserCacheSize increments the user cache size gauge.
// Called when a user is added to the cache to track the size of the cache.
func (m *metrics) setUserCacheSize(size int) {
	if m == nil || m.disabled {
		return
	}
	m.userCacheSize.Set(float64(size))
}

// updateActiveUsers computes and updates active user counts for 1h and 24h windows.
// This method is called periodically to maintain accurate user activity metrics.
func (m *metrics) updateActiveUsers() {
	var (
		now                = time.Now()
		oneHourAgo         = now.Add(-1 * time.Hour)
		twentyFourHoursAgo = now.Add(-24 * time.Hour)

		users1h, users24h, actions1h, actions24h int64
	)

	var toDelete []int64

	// Iterate through all users and categorize by activity
	m.userLastSeen.Range(func(userID int64, stat activeUserStat) bool {
		if !stat.sessionStart.IsZero() {
			m.sessionLength.Observe(time.Since(stat.sessionStart).Seconds())
		}
		// Count users active in last 1h
		if stat.lastSeen.After(oneHourAgo) {
			users1h++
			actions1h += stat.totalActions
			users24h++
			actions24h += stat.totalActions
		} else if stat.lastSeen.After(twentyFourHoursAgo) {
			// Count users active in last 24h but not in last 1h
			users24h++
			actions24h += stat.totalActions
		} else {
			// Mark for deletion if older than 24h
			toDelete = append(toDelete, userID)
		}
		return true
	})

	// Clean up old entries to prevent memory leaks
	for _, userID := range toDelete {
		m.userLastSeen.Delete(userID)
	}

	// Update gauges with current counts
	if users1h > 0 {
		m.currentActiveUsers.WithLabelValues(MetricsWindow1h).Set(float64(users1h))
		m.averageUsersActionsCount.WithLabelValues(MetricsWindow1h).Set(float64(actions1h / users1h))
	}
	if users24h > 0 {
		m.currentActiveUsers.WithLabelValues(MetricsWindow24h).Set(float64(users24h))
		m.averageUsersActionsCount.WithLabelValues(MetricsWindow24h).Set(float64(actions24h / users24h))
	}

	m.lastUpdateTime = now
}

// setWebhookStatus sets the webhook status gauge for the given URL and address.
// Called when webhook status changes to track webhook health.
func (m *metrics) setWebhookStatus(url string, address string) {
	if m == nil || m.disabled {
		return
	}
	m.webhookStatus.WithLabelValues(url, address).Set(1)
}

// HandleRequest records webhook request metrics.
// Called at the start of webhook request processing to track request volume and concurrency.
func (m *metrics) HandleRequest(r *http.Request) {
	if m == nil || m.disabled {
		return
	}
	m.webhookRequestsTotal.WithLabelValues(r.URL.Path).Inc()
	atomic.AddInt64(&m.onFlyRequestsCount, 1)
	m.webhookRequestsOnFly.WithLabelValues(r.URL.Path).Set(float64(m.onFlyRequestsCount))
}

// HandleResponse records webhook response metrics.
// Called at the end of webhook request processing to track response times and errors.
func (m *metrics) HandleResponse(r *http.Request, w http.ResponseWriter, statusCode int, duration time.Duration) {
	if m == nil || m.disabled {
		return
	}
	// Record error if status code indicates failure
	if statusCode >= 400 {
		m.webhookErrorsTotal.WithLabelValues(r.URL.Path, strconv.Itoa(statusCode)).Inc()
	}
	// Record response time and update concurrency metrics
	m.webhookResponseTime.WithLabelValues(r.URL.Path).Observe(duration.Seconds())
	atomic.AddInt64(&m.onFlyRequestsCount, -1)
	m.webhookRequestsOnFly.WithLabelValues(r.URL.Path).Set(float64(m.onFlyRequestsCount))
}

// newCounter creates a new CounterVec with the given name, help text, and label names.
// The counter is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newCounter(name, help string, labelNames ...string) *prometheus.CounterVec {
	counter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
		},
		labelNames,
	)
	r.Registry.MustRegister(counter)
	return counter
}

// newGauge creates a new GaugeVec with the given name, help text, and label names.
// The gauge is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newGauge(name, help string, labelNames ...string) *prometheus.GaugeVec {
	gauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
		},
		labelNames,
	)
	r.Registry.MustRegister(gauge)
	return gauge
}

// newHistogram creates a new HistogramVec with the given name, help text, buckets, and label names.
// The histogram is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newHistogram(name, help string, buckets []float64, labelNames ...string) *prometheus.HistogramVec {
	histogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
			Buckets:     buckets,
		},
		labelNames,
	)
	r.Registry.MustRegister(histogram)
	return histogram
}

// newSimpleCounter creates a new Counter with the given name and help text.
// The counter is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newSimpleCounter(name, help string) prometheus.Counter {
	counter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
		},
	)
	r.Registry.MustRegister(counter)
	return counter
}

// newSimpleGauge creates a new Gauge with the given name and help text.
// The gauge is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newSimpleGauge(name, help string) prometheus.Gauge {
	gauge := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
		},
	)
	r.Registry.MustRegister(gauge)
	return gauge
}

// newSimpleHistogram creates a new Histogram with the given name, help text, and buckets.
// The histogram is automatically registered with the registry.
// Uses the configured subsystem or defaults to "bote".
func (r *metrics) newSimpleHistogram(name, help string, buckets []float64) prometheus.Histogram {
	histogram := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace:   r.Namespace,
			Subsystem:   lang.Check(r.Subsystem, defaultSubsystem),
			Name:        name,
			Help:        help,
			ConstLabels: r.ConstLabels,
			Buckets:     buckets,
		},
	)
	r.Registry.MustRegister(histogram)
	return histogram
}
