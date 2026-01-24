# ==============================================================================
# PubSubKafka - Dockerfile Multi-Stage
# ==============================================================================
# Build optimisé avec multi-stage pour une image finale légère.
#
# Usage:
#   docker build -t kafka-demo:latest -f deployments/docker/Dockerfile .
#   docker build --build-arg SERVICE=producer -t kafka-demo-producer:latest .
#   docker build --build-arg SERVICE=consumer -t kafka-demo-consumer:latest .

# ==============================================================================
# Stage 1: Builder
# ==============================================================================
FROM golang:1.24-alpine AS builder

# Arguments de build
ARG SERVICE=producer
ARG VERSION=dev
ARG COMMIT=unknown

# Installation des dépendances système
RUN apk add --no-cache \
    git \
    ca-certificates \
    tzdata \
    gcc \
    musl-dev \
    pkgconfig \
    librdkafka-dev

# Configuration du workspace
WORKDIR /app

# Copie des fichiers de dépendances
COPY go.mod go.sum ./

# Téléchargement des dépendances
RUN go mod download && go mod verify

# Copie du code source
COPY . .

# Build du binaire
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
    -o /app/bin/${SERVICE} \
    ./cmd/${SERVICE}/main.go

# ==============================================================================
# Stage 2: Runtime
# ==============================================================================
FROM alpine:3.19 AS runtime

# Arguments
ARG SERVICE=producer

# Labels OCI
LABEL org.opencontainers.image.title="PubSubKafka ${SERVICE}"
LABEL org.opencontainers.image.description="Kafka Demo - ${SERVICE} service"
LABEL org.opencontainers.image.vendor="PubSubKafka"
LABEL org.opencontainers.image.source="https://github.com/example/pubsubkafka"

# Installation des dépendances runtime
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    librdkafka \
    && rm -rf /var/cache/apk/*

# Création d'un utilisateur non-root
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Configuration du workspace
WORKDIR /app

# Copie du binaire depuis le builder
COPY --from=builder /app/bin/${SERVICE} /app/service

# Copie des fichiers de configuration
COPY --from=builder /app/config /app/config

# Configuration des permissions
RUN chown -R appuser:appgroup /app

# Changement d'utilisateur
USER appuser

# Variables d'environnement par défaut
ENV KAFKA_BROKER="localhost:9092"
ENV KAFKA_TOPIC="orders"
ENV LOG_LEVEL="info"
ENV CONFIG_FILE="/app/config/config.yaml"

# Exposition des ports
EXPOSE 8080 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Point d'entrée
ENTRYPOINT ["/app/service"]
