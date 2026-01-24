/*
Package main provides the entry point for the Kafka Consumer service.

The consumer subscribes to order events from Kafka, processes them using
registered handlers, and manages offset commits. It supports graceful shutdown
and implements the Dead Letter Queue pattern for failed messages.

Usage:

	go run cmd/consumer/main.go [flags]

Flags:

	-broker   Kafka broker address (default: localhost:9092)
	-topic    Source topic name (default: orders)
	-group    Consumer group ID (default: order-tracker-group)
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kafka-demo/internal/handlers"
	"kafka-demo/pkg/dlq"
	"kafka-demo/pkg/kafka"
	"kafka-demo/pkg/models"
	"kafka-demo/pkg/monitoring"
)

func main() {
	// Parse command-line flags
	broker := flag.String("broker", models.DefaultKafkaBroker, "Kafka broker address")
	topic := flag.String("topic", models.DefaultTopic, "Source Kafka topic")
	group := flag.String("group", models.DefaultConsumerGroup, "Consumer group ID")
	flag.Parse()

	// Initialize structured logger
	logger := monitoring.NewLogger("consumer")
	logger.Info("Starting Kafka consumer",
		"broker", *broker,
		"topic", *topic,
		"group", *group,
	)

	// Create consumer configuration
	cfg := kafka.ConsumerConfig{
		Broker:        *broker,
		Topic:         *topic,
		GroupID:       *group,
		ReadTimeout:   models.TrackerConsumerReadTimeout,
		AutoCommit:    false, // Manual commit for at-least-once delivery
	}

	// Initialize DLQ handler
	dlqConfig := dlq.DefaultConfig()
	dlqConfig.Broker = *broker
	dlqConfig.ConsumerGroup = *group

	// Note: In production, inject a real Kafka producer
	dlqHandler := dlq.NewHandler(dlqConfig, nil)
	defer dlqHandler.Close()

	// Initialize order event handler
	orderHandler := handlers.NewOrderEventHandler(logger, dlqHandler)

	// Initialize Kafka consumer
	consumer, err := kafka.NewConsumer(cfg, orderHandler.Handle)
	if err != nil {
		logger.Error("Failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Received shutdown signal", "signal", sig.String())
		cancel()
	}()

	// Start consuming messages
	if err := consumer.Start(ctx); err != nil {
		logger.Error("Consumer error", "error", err)
		os.Exit(1)
	}

	fmt.Println("Consumer shutdown complete")
}
