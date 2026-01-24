# ==============================================================================
# MAKEFILE - Kafka Demo Go Project
# ==============================================================================
#
# Ce Makefile fournit des commandes pratiques pour compiler, tester et gérer
# le projet Kafka Demo.
#
# Usage:
#   make help       - Afficher l'aide
#   make build      - Compiler tous les binaires
#   make test       - Exécuter les tests
#   make run        - Démarrer l'environnement complet
#   make stop       - Arrêter l'environnement
#   make clean      - Nettoyer les fichiers générés
#
# ==============================================================================

# Variables
BINARY_PRODUCER = producer
BINARY_CONSUMER = consumer
BINARY_TRACKER = tracker
BINARY_MONITOR = log_monitor
BINARY_CLI = kafka-cli
GO = go
DOCKER_COMPOSE = docker compose
DOCKER_COMPOSE_FILE = deployments/docker/docker-compose.yml

# Détection du système d'exploitation
ifeq ($(OS),Windows_NT)
	BINARY_EXT = .exe
	RM = del /Q
	RMDIR = rmdir /S /Q
else
	BINARY_EXT =
	RM = rm -f
	RMDIR = rm -rf
endif

# Cibles par défaut
.PHONY: all build test clean run stop help deps lint

all: build

# ==============================================================================
# BUILD
# ==============================================================================

## build: Compiler tous les binaires
build: build-producer build-consumer build-tracker build-monitor build-cli

## build-producer: Compiler le producteur
build-producer:
	@echo "🔨 Compilation du producteur..."
	$(GO) build -o $(BINARY_PRODUCER)$(BINARY_EXT) ./cmd/producer/main.go

## build-consumer: Compiler le consumer
build-consumer:
	@echo "🔨 Compilation du consumer..."
	$(GO) build -o $(BINARY_CONSUMER)$(BINARY_EXT) ./cmd/consumer/main.go

## build-tracker: Compiler le tracker (consommateur legacy)
build-tracker:
	@echo "🔨 Compilation du tracker..."
	$(GO) build -tags tracker -o $(BINARY_TRACKER)$(BINARY_EXT) ./cmd/tracker/main.go

## build-monitor: Compiler le moniteur de logs
build-monitor:
	@echo "🔨 Compilation du moniteur de logs..."
	$(GO) build -tags monitor -o $(BINARY_MONITOR)$(BINARY_EXT) ./cmd/monitor/main.go

## build-cli: Compiler l'outil CLI
build-cli:
	@echo "🔨 Compilation du CLI..."
	$(GO) build -o $(BINARY_CLI)$(BINARY_EXT) ./cmd/cli/main.go

# ==============================================================================
# TESTS
# ==============================================================================

## test: Exécuter tous les tests
test:
	@echo "🧪 Exécution des tests..."
	$(GO) test -tags kafka,producer,tracker,monitor -v ./...

## test-cover: Exécuter les tests avec couverture
test-cover:
	@echo "🧪 Exécution des tests avec couverture..."
	$(GO) test -tags kafka,producer,tracker,monitor -v -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "📊 Rapport de couverture généré: coverage.html"

## test-kafka: Exécuter les tests nécessitant Kafka
test-kafka:
	@echo "🧪 Exécution des tests Kafka..."
	$(GO) test -tags kafka -v ./...

# ==============================================================================
# DÉPENDANCES
# ==============================================================================

## deps: Télécharger les dépendances
deps:
	@echo "📦 Téléchargement des dépendances..."
	$(GO) mod download
	$(GO) mod tidy

## deps-upgrade: Mettre à jour les dépendances
deps-upgrade:
	@echo "⬆️  Mise à jour des dépendances..."
	$(GO) get -u ./...
	$(GO) mod tidy

# ==============================================================================
# QUALITÉ DE CODE
# ==============================================================================

## lint: Analyser le code avec golint et go vet
lint:
	@echo "🔍 Analyse du code..."
	$(GO) vet ./...
	@echo "✅ Analyse terminée"

## fmt: Formater le code
fmt:
	@echo "🎨 Formatage du code..."
	$(GO) fmt ./...

# ==============================================================================
# DOCKER & EXÉCUTION
# ==============================================================================

## docker-up: Démarrer les conteneurs Docker
docker-up:
	@echo "🐳 Démarrage des conteneurs Docker..."
	$(DOCKER_COMPOSE) up -d

## docker-down: Arrêter les conteneurs Docker
docker-down:
	@echo "🐳 Arrêt des conteneurs Docker..."
	$(DOCKER_COMPOSE) down

## docker-logs: Afficher les logs Kafka
docker-logs:
	$(DOCKER_COMPOSE) logs -f kafka

## run: Démarrer l'environnement complet (Linux/macOS)
run:
	@echo "🚀 Démarrage de l'environnement..."
	./start.sh

## stop: Arrêter l'environnement complet (Linux/macOS)
stop:
	@echo "🛑 Arrêt de l'environnement..."
	./stop.sh

## run-producer: Exécuter le producteur directement
run-producer: docker-up
	@echo "📤 Lancement du producteur..."
	$(GO) run -tags kafka cmd/producer/main.go

## run-tracker: Exécuter le tracker directement
run-tracker: docker-up
	@echo "📥 Lancement du tracker..."
	$(GO) run -tags kafka cmd/tracker/main.go

## run-monitor: Exécuter le moniteur de logs
run-monitor:
	@echo "📊 Lancement du moniteur de logs..."
	$(GO) run -tags monitor cmd/monitor/main.go

# ==============================================================================
# KAFKA
# ==============================================================================

## kafka-topics: Lister les topics Kafka
kafka-topics:
	docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list

## kafka-create-topic: Créer le topic 'orders'
kafka-create-topic:
	docker exec kafka kafka-topics \
		--bootstrap-server localhost:9092 \
		--create \
		--if-not-exists \
		--topic orders \
		--partitions 1 \
		--replication-factor 1

## kafka-consume: Consommer les messages du topic 'orders'
kafka-consume:
	docker exec kafka kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic orders \
		--from-beginning

# ==============================================================================
# SCRIPTS
# ==============================================================================

## create-topics: Créer tous les topics Kafka
create-topics:
	@echo "📝 Création des topics..."
	./scripts/create-topics.sh

## setup-acls: Configurer les ACLs Kafka
setup-acls:
	@echo "🔐 Configuration des ACLs..."
	./scripts/setup-acls.sh --apply

## benchmark: Exécuter les benchmarks de performance
benchmark:
	@echo "📊 Exécution des benchmarks..."
	./scripts/benchmark.sh all

## benchmark-producer: Benchmark du producer uniquement
benchmark-producer:
	./scripts/benchmark.sh producer

## benchmark-consumer: Benchmark du consumer uniquement
benchmark-consumer:
	./scripts/benchmark.sh consumer

## run-consumer: Exécuter le consumer directement
run-consumer: docker-up
	@echo "📥 Lancement du consumer..."
	$(GO) run ./cmd/consumer/main.go

# ==============================================================================
# NETTOYAGE
# ==============================================================================

## clean: Nettoyer tous les fichiers générés
clean:
	@echo "🧹 Nettoyage des fichiers générés..."
	$(RM) $(BINARY_PRODUCER)$(BINARY_EXT)
	$(RM) $(BINARY_CONSUMER)$(BINARY_EXT)
	$(RM) $(BINARY_TRACKER)$(BINARY_EXT)
	$(RM) $(BINARY_MONITOR)$(BINARY_EXT)
	$(RM) $(BINARY_CLI)$(BINARY_EXT)
	$(RM) tracker.log
	$(RM) tracker.events
	$(RM) producer.pid
	$(RM) tracker.pid
	$(RM) coverage.out
	$(RM) coverage.html
	@echo "✅ Nettoyage terminé"

## clean-logs: Nettoyer uniquement les fichiers de logs
clean-logs:
	@echo "🧹 Nettoyage des logs..."
	$(RM) tracker.log
	$(RM) tracker.events

# ==============================================================================
# AIDE
# ==============================================================================

## help: Afficher cette aide
help:
	@echo ""
	@echo "╔══════════════════════════════════════════════════════════════════════╗"
	@echo "║                    KAFKA DEMO - MAKEFILE HELP                        ║"
	@echo "╚══════════════════════════════════════════════════════════════════════╝"
	@echo ""
	@echo "Usage: make [cible]"
	@echo ""
	@echo "Cibles disponibles:"
	@echo ""
	@echo "  BUILD:"
	@echo "    build            Compiler tous les binaires"
	@echo "    build-producer   Compiler le producteur"
	@echo "    build-consumer   Compiler le consumer"
	@echo "    build-tracker    Compiler le tracker"
	@echo "    build-monitor    Compiler le moniteur de logs"
	@echo "    build-cli        Compiler l'outil CLI"
	@echo ""
	@echo "  TESTS:"
	@echo "    test             Exécuter tous les tests"
	@echo "    test-cover       Tests avec rapport de couverture"
	@echo "    test-kafka       Tests nécessitant Kafka"
	@echo ""
	@echo "  DÉPENDANCES:"
	@echo "    deps             Télécharger les dépendances"
	@echo "    deps-upgrade     Mettre à jour les dépendances"
	@echo ""
	@echo "  QUALITÉ:"
	@echo "    lint             Analyser le code"
	@echo "    fmt              Formater le code"
	@echo ""
	@echo "  EXÉCUTION:"
	@echo "    run              Démarrer l'environnement complet"
	@echo "    stop             Arrêter l'environnement complet"
	@echo "    run-producer     Exécuter le producteur"
	@echo "    run-consumer     Exécuter le consumer"
	@echo "    run-tracker      Exécuter le tracker"
	@echo "    run-monitor      Exécuter le moniteur"
	@echo ""
	@echo "  DOCKER:"
	@echo "    docker-up        Démarrer Kafka"
	@echo "    docker-down      Arrêter Kafka"
	@echo "    docker-logs      Afficher les logs Kafka"
	@echo ""
	@echo "  KAFKA:"
	@echo "    kafka-topics       Lister les topics"
	@echo "    kafka-create-topic Créer le topic 'orders'"
	@echo "    kafka-consume      Consommer les messages"
	@echo ""
	@echo "  SCRIPTS:"
	@echo "    create-topics      Créer tous les topics"
	@echo "    setup-acls         Configurer les ACLs"
	@echo "    benchmark          Exécuter les benchmarks"
	@echo ""
	@echo "  NETTOYAGE:"
	@echo "    clean            Nettoyer tous les fichiers"
	@echo "    clean-logs       Nettoyer les logs uniquement"
	@echo ""
