#!/bin/bash
# ==============================================================================
# PubSubKafka - Script de création des topics Kafka
# ==============================================================================
# Ce script crée les topics Kafka nécessaires pour le projet.
#
# Usage:
#   ./scripts/create-topics.sh                      # Utilise localhost:9092
#   ./scripts/create-topics.sh kafka:9092           # Spécifie un broker
#   KAFKA_BROKER=kafka:9092 ./scripts/create-topics.sh
#
# Prérequis:
#   - Docker avec le conteneur Kafka en cours d'exécution
#   - OU kafka-topics disponible dans le PATH

set -euo pipefail

# ==============================================================================
# Configuration
# ==============================================================================
KAFKA_BROKER="${KAFKA_BROKER:-${1:-localhost:9092}}"
KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"

# Topics à créer
declare -A TOPICS=(
    ["orders"]="3:1"           # 3 partitions, factor 1
    ["orders-dlq"]="1:1"       # 1 partition, factor 1
    ["users"]="3:1"            # 3 partitions, factor 1
    ["users-dlq"]="1:1"        # 1 partition, factor 1
)

# Couleurs pour les logs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# ==============================================================================
# Fonctions
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

# Vérifie si le broker est accessible
check_broker() {
    log_info "Vérification de la connexion au broker: $KAFKA_BROKER"
    
    if docker exec "$KAFKA_CONTAINER" kafka-topics --bootstrap-server "$KAFKA_BROKER" --list >/dev/null 2>&1; then
        log_success "Connexion au broker réussie"
        return 0
    fi
    
    log_error "Impossible de se connecter au broker Kafka"
    return 1
}

# Crée un topic
create_topic() {
    local topic=$1
    local config=$2
    local partitions
    local replication
    
    IFS=':' read -r partitions replication <<< "$config"
    
    log_info "Création du topic: $topic (partitions: $partitions, replication: $replication)"
    
    if docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --create \
        --if-not-exists \
        --topic "$topic" \
        --partitions "$partitions" \
        --replication-factor "$replication" 2>/dev/null; then
        log_success "Topic '$topic' créé avec succès"
    else
        log_warning "Le topic '$topic' existe peut-être déjà"
    fi
}

# Liste tous les topics
list_topics() {
    log_info "Liste des topics existants:"
    docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --list
}

# Décrit un topic
describe_topic() {
    local topic=$1
    docker exec "$KAFKA_CONTAINER" kafka-topics \
        --bootstrap-server "$KAFKA_BROKER" \
        --describe \
        --topic "$topic"
}

# ==============================================================================
# Script principal
# ==============================================================================

main() {
    echo "============================================================"
    echo "  PubSubKafka - Création des topics Kafka"
    echo "============================================================"
    echo ""
    
    # Vérification de la connexion
    if ! check_broker; then
        exit 1
    fi
    
    echo ""
    
    # Création des topics
    for topic in "${!TOPICS[@]}"; do
        create_topic "$topic" "${TOPICS[$topic]}"
    done
    
    echo ""
    
    # Affichage des topics créés
    log_info "Vérification des topics créés..."
    echo ""
    list_topics
    
    echo ""
    log_success "Création des topics terminée!"
    echo ""
    
    # Description détaillée
    log_info "Détails des topics:"
    for topic in "${!TOPICS[@]}"; do
        echo ""
        echo "--- $topic ---"
        describe_topic "$topic"
    done
}

# Exécution
main "$@"
