/*
Package testutils provides testing utilities for Kafka integration tests.

This package includes helpers for:
- Spinning up Kafka containers using testcontainers
- Creating test topics
- Producing and consuming test messages
- Cleaning up test resources
*/
package testutils

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// KafkaContainer represents a Kafka container for testing.
type KafkaContainer struct {
	mu            sync.Mutex
	brokerAddress string
	started       bool
	topics        []string
}

// KafkaContainerConfig configures the Kafka container.
type KafkaContainerConfig struct {
	// Image is the Docker image to use.
	Image string
	// Port is the Kafka broker port.
	Port int
	// StartupTimeout is the maximum time to wait for startup.
	StartupTimeout time.Duration
	// Topics is the list of topics to create on startup.
	Topics []TopicConfig
}

// TopicConfig configures a Kafka topic.
type TopicConfig struct {
	Name              string
	Partitions        int
	ReplicationFactor int
	Config            map[string]string
}

// DefaultKafkaContainerConfig returns default container configuration.
func DefaultKafkaContainerConfig() KafkaContainerConfig {
	return KafkaContainerConfig{
		Image:          "confluentinc/cp-kafka:7.5.0",
		Port:           9092,
		StartupTimeout: 60 * time.Second,
		Topics: []TopicConfig{
			{
				Name:              "orders",
				Partitions:        3,
				ReplicationFactor: 1,
			},
			{
				Name:              "orders-dlq",
				Partitions:        1,
				ReplicationFactor: 1,
			},
		},
	}
}

// NewKafkaContainer creates a new Kafka container for testing.
// Note: This is a mock implementation. In production, use testcontainers-go.
func NewKafkaContainer(config KafkaContainerConfig) *KafkaContainer {
	return &KafkaContainer{
		brokerAddress: fmt.Sprintf("localhost:%d", config.Port),
		topics:        make([]string, 0),
	}
}

// Start starts the Kafka container.
// Note: This is a mock implementation for demonstration.
func (c *KafkaContainer) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.started {
		return nil
	}

	// In a real implementation, this would use testcontainers-go:
	// container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
	//     ContainerRequest: testcontainers.ContainerRequest{
	//         Image: "confluentinc/cp-kafka:7.5.0",
	//         ...
	//     },
	//     Started: true,
	// })

	c.started = true
	return nil
}

// Stop stops and removes the Kafka container.
func (c *KafkaContainer) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return nil
	}

	c.started = false
	return nil
}

// BrokerAddress returns the broker address for the container.
func (c *KafkaContainer) BrokerAddress() string {
	return c.brokerAddress
}

// CreateTopic creates a topic in the container.
func (c *KafkaContainer) CreateTopic(ctx context.Context, config TopicConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return fmt.Errorf("container not started")
	}

	c.topics = append(c.topics, config.Name)
	return nil
}

// DeleteTopic deletes a topic from the container.
func (c *KafkaContainer) DeleteTopic(ctx context.Context, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.started {
		return fmt.Errorf("container not started")
	}

	for i, topic := range c.topics {
		if topic == name {
			c.topics = append(c.topics[:i], c.topics[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("topic not found: %s", name)
}

// ListTopics returns all topics in the container.
func (c *KafkaContainer) ListTopics() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make([]string, len(c.topics))
	copy(result, c.topics)
	return result
}

// IsRunning returns true if the container is running.
func (c *KafkaContainer) IsRunning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.started
}

// TestEnvironment provides a complete test environment with Kafka.
type TestEnvironment struct {
	Container *KafkaContainer
	Config    KafkaContainerConfig
}

// NewTestEnvironment creates a new test environment.
func NewTestEnvironment(config KafkaContainerConfig) *TestEnvironment {
	return &TestEnvironment{
		Container: NewKafkaContainer(config),
		Config:    config,
	}
}

// Setup starts the test environment.
func (e *TestEnvironment) Setup(ctx context.Context) error {
	if err := e.Container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Create configured topics
	for _, topic := range e.Config.Topics {
		if err := e.Container.CreateTopic(ctx, topic); err != nil {
			return fmt.Errorf("failed to create topic %s: %w", topic.Name, err)
		}
	}

	return nil
}

// Teardown stops the test environment.
func (e *TestEnvironment) Teardown(ctx context.Context) error {
	return e.Container.Stop(ctx)
}

// BrokerAddress returns the broker address for the test environment.
func (e *TestEnvironment) BrokerAddress() string {
	return e.Container.BrokerAddress()
}
