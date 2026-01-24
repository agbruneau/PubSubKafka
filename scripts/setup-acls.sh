#!/bin/bash
# ==============================================================================
# PubSubKafka - Script de configuration des ACLs Kafka
# ==============================================================================
# Ce script configure les Access Control Lists (ACLs) pour sécuriser 
# l'accès aux topics Kafka.
#
# Usage:
#   ./scripts/setup-acls.sh                         # Mode interactif
#   ./scripts/setup-acls.sh --apply                 # Applique les ACLs
#   ./scripts/setup-acls.sh --list                  # Liste les ACLs
#   ./scripts/setup-acls.sh --remove                # Supprime les ACLs
#
# Prérequis:
#   - Kafka avec autorizer configuré
#   - kafka-acls disponible
#
# Note: Ce script nécessite que Kafka soit configuré avec un authorizer.
#       Exemple dans server.properties:
#       authorizer.class.name=kafka.security.authorizer.AclAuthorizer
#       super.users=User:admin

set -euo pipefail

# ==============================================================================
# Configuration
# ==============================================================================
KAFKA_BROKER="${KAFKA_BROKER:-localhost:9092}"
KAFKA_CONTAINER="${KAFKA_CONTAINER:-kafka}"
COMMAND_CONFIG="${COMMAND_CONFIG:-}"

# Utilisateurs et groupes
PRODUCER_USER="${PRODUCER_USER:-producer-service}"
CONSUMER_USER="${CONSUMER_USER:-consumer-service}"
ADMIN_USER="${ADMIN_USER:-admin}"
CONSUMER_GROUP="${CONSUMER_GROUP:-order-tracker-group}"

# Topics
TOPICS=("orders" "orders-dlq" "users" "users-dlq")

# Couleurs
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

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

# Exécute une commande kafka-acls
run_kafka_acls() {
    local args=("$@")
    
    if [ -n "$COMMAND_CONFIG" ]; then
        docker exec "$KAFKA_CONTAINER" kafka-acls \
            --bootstrap-server "$KAFKA_BROKER" \
            --command-config "$COMMAND_CONFIG" \
            "${args[@]}"
    else
        docker exec "$KAFKA_CONTAINER" kafka-acls \
            --bootstrap-server "$KAFKA_BROKER" \
            "${args[@]}"
    fi
}

# Crée les ACLs pour le producer
setup_producer_acls() {
    log_info "Configuration des ACLs pour le producer ($PRODUCER_USER)..."
    
    for topic in "${TOPICS[@]}"; do
        # Permission WRITE sur le topic
        run_kafka_acls --add \
            --allow-principal "User:$PRODUCER_USER" \
            --operation Write \
            --operation Describe \
            --topic "$topic" 2>/dev/null || true
        
        log_success "ACL WRITE ajoutée pour $PRODUCER_USER sur $topic"
    done
    
    # Permission IDEMPOTENT_WRITE pour le producer idempotent
    run_kafka_acls --add \
        --allow-principal "User:$PRODUCER_USER" \
        --operation IdempotentWrite \
        --cluster 2>/dev/null || true
    
    log_success "ACL IDEMPOTENT_WRITE ajoutée pour $PRODUCER_USER"
}

# Crée les ACLs pour le consumer
setup_consumer_acls() {
    log_info "Configuration des ACLs pour le consumer ($CONSUMER_USER)..."
    
    for topic in "${TOPICS[@]}"; do
        # Permission READ sur le topic
        run_kafka_acls --add \
            --allow-principal "User:$CONSUMER_USER" \
            --operation Read \
            --operation Describe \
            --topic "$topic" 2>/dev/null || true
        
        log_success "ACL READ ajoutée pour $CONSUMER_USER sur $topic"
    done
    
    # Permission sur le consumer group
    run_kafka_acls --add \
        --allow-principal "User:$CONSUMER_USER" \
        --operation Read \
        --operation Describe \
        --group "$CONSUMER_GROUP" 2>/dev/null || true
    
    log_success "ACL ajoutée pour le groupe $CONSUMER_GROUP"
    
    # Permission WRITE sur le DLQ (pour envoyer les messages échoués)
    run_kafka_acls --add \
        --allow-principal "User:$CONSUMER_USER" \
        --operation Write \
        --topic "orders-dlq" 2>/dev/null || true
    
    run_kafka_acls --add \
        --allow-principal "User:$CONSUMER_USER" \
        --operation Write \
        --topic "users-dlq" 2>/dev/null || true
    
    log_success "ACL WRITE sur DLQ ajoutée pour $CONSUMER_USER"
}

# Crée les ACLs pour l'admin
setup_admin_acls() {
    log_info "Configuration des ACLs pour l'admin ($ADMIN_USER)..."
    
    # Permissions cluster
    run_kafka_acls --add \
        --allow-principal "User:$ADMIN_USER" \
        --operation All \
        --cluster 2>/dev/null || true
    
    # Permissions sur tous les topics
    run_kafka_acls --add \
        --allow-principal "User:$ADMIN_USER" \
        --operation All \
        --topic "*" 2>/dev/null || true
    
    # Permissions sur tous les groupes
    run_kafka_acls --add \
        --allow-principal "User:$ADMIN_USER" \
        --operation All \
        --group "*" 2>/dev/null || true
    
    log_success "ACLs admin configurées"
}

# Liste toutes les ACLs
list_acls() {
    log_info "Liste des ACLs configurées:"
    echo ""
    run_kafka_acls --list
}

# Supprime toutes les ACLs pour un utilisateur
remove_user_acls() {
    local user=$1
    log_info "Suppression des ACLs pour $user..."
    
    for topic in "${TOPICS[@]}"; do
        run_kafka_acls --remove \
            --allow-principal "User:$user" \
            --operation All \
            --topic "$topic" \
            --force 2>/dev/null || true
    done
    
    run_kafka_acls --remove \
        --allow-principal "User:$user" \
        --operation All \
        --group "$CONSUMER_GROUP" \
        --force 2>/dev/null || true
    
    log_success "ACLs supprimées pour $user"
}

# Affiche l'aide
show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Options:
    --apply         Applique les ACLs pour producer, consumer et admin
    --list          Liste toutes les ACLs configurées
    --remove        Supprime toutes les ACLs
    --producer      Configure uniquement les ACLs du producer
    --consumer      Configure uniquement les ACLs du consumer
    --admin         Configure uniquement les ACLs de l'admin
    -h, --help      Affiche cette aide

Variables d'environnement:
    KAFKA_BROKER        Adresse du broker (défaut: localhost:9092)
    KAFKA_CONTAINER     Nom du conteneur Docker (défaut: kafka)
    PRODUCER_USER       Nom de l'utilisateur producer (défaut: producer-service)
    CONSUMER_USER       Nom de l'utilisateur consumer (défaut: consumer-service)
    ADMIN_USER          Nom de l'utilisateur admin (défaut: admin)
    CONSUMER_GROUP      Nom du groupe consumer (défaut: order-tracker-group)
    COMMAND_CONFIG      Fichier de configuration pour l'authentification

Exemples:
    $0 --apply
    PRODUCER_USER=my-producer $0 --producer
    $0 --list
EOF
}

# ==============================================================================
# Script principal
# ==============================================================================

main() {
    echo "============================================================"
    echo "  PubSubKafka - Configuration des ACLs Kafka"
    echo "============================================================"
    echo ""
    
    local action="${1:---help}"
    
    case "$action" in
        --apply)
            setup_producer_acls
            echo ""
            setup_consumer_acls
            echo ""
            setup_admin_acls
            echo ""
            log_success "Configuration des ACLs terminée!"
            echo ""
            list_acls
            ;;
        --list)
            list_acls
            ;;
        --remove)
            log_warning "Suppression de toutes les ACLs..."
            remove_user_acls "$PRODUCER_USER"
            remove_user_acls "$CONSUMER_USER"
            remove_user_acls "$ADMIN_USER"
            log_success "ACLs supprimées"
            ;;
        --producer)
            setup_producer_acls
            ;;
        --consumer)
            setup_consumer_acls
            ;;
        --admin)
            setup_admin_acls
            ;;
        -h|--help)
            show_help
            ;;
        *)
            log_error "Option inconnue: $action"
            show_help
            exit 1
            ;;
    esac
}

main "$@"
