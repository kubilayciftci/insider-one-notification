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

## Extensibility — Adding New Channels

The hexagonal architecture makes adding new channels (e.g., WhatsApp, Telegram, Slack) trivial:

1. Add the channel constant to `internal/core/domain/notification.go`:
   ```go
   ChannelWhatsApp Channel = "whatsapp"
   ```
2. Update `ParseChannel()` to accept the new value
3. Create Kafka topics: `notifications-whatsapp-{high,normal,low}`
4. Restart workers — they automatically consume from new topics

**Zero database migrations required** — the JSONB `payload` column handles any channel-specific data.

## Configuration

All settings are configurable via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `:8080` | HTTP server port |
| `DATABASE_URL` | `postgres://...` | PostgreSQL connection string |
| `KAFKA_BROKERS` | `localhost:9092` | Comma-separated Kafka brokers |
| `WEBHOOK_URL` | — | webhook.site URL for delivery simulation |
| `RATE_LIMIT_PER_SEC` | `100` | Max messages per second per channel (configurable) |
| `MAX_RETRIES` | `3` | Max delivery retry attempts |
| `BASE_RETRY_DELAY` | `5s` | Base delay for exponential backoff |
| `JAEGER_ENDPOINT` | `http://localhost:4318/v1/traces` | OTel trace exporter |
| `SERVICE_NAME` | `insider-one-notification` | Service name for traces/logs |

## Bonus Features

| Feature | Implementation |
|---------|---------------|
| **Failure Handling** | Exponential backoff with jitter + Dead Letter Queue |
| **Scheduled Notifications** | `scheduled_at` field, scheduler worker polls DB |
| **Template System** | `{{variable}}` substitution from `payload.template_vars` |
| **WebSocket Updates** | Real-time status push via PostgreSQL LISTEN/NOTIFY |
| **Distributed Tracing** | End-to-end OTel: KrakenD → API → Kafka → Worker → Webhook |
| **GitHub Actions CI/CD** | Lint + unit tests + integration tests + Docker build |

### Template Example

```bash
curl -X POST http://localhost:8000/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Hi {{name}}, your order {{order_id}} is confirmed!",
    "priority": "high",
    "payload": {
      "template_vars": {
        "name": "Kubilay",
        "order_id": "ORD-98765"
      }
    }
  }'
```

### Scheduled Notification Example

```bash
curl -X POST http://localhost:8000/api/v1/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "recipient": "+905551234567",
    "channel": "sms",
    "content": "Flash sale starts NOW!",
    "priority": "high",
    "scheduled_at": "2026-05-25T09:00:00Z"
  }'
```

### WebSocket Example

```bash
wscat -c ws://localhost:8080/api/v1/ws/notifications/{id}
```

You'll receive messages like:

```json
{"id":"uuid","status":"queued","channel":"sms","updated_at":"..."}
{"id":"uuid","status":"delivered","channel":"sms","updated_at":"..."}
```
