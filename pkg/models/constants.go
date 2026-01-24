/*
Package main centralizes all configuration constants shared across the
project components (producer, tracker, log_monitor).
*/

package models

import "time"

// Configuration Kafka par défaut
const (
	DefaultKafkaBroker   = "localhost:9092"
	DefaultConsumerGroup = "order-tracker-group"
	DefaultTopic         = "orders"
)

// Dead Letter Queue (DLQ) configuration
const (
	// DLQTopic is the Kafka topic for failed messages
	DLQTopic = "orders-dlq"
	// DLQMaxRetries is the maximum number of processing attempts before sending to DLQ
	DLQMaxRetries = 3
	// DLQRetryBaseDelay is the base delay for exponential backoff (doubles each retry)
	DLQRetryBaseDelay = 1 * time.Second
	// DLQRetryMaxDelay is the maximum delay between retries
	DLQRetryMaxDelay = 30 * time.Second
	// DLQFlushTimeout is the timeout for flushing DLQ producer
	DLQFlushTimeout = 5 * time.Second
)

// Fichiers de logs
const (
	TrackerLogFile    = "tracker.log"
	TrackerEventsFile = "tracker.events"
)

// Timeouts et intervalles communs
const (
	FlushTimeoutMs = 15000
)

// Constantes pour le producteur
const (
	ProducerMessageInterval     = 2 * time.Second
	ProducerDeliveryChannelSize = 10000
	ProducerDefaultTaxRate      = 0.20
	ProducerDefaultShippingFee  = 2.50
	ProducerDefaultCurrency     = "EUR"
	ProducerDefaultPayment      = "credit_card"
	ProducerDefaultWarehouse    = "PARIS-01"
)

// Constantes pour le tracker
const (
	TrackerMetricsInterval      = 30 * time.Second
	TrackerConsumerReadTimeout  = 1 * time.Second
	TrackerMaxConsecutiveErrors = 3
	TrackerServiceName          = "order-tracker"
)

// Constantes pour le moniteur de logs
const (
	MonitorMaxRecentLogs      = 20
	MonitorMaxRecentEvents    = 20
	MonitorMaxHistorySize     = 50
	MonitorLogChannelBuffer   = 100
	MonitorEventChannelBuffer = 100

	// Seuils de santé pour le taux de succès (%)
	MonitorSuccessRateExcellent = 95.0
	MonitorSuccessRateGood      = 80.0

	// Seuils de débit (messages par seconde)
	MonitorThroughputNormal = 0.3
	MonitorThroughputLow    = 0.1

	// Seuils de temps pour les erreurs
	MonitorErrorTimeoutCritical = 1 * time.Minute
	MonitorErrorTimeoutWarning  = 5 * time.Minute

	// Seuils de débit pour le score de qualité
	MonitorQualityThroughputHigh   = 0.5
	MonitorQualityThroughputMedium = 0.3
	MonitorQualityThroughputLow    = 0.1

	// Seuils de score de qualité
	MonitorQualityScoreExcellent = 90.0
	MonitorQualityScoreGood      = 70.0
	MonitorQualityScoreMedium    = 50.0

	// Intervalles de temps
	MonitorFileCheckInterval = 1 * time.Second
	MonitorFilePollInterval  = 200 * time.Millisecond
	MonitorUIUpdateInterval  = 500 * time.Millisecond

	// Limites d'affichage
	MonitorMaxLogRowLength   = 75
	MonitorMaxEventRowLength = 75
	MonitorTruncateSuffix    = "..."
)
