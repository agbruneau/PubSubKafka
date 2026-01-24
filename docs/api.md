# API Documentation - PubSubKafka

## Vue d'ensemble

Cette documentation décrit les interfaces publiques du système PubSubKafka, incluant les APIs HTTP, les formats de messages Kafka, et les interfaces Go.

---

## APIs HTTP

### Health Endpoints

#### GET /health

Vérifie l'état de santé du service.

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "version": "1.0.0",
  "checks": {
    "kafka": "connected",
    "database": "connected"
  }
}
```

**Status Codes:**
- `200 OK` - Service en bonne santé
- `503 Service Unavailable` - Service dégradé

#### GET /ready

Vérifie si le service est prêt à recevoir du trafic.

**Response:**

```json
{
  "ready": true,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Metrics Endpoint

#### GET /metrics

Expose les métriques Prometheus.

**Response:** Format Prometheus text

```
# HELP kafka_producer_messages_total Total messages produced
# TYPE kafka_producer_messages_total counter
kafka_producer_messages_total{topic="orders",status="success"} 12345

# HELP kafka_consumer_lag Consumer lag
# TYPE kafka_consumer_lag gauge
kafka_consumer_lag{topic="orders",partition="0"} 42
```

---

## Formats de Messages Kafka

### OrderEvent

Message principal pour les événements de commande.

**Topic:** `orders`

**Schema JSON:**

```json
{
  "order_id": "string (UUID)",
  "sequence": "integer",
  "status": "string (enum)",
  "items": [
    {
      "item_id": "string (UUID)",
      "item_name": "string",
      "quantity": "integer",
      "unit_price": "number",
      "total_price": "number"
    }
  ],
  "sub_total": "number",
  "tax": "number",
  "shipping_fee": "number",
  "total": "number",
  "currency": "string (ISO 4217)",
  "payment_method": "string",
  "shipping_address": "string",
  "metadata": {
    "timestamp": "string (ISO 8601)",
    "version": "string",
    "event_type": "string",
    "source": "string",
    "correlation_id": "string (UUID)"
  },
  "customer_info": {
    "customer_id": "string (UUID)",
    "name": "string",
    "email": "string",
    "phone": "string",
    "address": "string",
    "loyalty_level": "string"
  },
  "inventory_status": [
    {
      "item_id": "string",
      "item_name": "string",
      "available_qty": "integer",
      "reserved_qty": "integer",
      "unit_price": "number",
      "in_stock": "boolean",
      "warehouse": "string"
    }
  ]
}
```

**Exemple:**

```json
{
  "order_id": "550e8400-e29b-41d4-a716-446655440000",
  "sequence": 1234,
  "status": "pending",
  "items": [
    {
      "item_id": "item-001",
      "item_name": "Wireless Keyboard",
      "quantity": 2,
      "unit_price": 49.99,
      "total_price": 99.98
    }
  ],
  "sub_total": 99.98,
  "tax": 20.00,
  "shipping_fee": 5.99,
  "total": 125.97,
  "currency": "EUR",
  "payment_method": "credit_card",
  "shipping_address": "123 Main St, Paris, France",
  "metadata": {
    "timestamp": "2024-01-15T10:30:00Z",
    "version": "1.0",
    "event_type": "order.created",
    "source": "order-service",
    "correlation_id": "corr-123-456"
  },
  "customer_info": {
    "customer_id": "cust-001",
    "name": "Jean Dupont",
    "email": "jean.dupont@example.com",
    "phone": "+33612345678",
    "address": "123 Main St, Paris",
    "loyalty_level": "gold"
  },
  "inventory_status": [
    {
      "item_id": "item-001",
      "item_name": "Wireless Keyboard",
      "available_qty": 100,
      "reserved_qty": 2,
      "unit_price": 49.99,
      "in_stock": true,
      "warehouse": "PARIS-01"
    }
  ]
}
```

**Event Types:**
- `order.created` - Nouvelle commande
- `order.updated` - Commande modifiée
- `order.cancelled` - Commande annulée
- `order.shipped` - Commande expédiée
- `order.delivered` - Commande livrée

**Status Values:**
- `pending` - En attente
- `confirmed` - Confirmée
- `processing` - En cours de traitement
- `shipped` - Expédiée
- `delivered` - Livrée
- `cancelled` - Annulée
- `refunded` - Remboursée

### DeadLetterMessage

Message envoyé à la Dead Letter Queue en cas d'échec de traitement.

**Topic:** `orders-dlq`

**Schema JSON:**

```json
{
  "id": "string (UUID)",
  "timestamp": "string (ISO 8601)",
  "original_topic": "string",
  "original_partition": "integer",
  "original_offset": "integer",
  "original_key": "string",
  "original_payload": "string",
  "original_timestamp": "string (ISO 8601)",
  "failure_reason": "string (enum)",
  "error_message": "string",
  "error_stack": "string",
  "consumer_group": "string",
  "processing_host": "string",
  "retry_count": "integer",
  "first_failure": "string (ISO 8601)",
  "last_failure": "string (ISO 8601)",
  "next_retry_at": "string (ISO 8601)",
  "is_reprocessed": "boolean",
  "metadata": "object"
}
```

**Failure Reasons:**
- `DESERIALIZATION_ERROR` - Échec de parsing JSON
- `VALIDATION_ERROR` - Données invalides
- `PROCESSING_ERROR` - Erreur de traitement métier
- `TIMEOUT_ERROR` - Dépassement du timeout
- `UNKNOWN_ERROR` - Erreur inconnue

---

## Interfaces Go Publiques

### pkg/kafka

#### ProducerConfig

```go
type ProducerConfig struct {
    Broker            string
    Topic             string
    Interval          time.Duration
    FlushTimeout      time.Duration
    Acks              string
    Retries           int
    BatchSize         int
    LingerMs          int
    CompressionType   string
    EnableIdempotence bool
}
```

#### MessageProducer

```go
type MessageProducer interface {
    Produce(ctx context.Context, key string, value interface{}) error
    ProduceRaw(topic string, key, value []byte) error
    Flush(timeout time.Duration) error
    Close() error
    Metrics() ProducerMetrics
}
```

#### ConsumerConfig

```go
type ConsumerConfig struct {
    Broker             string
    Topic              string
    GroupID            string
    ReadTimeout        time.Duration
    AutoCommit         bool
    AutoCommitInterval time.Duration
    AutoOffsetReset    string
    MaxPollRecords     int
    SessionTimeout     time.Duration
    HeartbeatInterval  time.Duration
}
```

#### MessageConsumer

```go
type MessageConsumer interface {
    Start(ctx context.Context) error
    Commit(msg *Message) error
    Close() error
    Metrics() ConsumerMetrics
}
```

#### MessageHandler

```go
type MessageHandler func(ctx context.Context, msg *Message) error
```

#### Message

```go
type Message struct {
    Topic     string
    Partition int32
    Offset    int64
    Key       []byte
    Value     []byte
    Timestamp time.Time
    Headers   map[string]string
}
```

### pkg/dlq

#### Handler

```go
type Handler struct {
    // unexported fields
}

func NewHandler(config Config, producer Producer) *Handler
func (h *Handler) ProcessWithRetry(ctx context.Context, msg *MessageContext, process ProcessFunc) error
func (h *Handler) SendToDLQ(msg *MessageContext, reason FailureReason, errorMsg string) error
func (h *Handler) Flush() error
func (h *Handler) Close() error
func (h *Handler) Metrics() Metrics
```

### pkg/monitoring

#### Logger

```go
type Logger struct {
    // unexported fields
}

func NewLogger(service string, opts ...LoggerOption) *Logger
func (l *Logger) Debug(msg string, keysAndValues ...interface{})
func (l *Logger) Info(msg string, keysAndValues ...interface{})
func (l *Logger) Warn(msg string, keysAndValues ...interface{})
func (l *Logger) Error(msg string, keysAndValues ...interface{})
func (l *Logger) With(keysAndValues ...interface{}) *Logger
```

---

## Erreurs

### Codes d'Erreur Kafka

| Code | Erreur | Description |
|------|--------|-------------|
| `ERR_INVALID_BROKER` | ErrInvalidBroker | Adresse broker invalide |
| `ERR_INVALID_TOPIC` | ErrInvalidTopic | Nom de topic invalide |
| `ERR_CONNECTION_FAILED` | ErrConnectionFailed | Connexion échouée |
| `ERR_PRODUCER_CLOSED` | ErrProducerClosed | Producer fermé |
| `ERR_CONSUMER_CLOSED` | ErrConsumerClosed | Consumer fermé |
| `ERR_SERIALIZATION` | ErrSerializationFailed | Échec sérialisation |
| `ERR_DESERIALIZATION` | ErrDeserializationFailed | Échec désérialisation |
| `ERR_COMMIT` | ErrCommitFailed | Échec commit offset |

### Erreurs de Validation

| Erreur | Description |
|--------|-------------|
| `ErrEmptyOrderID` | order_id requis |
| `ErrInvalidSequence` | sequence doit être positif |
| `ErrEmptyStatus` | status requis |
| `ErrNoItems` | au moins un article requis |
| `ErrInvalidTotal` | total incohérent |
| `ErrEmptyCustomerID` | customer_id requis |
| `ErrInvalidEmail` | email invalide |

---

## Exemples d'Utilisation

### Producer

```go
package main

import (
    "context"
    "kafka-demo/pkg/kafka"
    "kafka-demo/pkg/models"
)

func main() {
    cfg := kafka.DefaultProducerConfig()
    cfg.Broker = "localhost:9092"
    cfg.Topic = "orders"

    producer, err := kafka.NewProducer(cfg)
    if err != nil {
        panic(err)
    }
    defer producer.Close()

    order := &models.Order{
        OrderID:  "order-123",
        Sequence: 1,
        Status:   "pending",
        // ... autres champs
    }

    ctx := context.Background()
    if err := producer.Produce(ctx, order.OrderID, order); err != nil {
        panic(err)
    }
}
```

### Consumer

```go
package main

import (
    "context"
    "encoding/json"
    "kafka-demo/pkg/kafka"
    "kafka-demo/pkg/models"
)

func main() {
    cfg := kafka.DefaultConsumerConfig()
    cfg.Broker = "localhost:9092"
    cfg.Topic = "orders"
    cfg.GroupID = "my-consumer-group"

    handler := func(ctx context.Context, msg *kafka.Message) error {
        var order models.Order
        if err := json.Unmarshal(msg.Value, &order); err != nil {
            return err
        }
        // Traiter la commande
        return nil
    }

    consumer, err := kafka.NewConsumer(cfg, handler)
    if err != nil {
        panic(err)
    }
    defer consumer.Close()

    ctx := context.Background()
    if err := consumer.Start(ctx); err != nil {
        panic(err)
    }
}
```

### Avec Middleware

```go
handler := kafka.Chain(
    kafka.WithLogging(logger),
    kafka.WithMetrics(metricsCollector),
    kafka.WithRetry(kafka.DefaultRetryConfig()),
    kafka.WithTimeout(30 * time.Second),
)(processMessage)

consumer, _ := kafka.NewConsumer(cfg, handler)
```
