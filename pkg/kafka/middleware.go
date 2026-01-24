/*
Package kafka provides middleware components for message processing.

Middleware allows adding cross-cutting concerns like:
- Retry logic with exponential backoff
- Metrics collection
- Distributed tracing
- Logging
- Rate limiting
*/
package kafka

import (
	"context"
	"fmt"
	"time"
)

// Middleware wraps a MessageHandler to add additional functionality.
type Middleware func(MessageHandler) MessageHandler

// Chain combines multiple middleware into a single middleware.
func Chain(middlewares ...Middleware) Middleware {
	return func(handler MessageHandler) MessageHandler {
		for i := len(middlewares) - 1; i >= 0; i-- {
			handler = middlewares[i](handler)
		}
		return handler
	}
}

// RetryConfig configures the retry middleware.
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// DefaultRetryConfig returns a RetryConfig with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// WithRetry creates a middleware that retries failed operations.
func WithRetry(config RetryConfig) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			var lastErr error
			delay := config.BaseDelay

			for attempt := 0; attempt <= config.MaxRetries; attempt++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				lastErr = next(ctx, msg)
				if lastErr == nil {
					return nil
				}

				// Check if error is retryable
				if !IsRetryable(lastErr) {
					return lastErr
				}

				if attempt < config.MaxRetries {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(delay):
					}

					// Calculate next delay with exponential backoff
					delay = time.Duration(float64(delay) * config.Multiplier)
					if delay > config.MaxDelay {
						delay = config.MaxDelay
					}
				}
			}

			return fmt.Errorf("max retries exceeded: %w", lastErr)
		}
	}
}

// MetricsCollector is an interface for collecting middleware metrics.
type MetricsCollector interface {
	RecordProcessed(topic string, partition int32, duration time.Duration)
	RecordFailed(topic string, partition int32, err error)
	RecordRetry(topic string, partition int32, attempt int)
}

// WithMetrics creates a middleware that collects processing metrics.
func WithMetrics(collector MetricsCollector) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			start := time.Now()
			err := next(ctx, msg)
			duration := time.Since(start)

			if err != nil {
				collector.RecordFailed(msg.Topic, msg.Partition, err)
			} else {
				collector.RecordProcessed(msg.Topic, msg.Partition, duration)
			}

			return err
		}
	}
}

// Logger is an interface for logging middleware events.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// WithLogging creates a middleware that logs message processing.
func WithLogging(logger Logger) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			logger.Debug("Processing message",
				"topic", msg.Topic,
				"partition", msg.Partition,
				"offset", msg.Offset,
			)

			start := time.Now()
			err := next(ctx, msg)
			duration := time.Since(start)

			if err != nil {
				logger.Error("Message processing failed",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"duration", duration.String(),
					"error", err.Error(),
				)
			} else {
				logger.Info("Message processed",
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
					"duration", duration.String(),
				)
			}

			return err
		}
	}
}

// TracerContextKey is the context key for trace spans.
type TracerContextKey struct{}

// Tracer is an interface for distributed tracing.
type Tracer interface {
	StartSpan(ctx context.Context, operationName string) (context.Context, Span)
}

// Span represents a trace span.
type Span interface {
	SetTag(key string, value interface{})
	SetError(err error)
	Finish()
}

// WithTracing creates a middleware that adds distributed tracing.
func WithTracing(tracer Tracer) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			operationName := fmt.Sprintf("kafka.consume.%s", msg.Topic)
			ctx, span := tracer.StartSpan(ctx, operationName)
			defer span.Finish()

			span.SetTag("kafka.topic", msg.Topic)
			span.SetTag("kafka.partition", msg.Partition)
			span.SetTag("kafka.offset", msg.Offset)

			err := next(ctx, msg)
			if err != nil {
				span.SetError(err)
			}

			return err
		}
	}
}

// RateLimiter is an interface for rate limiting.
type RateLimiter interface {
	Wait(ctx context.Context) error
}

// WithRateLimit creates a middleware that applies rate limiting.
func WithRateLimit(limiter RateLimiter) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			if err := limiter.Wait(ctx); err != nil {
				return fmt.Errorf("rate limiter: %w", err)
			}
			return next(ctx, msg)
		}
	}
}

// TimeoutConfig configures the timeout middleware.
type TimeoutConfig struct {
	Timeout time.Duration
}

// WithTimeout creates a middleware that enforces processing timeouts.
func WithTimeout(timeout time.Duration) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) error {
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			done := make(chan error, 1)
			go func() {
				done <- next(ctx, msg)
			}()

			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return fmt.Errorf("processing timeout exceeded: %w", ctx.Err())
			}
		}
	}
}

// RecoveryHandler is called when a panic is recovered.
type RecoveryHandler func(msg *Message, recovered interface{})

// WithRecovery creates a middleware that recovers from panics.
func WithRecovery(handler RecoveryHandler) Middleware {
	return func(next MessageHandler) MessageHandler {
		return func(ctx context.Context, msg *Message) (err error) {
			defer func() {
				if r := recover(); r != nil {
					if handler != nil {
						handler(msg, r)
					}
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()
			return next(ctx, msg)
		}
	}
}
