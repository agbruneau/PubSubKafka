# 🏗️ Architecture & Design Patterns

This document details the architectural models and design choices implemented in this project.

## 🧩 Architecture Patterns

### 1. Event-Driven Architecture (EDA)

Induces total decoupling between components via asynchronous messaging.

- **Implementation**: Apache Kafka serves as the central event bus.
- **Benefit**: High availability, horizontal scalability, and simplified extensibility.

### 2. Event Carried State Transfer (ECST)

Each message is "self-contained," carrying the full state required for downstream processing.

- **Benefit**: Eliminates synchronous API calls to upstream services or databases, enhancing consumer autonomy.
- **Resource**: [order.go](file:///c:/Users/agbru/OneDrive/Documents/GitHub/PubSubKafka/order.go) defines the enriched data structure.

### 3. Dual-Stream Logging (Audit vs. Health)

Clear separation of concerns regarding system journaling.

- **Service Health Monitoring** (`tracker.log`): Technical metrics and system lifecycle events.
- **Business Audit Trail** (`tracker.events`): An immutable, high-fidelity journal of all business event flows.

### 4. Graceful Shutdown

Services intercept system interrupt signals (`SIGINT` / `SIGTERM`).

- **Mechanics**: Ensures Kafka buffers are flushed and file descriptors are safely closed before termination, preventing data loss.

### 5. Dead Letter Queue (DLQ) 💀

The Dead Letter Queue pattern captures messages that cannot be processed successfully after multiple retry attempts, routing them to a dedicated topic for later analysis and potential reprocessing.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Message   │────▶│  Consumer   │────▶│  Processing │
│   (Topic)   │     │             │     │             │
└─────────────┘     └─────────────┘     └──────┬──────┘
                                               │
                           ┌───────────────────┼───────────────────┐
                           │                   │                   │
                           ▼                   ▼                   ▼
                      ┌─────────┐         ┌─────────┐         ┌─────────┐
                      │ Success │         │  Retry  │         │   DLQ   │
                      │ (commit)│         │ (backoff)│        │ (route) │
                      └─────────┘         └─────────┘         └─────────┘
```

#### Implementation Details

- **Topic**: `orders-dlq` - Dedicated Kafka topic for failed messages
- **Max Retries**: 3 attempts with exponential backoff (1s → 2s → 4s → ... max 30s)
- **Rich Context**: Each DLQ message includes:
  - Original message payload and metadata (topic, partition, offset)
  - Categorized failure reason (`VALIDATION_ERROR`, `DESERIALIZATION_ERROR`, `PROCESSING_ERROR`, `TIMEOUT_ERROR`)
  - Retry count and timestamps (first/last failure)
  - Consumer group and host identification

#### Failure Categories

| Reason | Description | Retryable |
|--------|-------------|-----------|
| `DESERIALIZATION_ERROR` | JSON parsing failure | No |
| `VALIDATION_ERROR` | Business rule violation | No |
| `PROCESSING_ERROR` | Transient service failure | Yes |
| `TIMEOUT_ERROR` | Operation timeout | Yes |

#### Usage Example

```go
import "kafka-demo/pkg/dlq"

handler := dlq.NewHandler(dlq.DefaultConfig(), producer)
defer handler.Close()

err := handler.ProcessWithRetry(ctx, message, func(ctx context.Context, msg *dlq.MessageContext) error {
    // Process message...
    return processOrder(msg.Value)
})
// If processing fails after retries, message is automatically sent to DLQ
```

#### Benefits

- **Zero Data Loss**: Failed messages are never discarded
- **Debugging**: Rich failure context for root cause analysis
- **Reprocessing**: Messages can be replayed after fixes
- **Monitoring**: Track failure rates and patterns via metrics

- **Resource**: [pkg/dlq/dlq.go](pkg/dlq/dlq.go) - DLQ handler implementation
- **Resource**: [pkg/models/models.go](pkg/models/models.go) - `DeadLetterMessage` structure

## 🛠️ Infrastructure & DevOps

- **Kafka KRaft Mode**: Utilizes the modern KRaft protocol, removing the dependency on Zookeeper for a leaner infrastructure.
- **Go Build Tags**: Orchestrates multiple entry points and conditional logic via compilation tags (`producer`, `tracker`, `monitor`).
- **Automated Lifecycle**: [start.sh](file:///c:/Users/agbru/OneDrive/Documents/GitHub/PubSubKafka/start.sh) and [stop.sh](file:///c:/Users/agbru/OneDrive/Documents/GitHub/PubSubKafka/stop.sh) provide reliable, environment-aware orchestration.
