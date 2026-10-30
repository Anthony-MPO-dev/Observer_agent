# LogStream: real-time log observability (POC)

I built LogStream because debugging my Python/Celery workers in production meant opening container after container and reading logs line by line. This repo is a proof of concept you can run locally. A mock FastAPI service plays the Celery workers and writes real log files, and the pipeline takes it from there:

```
Mock API (Python) ──writes──> .log files ──agent reads──> gRPC ──> Server ──> Dashboard (WebSocket)
```

## What's inside

- **Log agent (Go):** a sidecar that watches `.log` files with `fsnotify`, tails them, parses each line and ships batches over gRPC. It buffers locally, so nothing is lost if the server goes down.
- **healthmon (part of the agent):** watches the error rate of the external APIs the workers depend on. Past a configurable threshold, it blocks new requests to that service, and it reopens on its own once the service recovers. Retries with backoff weren't enough, because new requests kept hitting a service that was already down.
- **Log server (Go):** receives the gRPC streams, stores logs as `.jsonl` per service per day, and fans them out through an in-memory pub/sub hub. It also exposes REST and WebSocket.
- **Dashboard (React 18 + TypeScript):** live logs over WebSocket, no polling, with filters by service, level, document and task ID.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Docker Network (poc-logstream)                   │
│                                                                         │
│  ┌──────────────┐     volume       ┌──────────────┐    gRPC           │
│  │  Mock API    │  ──(poc_logs)──> │  Log Agent   │ ──────────────>   │
│  │  (FastAPI)   │    .log files    │  (Go sidecar)│                   │
│  │  :8000       │                  │              │                   │
│  └──────────────┘                  └──────────────┘                   │
│        │                                    │                          │
│        │ Simulates:                          │                          │
│        │ - Celery workers                    ▼                          │
│        │ - CNPJ/CPF lookups        ┌──────────────┐     WebSocket     │
│        │ - Errors and retries      │  Log Server  │ ──────────────>   │
│        │ - End-to-end flows        │  (Go)        │                   │
│        │                           │  gRPC :50051 │    ┌────────────┐ │
│        │                           │  HTTP  :8080 │    │ Dashboard  │ │
│        │                           └──────────────┘    │ (React)    │ │
│        │                                  │            │ :3000      │ │
│        │                                  ▼            └────────────┘ │
│        │                           ┌──────────────┐                   │
│        │                           │    Redis     │                   │
│        │                           │  (offsets +  │                   │
│        │                           │   dedup)     │                   │
│        │                           └──────────────┘                   │
└─────────────────────────────────────────────────────────────────────────┘
```

### Log flow

1. The **mock API** writes logs to `/app/logs/` with `LogService`, in the same format as the production API.
2. The **log agent** picks up new `.log` files via `fsnotify` and tails them in real time.
3. The agent parses each line into timestamp, level, uuid, document, task_id and module.
4. The agent sends batches over **gRPC** to the **log server**.
5. The server writes them to `.jsonl` files and publishes them through the **hub** (in-memory pub/sub).
6. The **dashboard** receives them over **WebSocket** and renders them live.

## Quick start

You need Docker and Docker Compose.

```bash
git clone https://github.com/Anthony-MPO-dev/Observer_agent.git
cd Observer_agent

# Copy the env files
cp .env.example .env
cp .env.agent.example .env.agent
cp .env.server.example .env.server

# Start everything
docker compose up --build -d
```

| Service       | URL                        | What it is                  |
|---------------|----------------------------|-----------------------------|
| Mock API      | http://localhost:8000      | Swagger UI (FastAPI docs)   |
| Mock API docs | http://localhost:8000/docs | Interactive API docs        |
| Dashboard     | http://localhost:3000      | LogStream dashboard         |
| Server REST   | http://localhost:8080      | log-server REST API         |

Dashboard login for the local demo: `admin` / `admin123`. Change it before you run this anywhere other than your own machine.

## Mock API endpoints

| Method | Endpoint                     | What it does                                  |
|--------|------------------------------|-----------------------------------------------|
| POST   | `/simulate/consulta-cnpj`    | Simulates a CNPJ lookup (worker_core)         |
| POST   | `/simulate/consulta-cpf`     | Simulates a CPF lookup (worker_quick)         |
| POST   | `/simulate/batch`            | Generates a batch of mixed logs               |
| POST   | `/simulate/error`            | Generates an error scenario with fallback     |
| POST   | `/simulate/worker-flow`      | Full flow: API -> dispatch -> worker          |
| POST   | `/simulate/start-continuous` | Starts continuous log generation              |
| POST   | `/simulate/stop-continuous`  | Stops continuous log generation               |
| GET    | `/health`                    | Health check                                  |

CNPJ and CPF are Brazilian company and individual tax IDs. Every lookup here is simulated.

### curl examples

```bash
# Simulate 5 CNPJ lookups
curl -X POST http://localhost:8000/simulate/consulta-cnpj \
  -H "Content-Type: application/json" \
  -d '{"count": 5}'

# Simulate a full worker flow
curl -X POST http://localhost:8000/simulate/worker-flow

# Generate an error scenario
curl -X POST http://localhost:8000/simulate/error

# Generate a batch of 20 logs, one per second
curl -X POST http://localhost:8000/simulate/batch \
  -H "Content-Type: application/json" \
  -d '{"count": 20, "interval": 1.0}'

# Set continuous generation to 1 log per second
curl -X POST http://localhost:8000/simulate/start-continuous \
  -H "Content-Type: application/json" \
  -d '{"interval": 1.0}'
```

## Log format

`LogService` writes lines like this:

```
2024-01-15 14:32:01 [INFO] [uuid=abc-def-123] [main.py:93] simulate_single_consulta_cnpj() - [DOC:12345678000195] Consultando fonte: receita_federal
```

Fields the agent extracts:

- **timestamp**: `2024-01-15 14:32:01`
- **level**: `INFO`
- **uuid**: `abc-def-123` (request correlation)
- **module**: `main.py:93`
- **function**: `simulate_single_consulta_cnpj`
- **document**: `12345678000195` (CNPJ/CPF)
- **message**: the full text

## Simulated workers

| Logger              | File it writes                              | Simulates                 |
|---------------------|---------------------------------------------|---------------------------|
| `worker_core`       | `dados_basicos_worker_consulta_core_*.log`  | Main Celery worker        |
| `worker_quick`      | `dados_basicos_worker_consulta_quick_*.log` | Fast Celery worker        |
| `worker_background` | `dados_basicos_worker_background_*.log`     | Background Celery worker  |
| `api`               | `dados_basicos_api_*.log`                   | FastAPI/Gunicorn          |

## Project structure

```
Observer_agent/
├── app/
│   ├── __init__.py
│   ├── main.py                  # FastAPI: simulation endpoints
│   └── services/
│       ├── __init__.py
│       └── log_service.py       # LogService (same format as the production API)
├── logstream/                   # LogStream: the observability system
│   ├── agent/                   # Go sidecar: watches .log files via fsnotify
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── ...                  # watcher, parser, sender, buffer, healthmon
│   ├── server/                  # Go server: gRPC + REST + WebSocket
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── ...                  # grpc, hub, store, api, auth, cleaner
│   ├── dashboard/               # React frontend: real-time view
│   │   ├── Dockerfile
│   │   ├── nginx.conf
│   │   └── src/                 # React 18 + TypeScript + TailwindCSS + ApexCharts
│   └── proto/
│       └── logs.proto           # gRPC schema (agent <-> server contract)
├── docker-compose.yml           # Full stack (5 services)
├── Dockerfile                   # Mock API build
├── requirements.txt             # Python dependencies
├── .env.example                 # Mock API config template
├── .env.agent.example           # Log agent config template
├── .env.server.example          # Log server config template
├── .gitignore
└── README.md
```

## Default behavior

On startup the mock API writes a log every 3 seconds on its own:

- **40%** CNPJ lookups
- **25%** CPF lookups
- **25%** full flows (API -> worker -> background)
- **10%** errors (timeout, connection error, service unavailable)

So the dashboard already has live logs the moment you open it.

## Operations

```bash
# Follow logs from every container
docker compose logs -f

# Only the mock API
docker compose logs -f mock-api

# Only the agent
docker compose logs -f log-agent

# Stop everything
docker compose down

# Stop and wipe volumes
docker compose down -v

# Rebuild after changes
docker compose up --build -d
```

## Where it came from

I built LogStream for a set of Python/Celery microservices that look up company and individual records in external sources through RabbitMQ. In production:

- the Celery workers (core, quick, background) process real CNPJ/CPF lookups through RabbitMQ;
- the log agent runs as a sidecar, watching the shared log volume;
- the log server runs on a central VPS and receives logs from several APIs;
- the dashboard gives real-time monitoring with filters by service, level, document and task_id.

The architecture is mine. I used Claude Code for parts of the implementation, the same way I use any other tool.
