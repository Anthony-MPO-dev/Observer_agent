# POC LogStream - Mock API

Proof of Concept que demonstra o sistema **LogStream** em funcionamento completo.  
Uma FastAPI mock simula workers Celery gerando logs reais, que fluem pela pipeline:

```
Mock API (Python) ──writes──> .log files ──agent reads──> gRPC ──> Server ──> Dashboard (WebSocket)
```

---

## Arquitetura

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
│        │ Simula:                             │                          │
│        │ - Workers Celery                    ▼                          │
│        │ - Consultas CNPJ/CPF      ┌──────────────┐     WebSocket     │
│        │ - Erros e retries         │  Log Server  │ ──────────────>   │
│        │ - Fluxos completos        │  (Go)        │                   │
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

### Fluxo dos Logs

1. **Mock API** escreve logs em `/app/logs/` usando o `LogService` (mesmo formato da API real)
2. **Log Agent** (Go) detecta novos arquivos `.log` via `fsnotify`, faz tail em tempo real
3. Agent parseia cada linha extraindo: timestamp, level, uuid, documento, task_id, module
4. Agent envia batches via **gRPC** para o **Log Server**
5. Server persiste em arquivos `.jsonl` e publica via **Hub** (pub/sub in-memory)
6. **Dashboard** recebe logs via **WebSocket** e renderiza em tempo real

---

## Quick Start

### Pre-requisitos

- Docker e Docker Compose

### Subir a stack

```bash
# Clonar e entrar no diretorio
cd POC_Agente

# Copiar arquivos de ambiente
cp .env.example .env
cp .env.agent.example .env.agent
cp .env.server.example .env.server

# Subir tudo
docker compose up --build -d
```

### Acessar

| Servico      | URL                          | Descricao                       |
|--------------|------------------------------|---------------------------------|
| Mock API     | http://localhost:8000        | Swagger UI (FastAPI docs)       |
| Mock API Docs| http://localhost:8000/docs   | Documentacao interativa         |
| Dashboard    | http://localhost:3000        | LogStream Dashboard             |
| Server REST  | http://localhost:8080        | API REST do log-server          |

### Login no Dashboard

- **Usuario:** `admin`
- **Senha:** `admin123`

---

## Endpoints da Mock API

### Simulacoes

| Metodo | Endpoint                      | Descricao                                    |
|--------|-------------------------------|----------------------------------------------|
| POST   | `/simulate/consulta-cnpj`     | Simula consulta CNPJ (worker_core)           |
| POST   | `/simulate/consulta-cpf`      | Simula consulta CPF (worker_quick)           |
| POST   | `/simulate/batch`             | Gera lote de logs variados                   |
| POST   | `/simulate/error`             | Gera cenario de erro com fallback            |
| POST   | `/simulate/worker-flow`       | Fluxo completo: API -> dispatch -> worker    |
| POST   | `/simulate/start-continuous`  | Inicia geracao continua de logs              |
| POST   | `/simulate/stop-continuous`   | Para geracao continua                        |
| GET    | `/health`                     | Health check                                 |

### Exemplos com curl

```bash
# Simular 5 consultas CNPJ
curl -X POST http://localhost:8000/simulate/consulta-cnpj \
  -H "Content-Type: application/json" \
  -d '{"count": 5}'

# Simular fluxo completo de worker
curl -X POST http://localhost:8000/simulate/worker-flow

# Gerar cenario de erro
curl -X POST http://localhost:8000/simulate/error

# Gerar lote de 20 logs com intervalo de 1s
curl -X POST http://localhost:8000/simulate/batch \
  -H "Content-Type: application/json" \
  -d '{"count": 20, "interval": 1.0}'

# Ajustar geracao continua para 1 log/segundo
curl -X POST http://localhost:8000/simulate/start-continuous \
  -H "Content-Type: application/json" \
  -d '{"interval": 1.0}'
```

---

## Formato dos Logs

O `LogService` gera logs no formato padrao da API_DadosBasicos:

```
2024-01-15 14:32:01 [INFO] [uuid=abc-def-123] [main.py:93] simulate_single_consulta_cnpj() - [DOC:12345678000195] Consultando fonte: receita_federal
```

Campos extraidos pelo agent:
- **timestamp**: `2024-01-15 14:32:01`
- **level**: `INFO`
- **uuid**: `abc-def-123` (correlacao de request)
- **module**: `main.py:93`
- **function**: `simulate_single_consulta_cnpj`
- **documento**: `12345678000195` (CNPJ/CPF)
- **message**: texto completo

---

## Tipos de Worker Simulados

| Logger              | Arquivo gerado                              | Simula                        |
|---------------------|---------------------------------------------|-------------------------------|
| `worker_core`       | `dados_basicos_worker_consulta_core_*.log`  | Worker Celery principal       |
| `worker_quick`      | `dados_basicos_worker_consulta_quick_*.log` | Worker Celery rapido          |
| `worker_background` | `dados_basicos_worker_background_*.log`     | Worker Celery background      |
| `api`               | `dados_basicos_api_*.log`                   | FastAPI/Gunicorn              |

---

## Estrutura do Projeto

```
POC_Agente/
├── app/
│   ├── __init__.py
│   ├── main.py                  # FastAPI — endpoints de simulacao
│   └── services/
│       ├── __init__.py
│       └── log_service.py       # LogService adaptado (mesmo formato da API real)
├── logstream/                   # LogStream — sistema completo de observabilidade
│   ├── agent/                   # Sidecar Go — observa .log files via fsnotify
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── ...                  # watcher, parser, sender, buffer, healthmon
│   ├── server/                  # Servidor Go — gRPC + REST + WebSocket
│   │   ├── Dockerfile
│   │   ├── main.go
│   │   └── ...                  # grpc, hub, store, api, auth, cleaner
│   ├── dashboard/               # Frontend React — visualizacao em tempo real
│   │   ├── Dockerfile
│   │   ├── nginx.conf
│   │   └── src/                 # React 18 + TypeScript + TailwindCSS + ApexCharts
│   └── proto/
│       └── logs.proto           # Schema gRPC (contrato agent <-> server)
├── docker-compose.yml           # Stack completa (5 servicos)
├── Dockerfile                   # Build da mock API
├── requirements.txt             # Dependencias Python
├── .env.example                 # Template — config da mock API
├── .env.agent.example           # Template — config do log agent
├── .env.server.example          # Template — config do log server
├── .gitignore
└── README.md
```

---

## Comportamento Padrao

Ao iniciar, a mock API **automaticamente** comeca a gerar logs a cada 3 segundos, simulando:
- **40%** consultas CNPJ
- **25%** consultas CPF
- **25%** fluxos completos (API -> worker -> background)
- **10%** erros variados (timeout, connection error, service unavailable)

Isso garante que, ao abrir o dashboard, ja havera logs fluindo em tempo real.

---

## Operacoes

```bash
# Ver logs de todos os containers
docker compose logs -f

# Ver logs so da mock API
docker compose logs -f mock-api

# Ver logs do agent
docker compose logs -f log-agent

# Parar tudo
docker compose down

# Parar e limpar volumes
docker compose down -v

# Rebuild apos alteracoes
docker compose up --build -d
```

---

## Relacao com o Projeto Principal

Este POC demonstra o funcionamento do **LogStream**, sistema de observabilidade desenvolvido para a **API_DadosBasicos**. Na producao:

- Os **workers Celery** (core, quick, background) processam consultas reais de CNPJ/CPF via RabbitMQ
- O **log agent** roda como sidecar observando o volume compartilhado de logs
- O **log server** roda em VPS centralizada recebendo logs de multiplas APIs
- O **dashboard** permite monitoramento em tempo real com filtros por servico, nivel, documento, task_id
