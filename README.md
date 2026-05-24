# Insider One Notification System

A scalable, event-driven notification delivery system built with Go, designed to process and deliver messages across SMS, Email, and Push channels with high throughput, reliable delivery, and real-time status tracking.

## Architecture

```
                    +------------+
    Client -------> | KrakenD    |  (Ingress Rate Limiting + Tracing)
                    +-----+------+
                          |
                    +-----v------+
                    |  API       |  (REST Handlers)
                    |  Server    |
                    +-----+------+
                          |
              +-----------+-----------+
              v           v           v
         +--------+  +--------+  +--------+
         | Kafka  |  | Kafka  |  | Kafka  |  (Per-Channel Topics)
         |  SMS   |  | Email  |  |  Push  |
         +---+----+  +---+----+  +---+----+
             |            |           |
         +---v------------v-----------v---+
         |        Worker Pool             |  (Token Bucket Rate Limiter)
         |   100 msg/s per channel        |
         +---+-------------------+--------+
             |                   |
        +----v----+       +-----v------+
        | Webhook |       | Retry/DLQ  |
        | (Send)  |       | Handler    |
        +----+----+       +------------+
             |
        +----v-----+    +----------+    +------------+
        | Postgres |    |  Jaeger  |    | Prometheus |
        |  (JSONB) |    | (Traces) |    | (Metrics)  |
        +----------+    +----------+    +------------+
```

**Design Principles:**
- **Hexagonal Architecture** -- Domain logic isolated from infrastructure
- **100% Stateless** -- No in-memory buffering; state lives in Kafka + PostgreSQL
- **Per-Channel Isolation** -- Independent Kafka topics, rate limiters, and scaling

## Quick Start

```bash
# Clone and start everything
git clone https://github.com/kubilayciftci/insider-one-notification.git
cd insider-one-notification

# Optional: Set your webhook.site URL
export WEBHOOK_URL=https://webhook.site/your-uuid

# Boot the entire ecosystem
docker-compose up --build
```

**Services available after startup:**

| Service      | URL                          |
|-------------|------------------------------|
| API Gateway | http://localhost:8000         |
| API Direct  | http://localhost:8080         |
| Grafana     | http://localhost:3000         |
| Jaeger UI   | http://localhost:16686        |
| Prometheus  | http://localhost:9090         |

## API Examples

### Create a Notification

```bash
curl -X POST http://localhost:8000/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Your verification code is 123456",
    "priority": "high",
    "idempotency_key": "verify-user-123"
  }'
```

### Create a Batch

```bash
curl -X POST http://localhost:8000/api/v1/notifications/batch \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {"recipient": "+905551111111", "channel": "sms", "content": "Flash sale!", "priority": "high"},
      {"recipient": "user@example.com", "channel": "email", "content": "Check this out", "priority": "normal"}
    ]
  }'
```

### Query Status

```bash
# By ID
curl http://localhost:8000/api/v1/notifications/{id}

# By Batch ID
curl http://localhost:8000/api/v1/notifications/batch/{batchId}

# List with filters
curl "http://localhost:8000/api/v1/notifications?status=delivered&channel=sms&page=1&page_size=20"
```

### Cancel a Notification

```bash
curl -X DELETE http://localhost:8000/api/v1/notifications/{id}
```

## Running Tests

```bash
# Unit tests
make test-unit

# Integration tests (requires Docker)
make test-integration

# All tests
make test

# Linting
make lint
```

## Project Structure

```
├── cmd/api/          # HTTP API server
├── cmd/worker/       # Kafka consumer workers
├── internal/
│   ├── core/
│   │   ├── domain/   # Entities, value objects, validation
│   │   ├── ports/    # Interface definitions (hexagonal ports)
│   │   └── service/  # Business logic
│   ├── adapters/
│   │   ├── postgres/ # Database adapter (pgx)
│   │   ├── kafka/    # Message queue adapter
│   │   ├── webhook/  # External provider adapter
│   │   └── rest/     # HTTP adapter (chi)
│   ├── config/       # Environment configuration
│   ├── telemetry/    # OTel, Prometheus, slog
│   └── worker/       # Worker orchestration
├── migrations/       # Versioned SQL migrations
├── api/              # OpenAPI 3.0 specification
├── deploy/k8s/       # Kubernetes + ArgoCD manifests
└── docker-compose.yml
```

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **pgx over GORM** | Direct SQL control, connection pooling, no magic |
| **JSONB payload** | Zero schema migrations for new channels |
| **Per-channel Kafka topics** | Independent scaling and failure isolation |
| **Token Bucket rate limiting** | Configurable per channel via `RATE_LIMIT_PER_SEC` env var (default 100 msg/s) |
| **Exponential backoff + jitter** | Prevents thundering herd on retry storms |
| **KrakenD gateway** | Ingress rate limiting, trace injection, decoupled from app |
| **Two binaries** | Independent scaling of API and workers |
| **Manual Kafka offset commits** | At-least-once delivery guarantee |

## Scaling Strategy

- **API**: HPA scales 2-10 replicas based on CPU utilization (70% threshold)
- **Worker**: HPA scales 2-20 replicas based on CPU utilization (60% threshold)
- **Production upgrade**: Use [KEDA](https://keda.sh) with `kafka_consumergroup_lag` metric for workload-driven autoscaling instead of CPU-based

## Technology Stack

- **Language:** Go 1.24
- **Database:** PostgreSQL 16 with JSONB
- **Queue:** Apache Kafka (KRaft mode)
- **Gateway:** KrakenD 2.6
- **Tracing:** OpenTelemetry -> Jaeger
- **Metrics:** Prometheus
- **Logging:** log/slog (structured JSON)
- **Migrations:** golang-migrate
- **Testing:** testcontainers-go, go.uber.org/mock
