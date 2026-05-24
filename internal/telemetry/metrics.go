package telemetry

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	NotificationsCreated   *prometheus.CounterVec
	NotificationsDelivered *prometheus.CounterVec
	NotificationsFailed    *prometheus.CounterVec
	NotificationsRetried   *prometheus.CounterVec
	DeliveryLatency        *prometheus.HistogramVec
	QueueDepth             *prometheus.GaugeVec
	HTTPRequestDuration    *prometheus.HistogramVec
	HTTPRequestsTotal      *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		NotificationsCreated: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "notifications_created_total",
			Help: "Total number of notifications created",
		}, []string{"channel", "priority"}),

		NotificationsDelivered: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "notifications_delivered_total",
			Help: "Total number of notifications successfully delivered",
		}, []string{"channel"}),

		NotificationsFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "notifications_failed_total",
			Help: "Total number of notification delivery failures",
		}, []string{"channel", "reason"}),

		NotificationsRetried: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "notifications_retried_total",
			Help: "Total number of notification retries",
		}, []string{"channel"}),

		DeliveryLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "notification_delivery_duration_seconds",
			Help:    "Time to deliver a notification via webhook",
			Buckets: prometheus.DefBuckets,
		}, []string{"channel"}),

		QueueDepth: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "notification_queue_depth",
			Help: "Current queue depth per channel",
		}, []string{"channel"}),

		HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),

		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"method", "path", "status"}),
	}
}
