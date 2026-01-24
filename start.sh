#!/bin/bash

# ==============================================================================
# SCRIPT DE DÉMARRAGE DE L'APPLICATION KAFKA DEMO
# ==============================================================================
#
# Ce script orchestre le démarrage complet de l'environnement de démonstration.
# Il exécute les étapes suivantes dans un ordre précis pour garantir que
# tous les composants sont prêts et connectés correctement.
#
# Étapes exécutées :
# 1. Démarrage des conteneurs Docker : Lance le service Kafka en arrière-plan
#    en utilisant la configuration de `docker-compose.yaml`.
# 2. Pause d'initialisation : Attend un temps défini (30 secondes) pour
#    s'assurer que le broker Kafka est entièrement initialisé et prêt à
#    accepter des connexions et des commandes.
# 3. Création du topic Kafka : Crée le topic 'orders', qui est le canal de
#    communication entre le producteur et le consommateur.
# 4. Installation des dépendances Go : Exécute `go mod download` pour
#    télécharger les bibliothèques nécessaires (client Kafka, UUID).
# 5. Lancement du consommateur (`tracker`) : Démarre le consommateur en
#    arrière-plan. Il commencera immédiatement à écouter les messages
#    sur le topic 'orders'.
# 6. Lancement du producteur (`producer`) : Démarre le producteur au
#    premier plan. Il commencera à générer et envoyer des messages.
#    Le script se terminera lorsque le producteur sera arrêté (Ctrl+C).
#
# Note : Le moniteur de logs (`log_monitor.go`) doit être lancé manuellement
#        dans un terminal séparé avec la commande : go run log_monitor.go
#
# ------------------------------------------------------------------------------

# Active le mode "verbose" pour afficher chaque commande avant son exécution.
# Utile pour le débogage.
set -x

# Configure le script pour qu'il s'arrête immédiatement en cas d'erreur.
# -e : quitte si une commande se termine avec un statut non nul.
# -o pipefail : quitte si une commande dans un pipeline échoue.
set -e
set -o pipefail

# Obtenir le répertoire du script
script_dir=$(dirname "$0")

# Étape 1: Démarrage des conteneurs Docker
echo "🚀 Démarrage des conteneurs Docker (Kafka)..."
sudo docker compose up -d

# Étape 2: Attente active de la disponibilité de Kafka
echo "⏳ Attente de la disponibilité du broker Kafka..."
max_attempts=30
attempt=0
while [ $attempt -lt $max_attempts ]; do
  if sudo docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; then
    echo "✅ Kafka est prêt !"
    break
  fi
  attempt=$((attempt + 1))
  echo "Kafka n'est pas encore prêt, tentative $attempt/$max_attempts..."
  sleep 2
done

if [ $attempt -eq $max_attempts ]; then
  echo "❌ Erreur : Kafka n'a pas pu démarrer dans le délai imparti"
  exit 1
fi

# Étape 3: Création du topic Kafka 'orders'
# Cette commande est idempotente ; elle ne fera rien si le topic existe déjà.
echo "📝 Création du topic Kafka 'orders' (s'il n'existe pas)..."
sudo docker exec kafka kafka-topics \
  --bootstrap-server localhost:9092 \
  --create \
  --if-not-exists \
  --topic orders \
  --partitions 1 \
  --replication-factor 1

# Étape 4: Téléchargement des dépendances Go
echo "📦 Téléchargement des dépendances Go via 'go mod download'..."
go mod download

# Étape 5: Lancement du consommateur (tracker) en arrière-plan
# Le `&` à la fin de la commande le fait tourner en tâche de fond.
# Les logs du tracker seront visibles dans les fichiers tracker.log et tracker.events.
echo "🟢 Lancement du consommateur (tracker) en arrière-plan..."
go run -tags kafka,tracker cmd/tracker/main.go &
echo $! > "$script_dir/tracker.pid"

# Étape 6: Lancement du producteur (producer) au premier plan
# Le script attendra ici jusqu'à ce que le producteur soit manuellement arrêté.
echo "🟢 Lancement du producteur (producer) au premier plan..."
go run -tags kafka,producer cmd/producer/main.go &
producer_pid=$!
echo $producer_pid > "$script_dir/producer.pid"

# Attendre que le producteur se termine (par exemple, via Ctrl+C)
wait $producer_pid
