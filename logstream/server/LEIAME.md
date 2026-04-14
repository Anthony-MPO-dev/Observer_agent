# LogStream Server — Documentação Completa

> Para desenvolvedores Python sem experiência em Go.

---

## O que é o Server?

O Server é o **componente central** do sistema. Ele:
- Recebe logs dos agents via gRPC
- Persiste logs em disco (arquivos `.jsonl`)
- Mantém metadados em SQLite (serviços, configurações)
- Transmite logs ao vivo para o dashboard via WebSocket
- Expõe uma REST API para o dashboard

**Analogia Python:** Seria como uma aplicação FastAPI com SQLAlchemy + SQLite + WebSocket + um servidor gRPC, tudo no mesmo processo.

---

## Estrutura de arquivos

```
server/
├── main.go          # Ponto de entrada — inicializa e conecta tudo
├── codec.go         # Registra codec JSON para o gRPC
├── config/
│   └── config.go    # Lê variáveis de ambiente
├── auth/
│   └── auth.go      # Geração e validação de JWT
├── db/
│   └── db.go        # SQLite — metadados de serviços e configurações
├── store/
│   └── store.go     # Persistência de logs em arquivos .jsonl
├── hub/
│   └── hub.go       # Pub/Sub em memória (fan-out para WebSocket)
├── grpc/
│   └── server.go    # Implementa os métodos gRPC (Register, StreamLogs, etc.)
├── gateway/
│   └── ws.go        # WebSocket — streaming de logs para o dashboard
├── api/
│   └── api.go       # REST API (login, serviços, histórico, stats)
├── cleaner/
│   └── cleaner.go   # Job diário de limpeza de logs antigos
└── pb/
    ├── logs.pb.go       # Structs de mensagens
    └── logs_grpc.pb.go  # Interface gRPC
```

---

## main.go — Ponto de entrada

**O que faz:** Inicializa todos os subsistemas e os conecta.

### Ordem de inicialização:
```
1. config.Load()          → lê variáveis de ambiente
2. db.Open()              → abre SQLite, aplica schema
3. store.New()            → prepara diretório de logs
4. hub.New()              → cria pub/sub em memória
5. cleaner.New().Start()  → goroutine de limpeza diária
6. grpc.NewServer()       → servidor gRPC (porta 9090)
7. api.New()              → REST handlers
8. gateway.Handler()      → WebSocket handler
9. http.ListenAndServe()  → servidor HTTP (porta 8080)
```

### Shutdown gracioso:
O servidor escuta `SIGTERM` / `SIGINT` (sinais do sistema operacional, como `Ctrl+C`):
```go
signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
<-quit  // bloqueia até receber sinal
// então faz cleanup ordenado
```
> Em Python: `signal.signal(signal.SIGTERM, handler)`

---

## codec.go — Codec JSON para gRPC

**O que faz:** Faz o gRPC usar JSON em vez de Protobuf binário.

```go
func init() {
    grpcencoding.RegisterCodec(jsonCodec{})
}
```

> `init()` em Go é chamado automaticamente quando o pacote é importado.  
> Em Python seria como um `__init__.py` que executa código ao importar o módulo.

**Por que JSON em vez de Protobuf?**  
Simplicidade: as structs `pb.LogEntry` não precisam de geração de código complexa. O JSON funciona perfeitamente para este caso de uso e é mais fácil de debugar.

---

## config/config.go — Configuração

### Variáveis do servidor:

| Variável             | Padrão              | Descrição                               |
|----------------------|---------------------|-----------------------------------------|
| `GRPC_PORT`          | `":9090"`           | Porta do servidor gRPC                  |
| `HTTP_PORT`          | `":8080"`           | Porta do servidor HTTP/WebSocket        |
| `DATA_DIR`           | `"./data"`          | Onde fica o SQLite                      |
| `LOGS_DIR`           | `"./logs"`          | Onde ficam os arquivos .jsonl           |
| `JWT_SECRET`         | `"changeme"`        | Segredo para assinar tokens JWT         |
| `ADMIN_USER`         | `"admin"`           | Usuário do dashboard                    |
| `ADMIN_PASS`         | `"admin"`           | Senha do dashboard                      |
| `DEFAULT_TTL_DAYS`   | `30`                | Dias padrão para reter logs             |
| `CLEANUP_HOUR`       | `3`                 | Hora UTC para rodar limpeza             |
| `TLS_ENABLED`        | `false`             | Habilita TLS                            |

---

## auth/auth.go — Autenticação JWT

**O que é JWT?** JSON Web Token — um "bilhete assinado" que prova que o portador foi autenticado.

**Analogia Python:**
```python
import jwt  # PyJWT

def create_token(username: str, secret: str) -> str:
    payload = {
        "sub": username,
        "exp": datetime.utcnow() + timedelta(hours=24)
    }
    return jwt.encode(payload, secret, algorithm="HS256")

def validate_token(token: str, secret: str) -> dict:
    return jwt.decode(token, secret, algorithms=["HS256"])
```

### Funções:
- **`NewToken(username, secret)`** → cria JWT válido por 24h
- **`ValidateToken(tokenStr, secret)`** → valida e retorna claims
- **`Middleware(secret, next)`** → middleware HTTP que bloqueia requisições sem token
- **`LoginHandler(user, pass, secret)`** → handler para `POST /api/auth/login`

### Como o token é passado:
- **REST API:** `Authorization: Bearer <token>` no header
- **WebSocket:** `/ws/logs?token=<token>` na query string (browsers não suportam headers customizados em WebSocket)

---

## db/db.go — Banco de Dados SQLite

**O que faz:** Persiste metadados sobre serviços (não os logs em si — esses ficam em arquivos).

**Analogia Python com SQLAlchemy/SQLite:**
```python
import sqlite3

conn = sqlite3.connect("logstream.db")
conn.execute("PRAGMA journal_mode=WAL")  # melhor para acesso concorrente
```

### Schema (tabelas):

```sql
-- Serviços registrados
CREATE TABLE services (
    id        TEXT PRIMARY KEY,  -- service_id do agente
    name      TEXT NOT NULL,     -- nome legível
    last_seen INTEGER,           -- timestamp da última atividade (ms)
    status    TEXT DEFAULT 'offline',  -- 'online' | 'offline'
    agent_id  TEXT,              -- UUID do agente atual
    version   TEXT               -- versão do agente
);

-- Configurações por serviço (ajustadas via dashboard)
CREATE TABLE service_configs (
    service_id TEXT PRIMARY KEY,
    ttl_days   INTEGER DEFAULT 30,   -- dias para reter logs
    min_level  TEXT    DEFAULT 'INFO', -- nível mínimo aceito
    batch_size INTEGER DEFAULT 100,  -- logs por batch do agente
    flush_ms   INTEGER DEFAULT 500,  -- intervalo de flush do agente
    enabled    INTEGER DEFAULT 1     -- 1=ativo, 0=desativado
);

-- Estatísticas de runtime dos agentes
CREATE TABLE agent_stats (
    agent_id      TEXT PRIMARY KEY,
    service_id    TEXT,
    buffer_used   INTEGER,  -- tamanho atual do ring buffer
    dropped_total INTEGER,  -- total de logs descartados por overflow
    logs_per_sec  REAL,     -- taxa atual de logs
    updated_at    INTEGER   -- timestamp da última atualização (ms)
);
```

### Pragmas SQLite:
```sql
PRAGMA journal_mode=WAL;      -- Write-Ahead Logging: melhor para múltiplos readers
PRAGMA synchronous=NORMAL;    -- Balanço entre durabilidade e performance
PRAGMA foreign_keys=ON;       -- Ativa verificação de chaves estrangeiras
```

### Métodos principais:
- **`UpsertService()`** → insere ou atualiza serviço; também cria config default se não existir
- **`SetServiceStatus()`** → marca como "online" ou "offline"
- **`GetConfig()`** → retorna configuração do serviço (ou defaults se não tiver)
- **`UpsertConfig()`** → atualiza configuração
- **`ListServices()`** → lista todos os serviços com suas configs (JOIN)
- **`UpdateAgentStats()`** → salva métricas do heartbeat

---

## store/store.go — Persistência de Logs em Arquivos

**O que faz:** Salva cada `LogEntry` como uma linha JSON em um arquivo `.jsonl` diário.

### Estrutura em disco:
```
logs/
└── worker-core/
    ├── 2024-01-14.jsonl   ← um arquivo por dia por serviço
    ├── 2024-01-15.jsonl
    └── 2024-01-16.jsonl
```

### Por que `.jsonl` (JSON Lines)?
Cada linha é um JSON completo e independente. Facilita:
- Escrita incremental (apenas append)
- Leitura linha por linha sem carregar tudo na memória
- Queries simples com `bufio.Scanner` (como `for line in file` do Python)

### Write — escrita de log:
```go
// Equivalente Python:
# with open(path, 'a') as f:
#     f.write(json.dumps(entry) + '\n')
```
Usa `sync.Mutex` para evitar que duas goroutines escrevam no mesmo arquivo ao mesmo tempo.

### Query — busca histórica:
1. Descobre quais arquivos de data cobrem o intervalo `from_ts` → `to_ts`
2. Para cada arquivo, lê linha por linha com `bufio.Scanner`
3. Para cada linha, faz `json.Unmarshal` e verifica os filtros
4. Respeita `limit` e `offset` para paginação

### DeleteOlderThan — limpeza:
Remove arquivos `.jsonl` com data anterior ao cutoff. Comparação lexicográfica de strings funciona porque o formato `YYYY-MM-DD` é ordenável.

---

## hub/hub.go — Pub/Sub em Memória

**O que é?** Um "roteador" que recebe logs dos agents (via gRPC) e os distribui para todos os dashboards conectados (via WebSocket).

**Analogia Python:**
```python
import asyncio
from typing import Dict

class Hub:
    def __init__(self):
        self.subscribers: Dict[str, asyncio.Queue] = {}
    
    def subscribe(self, filter) -> asyncio.Queue:
        q = asyncio.Queue(maxsize=256)
        self.subscribers[id(q)] = q
        return q
    
    async def publish(self, entry):
        for q in self.subscribers.values():
            if matches(entry, q.filter):
                try:
                    q.put_nowait(entry)  # não bloqueia
                except asyncio.QueueFull:
                    pass  # subscriber lento: descarta
```

### Como funciona:
1. `hub.Subscribe(filter)` → cria um `Subscriber` com um channel Go de 256 posições
2. `hub.Publish(entry)` → para cada subscriber, se o log satisfaz o filtro, envia ao channel sem bloquear
3. `hub.Unsubscribe(id)` → remove o subscriber e fecha o channel

### Thread safety:
- `Publish` usa `sync.RWMutex` com `RLock` (múltiplos readers ao mesmo tempo)
- `Subscribe`/`Unsubscribe` usam `Lock` (writer exclusivo)

> Em Python: `threading.RLock` ou `asyncio.Lock`

### Filter:
O subscriber define quais logs quer receber:
```go
Filter{
    ServiceIDs: []string{"worker-core", "worker-quick"},
    Levels:     []string{"ERROR", "WARNING"},
    TaskID:     "abc-123",
    Documento:  "12345678000195",
    Module:     "consulta",
    Search:     "texto livre",
}
```
Todos os campos são opcionais. Se vazio, aceita tudo.

---

## grpc/server.go — Implementação dos Métodos gRPC

**O que faz:** Implementa os métodos definidos em `logs_grpc.pb.go` — a "lógica de negócio" da interface gRPC.

**Analogia Python:** São como os handlers de rota do FastAPI:
```python
@app.post("/register")
async def register(request: RegisterRequest) -> RegisterResponse: ...
```

### Register:
```
Agente → [RegisterRequest com AgentInfo] → Server
Server → UpsertService no SQLite → GetConfig
Server → [RegisterResponse com ServiceConfig] → Agente
```

### StreamLogs (mais importante):
```
Loop:
  Agente → [LogBatch] → Server
  Server → Write em disco (store.Write)
  Server → Publica no Hub (hub.Publish) → WebSocket → Dashboard
  Server → [StreamResponse com ack] → Agente
```

Se o agente desconectar, o serviço é marcado como "offline" no SQLite.

### Heartbeat:
```
A cada 10s:
  Agente → [stats: buffer, dropped, logs/sec] → Server
  Server → atualiza SQLite
  Server → [ServiceConfig atualizado] → Agente
```
Permite que o dashboard mude `batch_size` e `flush_ms` em tempo real.

### Subscribe:
Usado pelo dashboard (via gRPC — mas o dashboard usa WebSocket via gateway):
```
Dashboard → [filtros] → Server
Server → hub.Subscribe(filtros)
Loop: recebe do Hub → envia ao dashboard
```

### Query:
Busca histórica — chama `store.Query()` e retorna os resultados.

### UpdateConfig:
Salva configuração nova no SQLite. O agente vai receber no próximo heartbeat.

---

## gateway/ws.go — WebSocket para o Dashboard

**O que faz:** Aceita conexões WebSocket do dashboard e transmite logs em tempo real.

**URL:** `GET /ws/logs?token=JWT&service_ids=a,b&levels=INFO,ERROR&...`

### Fluxo:
```
1. Valida JWT do query param
2. Parseia filtros dos query params
3. Faz upgrade HTTP → WebSocket
4. hub.Subscribe(filtros)  ← registra no hub
5. Loop:
   - entry chegou → json.Marshal → conn.WriteMessage
   - ticker 30s   → conn.WriteMessage(PingMessage)
   - cliente fechou → hub.Unsubscribe → retorna
```

### Por que JWT no query param?
Browsers não permitem enviar headers customizados ao abrir uma conexão WebSocket. O workaround padrão é passar o token na URL.

### Write deadline:
```go
conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
```
Se o dashboard não conseguir receber a mensagem em 10s, a conexão é considerada morta e fechada.

---

## api/api.go — REST API

**O que faz:** Expõe endpoints HTTP para o dashboard React.

### Rotas:

| Método | Rota                              | Descrição                         |
|--------|-----------------------------------|-----------------------------------|
| POST   | `/api/auth/login`                 | Login, retorna JWT                |
| GET    | `/api/services`                   | Lista todos os serviços           |
| GET    | `/api/services/{id}/config`       | Configuração de um serviço        |
| PUT    | `/api/services/{id}/config`       | Atualiza configuração             |
| GET    | `/api/logs?service_id=X&...`      | Busca histórica de logs           |
| DELETE | `/api/logs/{service_id}?days=N`   | Remove logs antigos manualmente   |
| GET    | `/api/stats`                      | Estatísticas gerais               |

### Autenticação:
- `POST /api/auth/login` → pública
- Todo o resto → protegido por `auth.Middleware`

### Formato da resposta de `GET /api/logs`:
```json
{
  "entries": [...],
  "total": 150,
  "limit": 100,
  "offset": 0
}
```

### Filtros de `GET /api/logs`:
- `service_id` (obrigatório)
- `level` (repetível: `?level=INFO&level=ERROR`)
- `task_id`, `documento`, `module`, `search`
- `from` / `to` (Unix milliseconds)
- `limit` / `offset` (paginação)

---

## cleaner/cleaner.go — Limpeza Automática

**O que faz:** Uma goroutine que acorda todo dia num horário configurado e deleta arquivos `.jsonl` antigos.

**Analogia Python:**
```python
import schedule
import time

def cleanup():
    for service in db.list_services():
        ttl = service.config.ttl_days or 30
        cutoff = datetime.utcnow() - timedelta(days=ttl)
        store.delete_older_than(service.id, cutoff)

schedule.every().day.at("03:00").do(cleanup)
```

### Como funciona:
1. Calcula quando é a próxima ocorrência de `CLEANUP_HOUR` (ex: 3h UTC)
2. Dorme até lá com `time.After()`
3. Executa `runOnce()` que itera todos os serviços
4. Para cada serviço, usa o TTL configurado no dashboard (ou o padrão)
5. Remove arquivos `.jsonl` com data anterior ao cutoff
6. Volta ao passo 1

---

## pb/ — Definições de Mensagens

Idêntico ao do agent, mas com campos adicionais em `LogEntry` para o dashboard:
- `ServiceName` — nome legível (para exibir no dashboard)
- `WorkerType`, `Queue`, `LogFile` — detalhes do worker
- `UnixTs`, `TimestampStr` — timestamps em formatos alternativos
- `IsContinuation` — indica linha de continuação de stack trace

Veja o [README do agent](../agent/LEIAME.md#pblogspbgo--definições-de-mensagens) para a explicação geral de structs pb.
