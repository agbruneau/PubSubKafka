#!/bin/bash
# ==============================================================================
# PubSubKafka - Script de benchmark Kafka
# ==============================================================================
# Ce script effectue des tests de performance sur le cluster Kafka.
#
# Usage:
#   ./scripts/benchmark.sh producer           # Test de performance producer
#   ./scripts/benchmark.sh consumer           # Test de performance consumer
#   ./scripts/benchmark.sh end-to-end         # Test end-to-end latency
#   ./scripts/benchmark.sh all                # Tous les tests
#
# Options:
#   --messages N        Nombre de messages (défaut: 100000)
#   --size N            Taille des messages en bytes (défaut: 1024)
#   --throughput N      Débit cible en msg/s (défaut: -1 = max)
#   --topic NAME        Topic à utiliser (défaut: benchmark-test)

set -euo pipefail

# ==============================================================================
# Configuration
# ==============================================================================
KAFKA_BROKER="${KAFKA_BROKER:-localhost:9092}"
KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"

# Paramètres par défaut
NUM_MESSAGES=100000
MESSAGE_SIZE=1024
THROUGHPUT=-1
TOPIC="benchmark-test"
CONSUMER_GROUP="benchmark-consumer-group"

# Couleurs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# ==============================================================================
# Fonctions utilitaires
# ==============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_metric() {
    echo -e "${CYAN}[METRIC]${NC} $1"
}

print_separator() {
    echo "============================================================"
}

# Parse les arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --messages)
                NUM_MESSAGES="$2"
                shift 2
                ;;
            --size)
                MESSAGE_SIZE="$2"
                shift 2
                ;;
            --throughput)
                THROUGHPUT="$2"
                shift 2
                ;;
            --topic)
                TOPIC="$2"
                shift 2
                ;;
            producer|consumer|end-to-end|all|-h|--help)
                # Ces arguments sont traités dans main
                shift
                ;;
            *)
                log_error "Option inconnue: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Affiche l'aide
show_help() {
    cat << EOF
Usage: $0 <command> [options]

Commands:
    producer        Test de performance du producer
    consumer        Test de performance du consumer
    end-to-end      Test de latence end-to-end
    all             Exécute tous les tests

Options:
    --messages N    Nombre de messages (défaut: 100000)
    --size N        Taille des messages en bytes (défaut: 1024)
    --throughput N  Débit cible en msg/s (défaut: -1 = max)
    --topic NAME    Topic à utiliser (défaut: benchmark-test)
    -h, --help      Affiche cette aide

Variables d'environnement:
    KAFKA_BROKER        Adresse du broker (défaut: localhost:9092)
    KAFKA_CONTAINER     Nom du conteneur Docker (défaut: kafka)

Exemples:
    $0 producer --messages 50000 --size 2048
    $0 consumer --topic orders
    $0 all --throughput 10000
EOF
}

# ==============================================================================
# Préparation
# ==============================================================================

# Crée le topic de benchmark si nécessaire
setup_topic() {
    log_info "Création du topic de benchmark: $TOPIC"
    
    docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --create \
        --if-not-exists \
        --topic "$TOPIC" \
        --partitions 6 \
        --replication-factor 1 \
        --config retention.ms=600000 2>/dev/null || true
    
    log_success "Topic prêt"
}

# Nettoie le topic de benchmark
cleanup_topic() {
    log_info "Nettoyage du topic de benchmark..."
    
    docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --delete \
        --topic "$TOPIC" 2>/dev/null || true
}

# ==============================================================================
# Tests de benchmark
# ==============================================================================

# Test de performance du producer
benchmark_producer() {
    print_separator
    echo "  BENCHMARK PRODUCER"
    print_separator
    echo ""
    
    log_info "Configuration:"
    echo "  - Broker: $KAFKA_BROKER"
    echo "  - Topic: $TOPIC"
    echo "  - Messages: $NUM_MESSAGES"
    echo "  - Taille message: $MESSAGE_SIZE bytes"
    echo "  - Throughput cible: ${THROUGHPUT} msg/s"
    echo ""
    
    setup_topic
    
    log_info "Démarrage du benchmark producer..."
    echo ""
    
    docker exec "$KAFKA_CONTAINER" kafka-producer-perf-test \
        --topic "$TOPIC" \
        --num-records "$NUM_MESSAGES" \
        --record-size "$MESSAGE_SIZE" \
        --throughput "$THROUGHPUT" \
        --producer-props \
            bootstrap.servers="$KAFKA_BROKER" \
            acks=all \
            linger.ms=5 \
            batch.size=16384 \
            compression.type=snappy
    
    echo ""
    log_success "Benchmark producer terminé"
}

# Test de performance du consumer
benchmark_consumer() {
    print_separator
    echo "  BENCHMARK CONSUMER"
    print_separator
    echo ""
    
    log_info "Configuration:"
    echo "  - Broker: $KAFKA_BROKER"
    echo "  - Topic: $TOPIC"
    echo "  - Messages: $NUM_MESSAGES"
    echo "  - Consumer Group: $CONSUMER_GROUP"
    echo ""
    
    log_info "Démarrage du benchmark consumer..."
    echo ""
    
    docker exec "$KAFKA_CONTAINER" kafka-consumer-perf-test \
        --bootstrap-server "$KAFKA_BROKER" \
        --topic "$TOPIC" \
        --messages "$NUM_MESSAGES" \
        --group "$CONSUMER_GROUP" \
        --show-detailed-stats \
        --print-metrics
    
    echo ""
    log_success "Benchmark consumer terminé"
}

# Test de latence end-to-end
benchmark_e2e() {
    print_separator
    echo "  BENCHMARK END-TO-END LATENCY"
    print_separator
    echo ""
    
    log_info "Configuration:"
    echo "  - Broker: $KAFKA_BROKER"
    echo "  - Topic: $TOPIC"
    echo "  - Messages: 10000"
    echo "  - Taille message: $MESSAGE_SIZE bytes"
    echo ""
    
    setup_topic
    
    log_info "Démarrage du benchmark end-to-end..."
    log_warning "Ce test prend environ 30 secondes..."
    echo ""
    
    # Test simplifié avec timeout
    docker exec "$KAFKA_CONTAINER" timeout 60 kafka-producer-perf-test \
        --topic "$TOPIC" \
        --num-records 10000 \
        --record-size "$MESSAGE_SIZE" \
        --throughput 1000 \
        --producer-props \
            bootstrap.servers="$KAFKA_BROKER" \
            acks=all
    
    echo ""
    log_success "Benchmark end-to-end terminé"
}

# Affiche un résumé des métriques système
show_system_metrics() {
    print_separator
    echo "  MÉTRIQUES SYSTÈME"
    print_separator
    echo ""
    
    log_info "Statistiques du cluster Kafka:"
    echo ""
    
    # Nombre de topics
    local topic_count
    topic_count=$(docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --list 2>/dev/null | wc -l)
    log_metric "Nombre de topics: $topic_count"
    
    # Détails du topic de benchmark
    if docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --describe \
        --topic "$TOPIC" 2>/dev/null; then
        echo ""
    fi
    
    # Consumer groups
    log_info "Consumer groups actifs:"
    docker exec "$KAFKA_CONTAINER" kafka-consumer-groups \
        --bootstrap-server "$KAFKA_BROKER" \
        --list 2>/dev/null || true
}

# ==============================================================================
# Script principal
# ==============================================================================

main() {
    local command="${1:-help}"
    shift || true
    
    # Parse les arguments restants
    parse_args "$@"
    
    echo ""
    print_separator
    echo "  PubSubKafka - Benchmark Kafka"
    print_separator
    echo ""
    
    case "$command" in
        producer)
            benchmark_producer
            ;;
        consumer)
            benchmark_consumer
            ;;
        end-to-end|e2e)
            benchmark_e2e
            ;;
        all)
            benchmark_producer
            echo ""
            benchmark_consumer
            echo ""
            benchmark_e2e
            echo ""
            show_system_metrics
            ;;
        cleanup)
            cleanup_topic
            ;;
        -h|--help|help)
            show_help
            ;;
        *)
            log_error "Commande inconnue: $command"
            show_help
            exit 1
            ;;
    esac
    
    echo ""
    print_separator
    log_success "Benchmark terminé!"
    print_separator
}

main "$@"
