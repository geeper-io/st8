package st8metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	httpRequestsTotal     *prometheus.CounterVec
	httpRequestDuration   *prometheus.HistogramVec
	operationsTotal       *prometheus.CounterVec
	operationDuration     *prometheus.HistogramVec
	changedDocumentsTotal *prometheus.CounterVec
}

func New(reg prometheus.Registerer) *Collector {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "st8_http_requests_total",
			Help: "Total HTTP requests handled by st8d.",
		},
		[]string{"route", "method", "status"},
	)
	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "st8_http_request_duration_seconds",
			Help:    "HTTP request duration for st8d handlers.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"route", "method"},
	)
	operationsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "st8_operations_total",
			Help: "Total st8 business operations by operation name and result.",
		},
		[]string{"operation", "result"},
	)
	operationDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "st8_operation_duration_seconds",
			Help:    "Duration of st8 business operations.",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)
	changedDocumentsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "st8_changed_documents_total",
			Help: "Total changed documents produced by st8 mutation operations.",
		},
		[]string{"operation"},
	)

	reg.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		operationsTotal,
		operationDuration,
		changedDocumentsTotal,
	)

	return &Collector{
		httpRequestsTotal:     httpRequestsTotal,
		httpRequestDuration:   httpRequestDuration,
		operationsTotal:       operationsTotal,
		operationDuration:     operationDuration,
		changedDocumentsTotal: changedDocumentsTotal,
	}
}

func (c *Collector) ObserveHTTPRequest(route, method string, statusCode int, duration time.Duration) {
	if c == nil {
		return
	}
	status := strconv.Itoa(statusCode)
	c.httpRequestsTotal.WithLabelValues(route, method, status).Inc()
	c.httpRequestDuration.WithLabelValues(route, method).Observe(duration.Seconds())
}

func (c *Collector) ObserveOperation(operation, result string, duration time.Duration) {
	if c == nil {
		return
	}
	c.operationsTotal.WithLabelValues(operation, result).Inc()
	c.operationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

func (c *Collector) AddChangedDocuments(operation string, count int) {
	if c == nil || count <= 0 {
		return
	}
	c.changedDocumentsTotal.WithLabelValues(operation).Add(float64(count))
}
