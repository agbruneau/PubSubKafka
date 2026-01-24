/*
Package monitoring provides structured logging for Kafka operations.

This package offers a structured logger with:
- JSON output format for log aggregation
- Log levels (DEBUG, INFO, WARN, ERROR)
- Contextual fields
- Kafka-specific log helpers
*/
package monitoring

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String returns the string representation of the log level.
func (l LogLevel) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEntry represents a structured log entry.
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Service   string                 `json:"service"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// Logger provides structured logging capabilities.
type Logger struct {
	mu        sync.Mutex
	output    io.Writer
	level     LogLevel
	service   string
	fields    map[string]interface{}
}

// LoggerOption configures a Logger.
type LoggerOption func(*Logger)

// WithOutput sets the log output destination.
func WithOutput(w io.Writer) LoggerOption {
	return func(l *Logger) {
		l.output = w
	}
}

// WithLevel sets the minimum log level.
func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

// WithFields adds default fields to all log entries.
func WithFields(fields map[string]interface{}) LoggerOption {
	return func(l *Logger) {
		for k, v := range fields {
			l.fields[k] = v
		}
	}
}

// NewLogger creates a new structured logger.
func NewLogger(service string, opts ...LoggerOption) *Logger {
	l := &Logger{
		output:  os.Stdout,
		level:   LevelInfo,
		service: service,
		fields:  make(map[string]interface{}),
	}

	for _, opt := range opts {
		opt(l)
	}

	return l
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.log(LevelDebug, msg, keysAndValues...)
}

// Info logs an info message.
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.log(LevelInfo, msg, keysAndValues...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	l.log(LevelWarn, msg, keysAndValues...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	l.log(LevelError, msg, keysAndValues...)
}

// log writes a log entry at the specified level.
func (l *Logger) log(level LogLevel, msg string, keysAndValues ...interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Service:   l.service,
		Message:   msg,
		Fields:    make(map[string]interface{}),
	}

	// Copy default fields
	for k, v := range l.fields {
		entry.Fields[k] = v
	}

	// Parse key-value pairs
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key, ok := keysAndValues[i].(string)
			if !ok {
				key = fmt.Sprintf("%v", keysAndValues[i])
			}
			value := keysAndValues[i+1]

			if key == "error" {
				if err, ok := value.(error); ok {
					entry.Error = err.Error()
				} else {
					entry.Error = fmt.Sprintf("%v", value)
				}
			} else {
				entry.Fields[key] = value
			}
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(l.output, "ERROR: failed to marshal log entry: %v\n", err)
		return
	}

	fmt.Fprintln(l.output, string(data))
}

// With returns a new logger with additional default fields.
func (l *Logger) With(keysAndValues ...interface{}) *Logger {
	newLogger := &Logger{
		output:  l.output,
		level:   l.level,
		service: l.service,
		fields:  make(map[string]interface{}),
	}

	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	// Add new fields
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key, ok := keysAndValues[i].(string)
			if !ok {
				key = fmt.Sprintf("%v", keysAndValues[i])
			}
			newLogger.fields[key] = keysAndValues[i+1]
		}
	}

	return newLogger
}

// KafkaLogger provides Kafka-specific logging helpers.
type KafkaLogger struct {
	*Logger
}

// NewKafkaLogger creates a new Kafka-specific logger.
func NewKafkaLogger(service string, opts ...LoggerOption) *KafkaLogger {
	return &KafkaLogger{
		Logger: NewLogger(service, opts...),
	}
}

// LogMessageProduced logs a message production event.
func (l *KafkaLogger) LogMessageProduced(topic string, partition int32, offset int64, key string) {
	l.Info("Message produced",
		"topic", topic,
		"partition", partition,
		"offset", offset,
		"key", key,
	)
}

// LogMessageConsumed logs a message consumption event.
func (l *KafkaLogger) LogMessageConsumed(topic string, partition int32, offset int64, latency time.Duration) {
	l.Info("Message consumed",
		"topic", topic,
		"partition", partition,
		"offset", offset,
		"latency_ms", latency.Milliseconds(),
	)
}

// LogMessageProcessed logs a successful message processing event.
func (l *KafkaLogger) LogMessageProcessed(topic string, partition int32, offset int64, duration time.Duration) {
	l.Info("Message processed",
		"topic", topic,
		"partition", partition,
		"offset", offset,
		"duration_ms", duration.Milliseconds(),
	)
}

// LogMessageFailed logs a failed message processing event.
func (l *KafkaLogger) LogMessageFailed(topic string, partition int32, offset int64, err error) {
	l.Error("Message processing failed",
		"topic", topic,
		"partition", partition,
		"offset", offset,
		"error", err,
	)
}

// LogDLQSent logs a message sent to DLQ.
func (l *KafkaLogger) LogDLQSent(originalTopic string, dlqTopic string, reason string, err error) {
	l.Warn("Message sent to DLQ",
		"original_topic", originalTopic,
		"dlq_topic", dlqTopic,
		"reason", reason,
		"error", err,
	)
}

// LogRebalance logs a consumer group rebalance event.
func (l *KafkaLogger) LogRebalance(consumerGroup string, assignedPartitions []int32, revokedPartitions []int32) {
	l.Info("Consumer group rebalance",
		"consumer_group", consumerGroup,
		"assigned_partitions", assignedPartitions,
		"revoked_partitions", revokedPartitions,
	)
}

// LogConsumerLag logs consumer lag information.
func (l *KafkaLogger) LogConsumerLag(topic string, partition int32, consumerGroup string, lag int64) {
	l.Debug("Consumer lag",
		"topic", topic,
		"partition", partition,
		"consumer_group", consumerGroup,
		"lag", lag,
	)
}
