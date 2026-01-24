#!/bin/bash

# ==============================================================================
# SCRIPT D'ARRÊT PROPRE DE L'APPLICATION KAFKA DEMO
# ==============================================================================
#
# Ce script est conçu pour arrêter proprement tous les composants de l'application.
# Il suit une approche en plusieurs étapes pour s'assurer que les données en
# transit sont traitées avant l'arrêt complet.
#
# Étapes exécutées :
# 1. Arrêt des processus Go :
#    a. Envoi d'un signal SIGTERM : Ce signal demande aux processus Go de
#       s'arrêter proprement. Le producteur videra son tampon et le
#       consommateur terminera de traiter le message en cours.
#    b. Période de grâce : Le script attend jusqu'à 10 secondes pour laisser
#       le temps aux applications de se terminer d'elles-mêmes.
#    c. Arrêt forcé (si nécessaire) : Si les processus sont toujours actifs
#       après le délai, un signal SIGKILL est envoyé pour les forcer à
#       s'arrêter. C'est une mesure de sécurité.
# 2. Arrêt des conteneurs Docker : Une fois les applications Go terminées,
#    `docker compose down` est appelé pour arrêter et supprimer les conteneurs
#    Kafka.
#
# ------------------------------------------------------------------------------

# Active le mode "verbose" pour afficher chaque commande.
set -x

# Obtenir le répertoire du script
script_dir=$(dirname "$0")

# Fonction pour arrêter un processus proprement par son PID
# Prend en paramètre le nom du service et son PID
shutdown_process() {
    local service_name=$1
    local pid=$2

    echo "   -> Arrêt de $service_name (PID: $pid)..."
    # Envoi du signal SIGTERM pour un arrêt gracieux
    kill -TERM $pid

    # Période de grâce de 15 secondes
    for i in {1..15}; do
        if ! kill -0 $pid 2>/dev/null; then
            echo "   ✅ $service_name s'est arrêté proprement."
            return 0
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    # Si le processus est toujours là, on force l'arrêt
    echo "   ⚠️  $service_name ne s'est pas arrêté à temps. Arrêt forcé (SIGKILL)..."
    kill -KILL $pid
    return 1
}

# Étape 1: Arrêter proprement les processus Go (producer PUIS tracker)
echo "🔴 Arrêt séquentiel des processus applicatifs Go..."

if [ -f "$script_dir/producer.pid" ] || [ -f "$script_dir/tracker.pid" ]; then
    producer_pid=""
    tracker_pid=""

    if [ -f "$script_dir/producer.pid" ]; then
        producer_pid=$(cat "$script_dir/producer.pid")
        if ! kill -0 $producer_pid 2>/dev/null; then
            echo "   ⚠️  Le producer (PID: $producer_pid) n'est plus actif."
            producer_pid=""
        fi
    fi

    if [ -f "$script_dir/tracker.pid" ]; then
        tracker_pid=$(cat "$script_dir/tracker.pid")
        if ! kill -0 $tracker_pid 2>/dev/null; then
            echo "   ⚠️  Le tracker (PID: $tracker_pid) n'est plus actif."
            tracker_pid=""
        fi
    fi

    # 1. Arrêter le producer d'abord pour stopper l'envoi de nouveaux messages
    if [ -n "$producer_pid" ]; then
        echo "   1. Arrêt du producer..."
        shutdown_process "Producer" $producer_pid
        echo ""
    fi

    # 2. Ensuite, arrêter le tracker pour qu'il traite les messages restants
    if [ -n "$tracker_pid" ]; then
        echo "   2. Arrêt du tracker..."
        shutdown_process "Tracker" $tracker_pid
        echo ""
    fi

    # Vérification finale que les processus sont bien arrêtés
    echo "   🔍 Vérification finale que tous les processus sont arrêtés..."
    sleep 1
    if [ -n "$producer_pid" ] && kill -0 $producer_pid 2>/dev/null; then
        echo "   ⚠️  Le producer est toujours actif, arrêt forcé..."
        kill -KILL $producer_pid 2>/dev/null || true
    fi
    if [ -n "$tracker_pid" ] && kill -0 $tracker_pid 2>/dev/null; then
        echo "   ⚠️  Le tracker est toujours actif, arrêt forcé..."
        kill -KILL $tracker_pid 2>/dev/null || true
    fi

    # Nettoyer les fichiers PID
    rm -f "$script_dir/producer.pid" "$script_dir/tracker.pid"
else
    echo "   ⚠️ Fichiers PID non trouvés. Tentative d'arrêt par pkill (moins fiable)..."
    pkill -TERM -f "go run.*kafka,producer.*cmd/producer/main.go" 2>/dev/null || true
    sleep 5 # Laisse un peu de temps au producer
    pkill -TERM -f "go run.*kafka,tracker.*cmd/tracker/main.go" 2>/dev/null || true
    sleep 2
    # Arrêt forcé si nécessaire
    pkill -KILL -f "go run.*kafka,producer.*cmd/producer/main.go" 2>/dev/null || true
    pkill -KILL -f "go run.*kafka,tracker.*cmd/tracker/main.go" 2>/dev/null || true
fi

# Vérification finale supplémentaire
echo "   🔍 Vérification finale supplémentaire..."
sleep 1
if pgrep -f "go run.*cmd/producer/main.go" >/dev/null 2>&1 || pgrep -f "go run.*cmd/tracker/main.go" >/dev/null 2>&1; then
    echo "   ⚠️  Certains processus Go sont encore actifs, arrêt forcé..."
    pkill -KILL -f "go run.*cmd/producer/main.go" 2>/dev/null || true
    pkill -KILL -f "go run.*cmd/tracker/main.go" 2>/dev/null || true
    sleep 1
fi

# Étape 2: Arrêter et supprimer les conteneurs Docker
echo "🔴 Arrêt et suppression des conteneurs Docker..."
sudo docker compose down

echo "✅ L'environnement a été complètement arrêté."
