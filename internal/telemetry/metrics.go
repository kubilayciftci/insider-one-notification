package telemetry

import (
	"sync"

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

var (
	metricsOnce     sync.Once
	metricsInstance *Metrics
)

func NewMetrics() *Metrics {
	metricsOnce.Do(func() {
		metricsInstance = &Metrics{
			NotificationsCreated: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "notifications_created_total",
			}, []string{"channel", "priority"}),

			NotificationsDelivered: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "notifications_delivered_total",
			}, []string{"channel"}),

			NotificationsFailed: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "notifications_failed_total",
			}, []string{"channel", "reason"}),

			NotificationsRetried: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "notifications_retried_total",
			}, []string{"channel"}),

			DeliveryLatency: promauto.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "notification_delivery_duration_seconds",
				Buckets: prometheus.DefBuckets,
			}, []string{"channel"}),

			QueueDepth: promauto.NewGaugeVec(prometheus.GaugeOpts{
				Name: "notification_queue_depth",
			}, []string{"channel"}),

			HTTPRequestDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Buckets: prometheus.DefBuckets,
			}, []string{"method", "path", "status"}),

			HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
				Name: "http_requests_total",
			}, []string{"method", "path", "status"}),
		}
	})
	return metricsInstance
}
