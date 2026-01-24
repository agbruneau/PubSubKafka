/*
Package monitoring provides Prometheus metrics for Kafka operations.

This package exposes standard metrics for:
- Message production and consumption rates
- Processing latencies
- Error rates
- Consumer lag
- DLQ statistics
*/
package monitoring

import (
	"sync"
	"time"
)

// MetricType defines the type of metric.
type MetricType string

const (
	MetricTypeCounter   MetricType = "counter"
	MetricTypeGauge     MetricType = "gauge"
	MetricTypeHistogram MetricType = "histogram"
)

// Metric represents a single metric with labels.
type Metric struct {
	Name        string
	Help        string
	Type        MetricType
	Labels      []string
	Buckets     []float64 // For histograms
}

// StandardMetrics defines the standard metrics for Kafka operations.
var StandardMetrics = struct {
	// Producer metrics
	MessagesProduced    Metric
	ProduceLatency      Metric
	ProduceErrors       Metric
	BytesProduced       Metric

	// Consumer metrics
	MessagesConsumed    Metric
	ConsumeLatency      Metric
	ConsumeErrors       Metric
	BytesConsumed       Metric
	ConsumerLag         Metric
	RebalanceTotal      Metric

	// DLQ metrics
	DLQMessagesSent     Metric
	DLQMessagesRetried  Metric
	DLQMessagesDropped  Metric

	// Processing metrics
	ProcessingDuration  Metric
	BatchSize           Metric
}{
	MessagesProduced: Metric{
		Name:   "kafka_producer_messages_total",
		Help:   "Total number of messages produced",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "status"},
	},
	ProduceLatency: Metric{
		Name:    "kafka_producer_latency_seconds",
		Help:    "Message production latency in seconds",
		Type:    MetricTypeHistogram,
		Labels:  []string{"topic"},
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	},
	ProduceErrors: Metric{
		Name:   "kafka_producer_errors_total",
		Help:   "Total number of production errors",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "error_type"},
	},
	BytesProduced: Metric{
		Name:   "kafka_producer_bytes_total",
		Help:   "Total bytes produced",
		Type:   MetricTypeCounter,
		Labels: []string{"topic"},
	},
	MessagesConsumed: Metric{
		Name:   "kafka_consumer_messages_total",
		Help:   "Total number of messages consumed",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "partition", "consumer_group"},
	},
	ConsumeLatency: Metric{
		Name:    "kafka_consumer_latency_seconds",
		Help:    "Message consumption latency in seconds",
		Type:    MetricTypeHistogram,
		Labels:  []string{"topic", "consumer_group"},
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	},
	ConsumeErrors: Metric{
		Name:   "kafka_consumer_errors_total",
		Help:   "Total number of consumption errors",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "consumer_group", "error_type"},
	},
	BytesConsumed: Metric{
		Name:   "kafka_consumer_bytes_total",
		Help:   "Total bytes consumed",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "consumer_group"},
	},
	ConsumerLag: Metric{
		Name:   "kafka_consumer_lag",
		Help:   "Consumer lag (messages behind)",
		Type:   MetricTypeGauge,
		Labels: []string{"topic", "partition", "consumer_group"},
	},
	RebalanceTotal: Metric{
		Name:   "kafka_consumer_rebalances_total",
		Help:   "Total number of consumer group rebalances",
		Type:   MetricTypeCounter,
		Labels: []string{"consumer_group"},
	},
	DLQMessagesSent: Metric{
		Name:   "kafka_dlq_messages_sent_total",
		Help:   "Total messages sent to DLQ",
		Type:   MetricTypeCounter,
		Labels: []string{"topic", "failure_reason"},
	},
	DLQMessagesRetried: Metric{
		Name:   "kafka_dlq_messages_retried_total",
		Help:   "Total DLQ messages retried",
		Type:   MetricTypeCounter,
		Labels: []string{"topic"},
	},
	DLQMessagesDropped: Metric{
		Name:   "kafka_dlq_messages_dropped_total",
		Help:   "Total DLQ messages dropped after max retries",
		Type:   MetricTypeCounter,
		Labels: []string{"topic"},
	},
	ProcessingDuration: Metric{
		Name:    "kafka_message_processing_duration_seconds",
		Help:    "Message processing duration in seconds",
		Type:    MetricTypeHistogram,
		Labels:  []string{"topic", "handler"},
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	},
	BatchSize: Metric{
		Name:   "kafka_batch_size",
		Help:   "Number of messages in a batch",
		Type:   MetricTypeGauge,
		Labels: []string{"topic"},
	},
}

// MetricsCollector provides an in-memory metrics collection.
// For production use, integrate with Prometheus client library.
type MetricsCollector struct {
	mu       sync.RWMutex
	counters map[string]float64
	gauges   map[string]float64
	histograms map[string]*HistogramData
}

// HistogramData stores histogram observations.
type HistogramData struct {
	Count   uint64
	Sum     float64
	Buckets map[float64]uint64
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		counters:   make(map[string]float64),
		gauges:     make(map[string]float64),
		histograms: make(map[string]*HistogramData),
	}
}

// RecordProcessed implements kafka.MetricsCollector.
func (m *MetricsCollector) RecordProcessed(topic string, partition int32, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := topic + "_processed"
	m.counters[key]++
	
	histKey := topic + "_duration"
	if _, ok := m.histograms[histKey]; !ok {
		m.histograms[histKey] = &HistogramData{
			Buckets: make(map[float64]uint64),
		}
	}
	m.histograms[histKey].Count++
	m.histograms[histKey].Sum += duration.Seconds()
}

// RecordFailed implements kafka.MetricsCollector.
func (m *MetricsCollector) RecordFailed(topic string, partition int32, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := topic + "_failed"
	m.counters[key]++
}

// RecordRetry implements kafka.MetricsCollector.
func (m *MetricsCollector) RecordRetry(topic string, partition int32, attempt int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := topic + "_retry"
	m.counters[key]++
}

// IncrementCounter increments a counter metric.
func (m *MetricsCollector) IncrementCounter(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name] += value
}

// SetGauge sets a gauge metric value.
func (m *MetricsCollector) SetGauge(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[name] = value
}

// ObserveHistogram records a histogram observation.
func (m *MetricsCollector) ObserveHistogram(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.histograms[name]; !ok {
		m.histograms[name] = &HistogramData{
			Buckets: make(map[float64]uint64),
		}
	}
	m.histograms[name].Count++
	m.histograms[name].Sum += value
}

// GetCounter returns the current value of a counter.
func (m *MetricsCollector) GetCounter(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.counters[name]
}

// GetGauge returns the current value of a gauge.
func (m *MetricsCollector) GetGauge(name string) float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.gauges[name]
}

// Snapshot returns a copy of all metrics.
func (m *MetricsCollector) Snapshot() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]interface{})
	
	for k, v := range m.counters {
		result["counter_"+k] = v
	}
	for k, v := range m.gauges {
		result["gauge_"+k] = v
	}
	for k, v := range m.histograms {
		result["histogram_"+k] = map[string]interface{}{
			"count": v.Count,
			"sum":   v.Sum,
		}
	}

	return result
}
