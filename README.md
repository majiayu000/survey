# Survey Service

A standalone survey/polling service with ATProto integration.

## Features

- **Multi-question surveys**: Single choice, multiple choice, and free text questions
- **YAML/JSON definitions**: Define surveys in YAML or JSON
- **Web UI**: Clean, responsive HTML interface with HTMX
- **JSON API**: RESTful API for programmatic access
- **Live results**: Real-time result aggregation with polling
- **Privacy-preserving**: Per-survey salted guest identity (can't track across surveys)
- **ATProto ready**: Lexicons defined for future Bluesky integration

## Architecture

- **survey-api**: Web server with HTML (Templ) and JSON API endpoints
- **survey-consumer**: Jetstream consumer for ATProto votes (future)

## Tech Stack

- **Language**: Go 1.24+
- **HTTP Framework**: Echo v4
- **Templates**: Templ + HTMX
- **Database**: PostgreSQL (via pgx/v5)
- **Observability**: OpenTelemetry (otelsql)
- **Metrics**: Prometheus

## Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL 14+

### Database Setup

```bash
# Create database
createdb survey

# Run migrations
psql survey < internal/db/migrations/001_initial.up.sql
```

### Configuration

```bash
export DATABASE_HOST=localhost
export DATABASE_PORT=5432
export DATABASE_USER=postgres
export DATABASE_PASSWORD=yourpassword
export DATABASE_NAME=survey
export PORT=8080
```

### Running

```bash
# Run the API server
go run ./cmd/api

# Server starts on http://localhost:8080
```

### Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | Survey list (HTML) |
| `GET /surveys/new` | Create survey form (HTML) |
| `GET /surveys/:slug` | Survey form (HTML) |
| `GET /surveys/:slug/results` | Results page (HTML) |
| `GET /health` | Liveness probe |
| `GET /health/ready` | Readiness probe (checks DB) |
| `GET /metrics` | Prometheus metrics |

#### JSON API

| Endpoint | Description |
|----------|-------------|
| `POST /api/v1/surveys` | Create survey |
| `GET /api/v1/surveys` | List surveys |
| `GET /api/v1/surveys/:slug` | Get survey |
| `POST /api/v1/surveys/:slug/responses` | Submit response |
| `GET /api/v1/surveys/:slug/results` | Get results |

## Survey Definition Format

```yaml
name: "Weekly Sync Preference"
description: "Help us pick a meeting time"
anonymous: false
startsAt: "2025-12-11T00:00:00Z"
endsAt: "2025-12-31T23:59:00Z"

questions:
  - id: q1
    text: "Preferred day?"
    type: single
    required: true
    options:
      - id: mon
        text: "Monday"
      - id: tue
        text: "Tuesday"

  - id: q2
    text: "What topics should we cover?"
    type: multi
    required: false
    options:
      - id: planning
        text: "Sprint planning"
      - id: demos
        text: "Demos"

  - id: q3
    text: "Any other feedback?"
    type: text
    required: false
```

## Project Structure

```
survey/
├── cmd/
│   ├── api/              # survey-api entrypoint
│   └── consumer/         # survey-consumer entrypoint (stub)
├── internal/
│   ├── api/              # HTTP handlers, router, middleware
│   ├── db/               # Database access and migrations
│   ├── telemetry/        # Metrics setup
│   ├── models/           # Domain models
│   └── templates/        # Templ templates
├── lexicon/              # ATProto lexicon schemas
├── k8s/                  # Kubernetes manifests
└── Dockerfile
```

## Deployment

### Docker

```bash
docker build -t survey .
docker run -p 8080:8080 -e DATABASE_PASSWORD=secret survey
```

### Kubernetes

```bash
kubectl apply -k k8s/base/
```

The deployment includes:
- **survey-api**: 2 replicas (stateless, scalable)
- **survey-consumer**: 1 replica (single Jetstream cursor)

## ATProto Lexicons

- `net.openmeet.survey`: Survey/poll definition record
- `net.openmeet.survey.response`: User response record

See `lexicon/` directory for full schemas.

## License

Apache License 2.0 - See LICENSE file.
