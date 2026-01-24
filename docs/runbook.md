# Runbook Opérationnel - PubSubKafka

## Table des Matières

1. [Démarrage et Arrêt](#démarrage-et-arrêt)
2. [Vérifications de Santé](#vérifications-de-santé)
3. [Incidents Courants](#incidents-courants)
4. [Maintenance](#maintenance)
5. [Procédures d'Urgence](#procédures-durgence)

---

## Démarrage et Arrêt

### Démarrage de l'environnement

#### Développement Local

```bash
# 1. Démarrer Kafka et les dépendances
make docker-up

# 2. Créer les topics
./scripts/create-topics.sh

# 3. Lancer le producer
make run-producer

# 4. Lancer le consumer (autre terminal)
make run-tracker
```

#### Production (Kubernetes)

```bash
# 1. Appliquer les configurations
kubectl apply -f deployments/kubernetes/configmap.yaml
kubectl apply -f deployments/kubernetes/deployment.yaml
kubectl apply -f deployments/kubernetes/service.yaml

# 2. Vérifier le déploiement
kubectl get pods -l app=kafka-demo
kubectl rollout status deployment/kafka-consumer
```

### Arrêt propre

#### Développement

```bash
# Arrêt gracieux
make stop

# Ou manuellement
docker compose down
```

#### Production

```bash
# Scale down progressif pour éviter le rebalance brutal
kubectl scale deployment kafka-consumer --replicas=1
sleep 30
kubectl scale deployment kafka-consumer --replicas=0

# Puis arrêter le producer
kubectl scale deployment kafka-producer --replicas=0
```

---

## Vérifications de Santé

### Checklist Quotidienne

| Vérification | Commande | Attendu |
|--------------|----------|---------|
| Pods running | `kubectl get pods -l app=kafka-demo` | STATUS: Running |
| Consumer lag | `kafka-consumer-groups --describe` | LAG < 100 |
| DLQ empty | `kafka-console-consumer --topic orders-dlq` | Pas de nouveaux messages |
| Métriques OK | `curl :9090/metrics` | HTTP 200 |

### Vérifier l'état du cluster Kafka

```bash
# Lister les topics
docker exec kafka kafka-topics --bootstrap-server localhost:9092 --list

# Décrire un topic
docker exec kafka kafka-topics --bootstrap-server localhost:9092 \
  --describe --topic orders

# Vérifier les consumer groups
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group order-tracker-group
```

### Vérifier les métriques

```bash
# Métriques du producer
curl -s http://localhost:9090/metrics | grep kafka_producer

# Métriques du consumer
curl -s http://localhost:9090/metrics | grep kafka_consumer

# Consumer lag
curl -s http://localhost:9090/metrics | grep kafka_consumer_lag
```

---

## Incidents Courants

### INC-001: Consumer Lag Élevé

**Symptômes:**
- Métrique `kafka_consumer_lag` > 1000
- Messages traités avec retard

**Diagnostic:**

```bash
# Vérifier le lag actuel
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group order-tracker-group

# Vérifier les logs du consumer
kubectl logs -l component=consumer --tail=100
```

**Résolution:**

1. **Scaling horizontal:**
```bash
kubectl scale deployment kafka-consumer --replicas=6
```

2. **Vérifier les performances:**
```bash
# Temps de traitement moyen
curl -s http://localhost:9090/metrics | grep processing_duration
```

3. **Identifier les messages lents:**
   - Activer le debug logging temporairement
   - Analyser les traces Jaeger

**Prévention:**
- Alerter si lag > 500 pendant 5 min
- Auto-scaling basé sur le lag

---

### INC-002: Messages dans la DLQ

**Symptômes:**
- Messages apparaissent dans `orders-dlq`
- Métrique `kafka_dlq_messages_total` augmente

**Diagnostic:**

```bash
# Lire les messages DLQ
docker exec kafka kafka-console-consumer \
  --bootstrap-server localhost:9092 \
  --topic orders-dlq \
  --from-beginning \
  --max-messages 10

# Analyser un message DLQ
# Chaque message contient:
# - original_payload: le message original
# - failure_reason: raison de l'échec
# - error_message: détail de l'erreur
# - retry_count: nombre de tentatives
```

**Résolution:**

1. **Erreurs de validation:**
   - Identifier le producteur fautif
   - Corriger les données source

2. **Erreurs de désérialisation:**
   - Vérifier la compatibilité des schémas
   - Mettre à jour les consumers

3. **Erreurs transitoires:**
   - Rejouer les messages après correction:
```bash
# Via le CLI
go run cmd/cli/main.go dlq replay --id <message-id>
```

---

### INC-003: Broker Kafka Inaccessible

**Symptômes:**
- Erreur `ErrBrokerNotAvailable`
- Producer/Consumer ne peuvent pas se connecter

**Diagnostic:**

```bash
# Vérifier l'état du broker
docker exec kafka kafka-broker-api-versions \
  --bootstrap-server localhost:9092

# Vérifier les logs Kafka
docker logs kafka --tail=100
```

**Résolution:**

1. **Redémarrer le broker:**
```bash
docker restart kafka
```

2. **Vérifier Zookeeper:**
```bash
docker exec zookeeper zkServer.sh status
```

3. **Vérifier les ressources:**
```bash
# Mémoire/CPU du conteneur
docker stats kafka
```

**Escalade:** Si le cluster ne récupère pas en 10 minutes, escalader.

---

### INC-004: Rebalance Continu du Consumer Group

**Symptômes:**
- Logs répétés de rebalance
- Traitement des messages interrompu

**Diagnostic:**

```bash
# Vérifier les membres du groupe
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --describe --group order-tracker-group --members

# Chercher les déconnexions dans les logs
kubectl logs -l component=consumer | grep -i rebalance
```

**Causes possibles:**
- Session timeout trop court
- Pods qui crashent
- Réseau instable

**Résolution:**

1. **Ajuster les timeouts:**
```yaml
# Dans la config consumer
session.timeout.ms: 45000
heartbeat.interval.ms: 10000
```

2. **Stabiliser les pods:**
   - Vérifier les liveness probes
   - Augmenter les ressources si OOMKilled

---

### INC-005: Producer ne peut pas envoyer

**Symptômes:**
- Erreur `ErrDeliveryFailed`
- Messages en attente non délivrés

**Diagnostic:**

```bash
# Vérifier les logs producer
kubectl logs -l component=producer --tail=50

# Tester la connexion
docker exec kafka kafka-broker-api-versions \
  --bootstrap-server localhost:9092
```

**Résolution:**

1. **Vérifier la configuration acks:**
   - `acks=all` requiert tous les ISR disponibles

2. **Augmenter le timeout de flush:**
```go
FlushTimeout: 30 * time.Second
```

3. **Vérifier l'espace disque:**
```bash
docker exec kafka df -h /var/lib/kafka
```

---

## Maintenance

### Mise à jour des Applications

#### Rolling Update (Zero Downtime)

```bash
# 1. Mettre à jour l'image
kubectl set image deployment/kafka-consumer \
  consumer=kafka-demo-consumer:v2.0.0

# 2. Suivre le rollout
kubectl rollout status deployment/kafka-consumer

# 3. Vérifier la santé
kubectl get pods -l component=consumer
```

#### Rollback si nécessaire

```bash
kubectl rollout undo deployment/kafka-consumer
```

### Ajout de Partitions

```bash
# Augmenter le nombre de partitions
docker exec kafka kafka-topics --bootstrap-server localhost:9092 \
  --alter --topic orders --partitions 12

# Vérifier
docker exec kafka kafka-topics --bootstrap-server localhost:9092 \
  --describe --topic orders
```

⚠️ **Attention:** Ne JAMAIS réduire le nombre de partitions.

### Purge d'un Topic

```bash
# Option 1: Supprimer et recréer
docker exec kafka kafka-topics --bootstrap-server localhost:9092 \
  --delete --topic orders

# Option 2: Modifier la retention
docker exec kafka kafka-configs --bootstrap-server localhost:9092 \
  --alter --entity-type topics --entity-name orders \
  --add-config retention.ms=1000

# Attendre puis restaurer
docker exec kafka kafka-configs --bootstrap-server localhost:9092 \
  --alter --entity-type topics --entity-name orders \
  --add-config retention.ms=604800000
```

### Backup des Offsets

```bash
# Exporter les offsets actuels
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group order-tracker-group --describe > offsets_backup.txt

# Réinitialiser si nécessaire
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group order-tracker-group --reset-offsets \
  --to-earliest --topic orders --execute
```

---

## Procédures d'Urgence

### Arrêt d'Urgence

```bash
# Arrêter tous les consumers immédiatement
kubectl scale deployment kafka-consumer --replicas=0

# Arrêter les producers
kubectl scale deployment kafka-producer --replicas=0
```

### Disaster Recovery

1. **Évaluer l'impact:**
   - Nombre de messages perdus
   - Dernière position connue

2. **Récupérer depuis les backups:**
```bash
# Restaurer les offsets
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group order-tracker-group --reset-offsets \
  --to-offset <offset> --topic orders --execute
```

3. **Rejouer les messages:**
   - Utiliser un consumer temporaire depuis l'offset désiré

### Contact Escalade

| Niveau | Contact | Délai |
|--------|---------|-------|
| L1 | On-call SRE | Immédiat |
| L2 | Team Lead | 15 min |
| L3 | Architecture | 30 min |
| Critique | Management | 1h |

---

## Commandes Utiles

```bash
# État général
make status

# Logs en temps réel
kubectl logs -f -l app=kafka-demo

# Métriques résumées
curl -s localhost:9090/metrics | grep -E '^kafka_'

# Test de connectivité
docker exec kafka kafka-broker-api-versions \
  --bootstrap-server localhost:9092

# Consumer lag toutes les 5s
watch -n5 'docker exec kafka kafka-consumer-groups \
  --bootstrap-server localhost:9092 \
  --describe --group order-tracker-group'
```
