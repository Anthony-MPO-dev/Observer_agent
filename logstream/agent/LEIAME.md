# LogStream Agent — Documentação Completa

> Para desenvolvedores Python sem experiência em Go.

---

## O que é o Agent?

O Agent é um processo Go que roda **ao lado dos workers Python** dentro do container Docker.  
Ele faz o equivalente a isso em Python:

```python
import time
import re
from watchdog.observers import Observer
from watchdog.events import FileSystemEventHandler

# Monitora arquivos .log e envia cada nova linha para um servidor central
```

A diferença é que o Agent em Go é extremamente eficiente: usa pouquíssima memória e CPU mesmo monitorando dezenas de arquivos simultâneos.

---

## Estrutura de arquivos

```
agent/
├── main.go          # Ponto de entrada — inicializa tudo e conecta as peças
├── config/
│   └── config.go    # Lê variáveis de ambiente (.env / Docker env vars)
├── buffer/
│   └── buffer.go    # Buffer circular para quando o servidor está offline
├── parser/
│   └── parser.go    # Converte linha de texto em struct LogEntry
├── watcher/
│   └── watcher.go   # Monitora arquivos .log com fsnotify + tail
├── sender/
│   └── sender.go    # Envia logs ao servidor via gRPC
└── pb/
    ├── logs.pb.go       # Definições de structs (LogEntry, LogBatch, etc.)
    └── logs_grpc.pb.go  # Interface gRPC gerada (cliente/servidor)
```

---

## main.go — O ponto de entrada

**O que faz:** Inicializa todos os componentes e os conecta como um pipeline:

```
arquivos .log → [watcher] → [parser] → [buffer de entrada] → [sender] → servidor gRPC
```

### Componentes criados:
1. **`buffer.New(cfg.BufferSize)`** — cria o ring buffer para armazenar logs quando offline
2. **`parser.New(...)`** — cria o parser com o `service_id` e `service_name`
3. **`watcher.New(cfg.LogVolume, p)`** — cria o monitor de arquivos
4. **`sender.New(cfg, buf, agentID)`** — cria o enviador gRPC

### Pipeline de dados:
```go
// Goroutine que conecta watcher → sender
go func() {
    for entry := range w.EntryCh() {
        s.Send(entry)
    }
}()
```
> Em Python: `for entry in watcher_queue: sender_queue.put(entry)`

### Endpoint `/metrics`:
Expõe métricas em texto simples para monitoramento:
- `connected` — se está conectado ao servidor
- `buffer_used` — quantos logs estão no buffer offline
- `dropped_total` — quantos logs foram descartados por overflow
- `logs_per_sec` — taxa de logs por segundo

---

## config/config.go — Configuração

Lê variáveis de ambiente. Em Python seria:

```python
import os
SERVICE_ID = os.getenv("SERVICE_ID", "default")
```

### Variáveis principais:

| Variável          | Padrão          | Descrição                              |
|-------------------|-----------------|----------------------------------------|
| `SERVICE_ID`      | `"default"`     | Identificador único do serviço         |
| `SERVICE_NAME`    | `"Default"`     | Nome legível do serviço                |
| `SERVER_ADDR`     | `"server:9090"` | Endereço do servidor gRPC              |
| `LOG_VOLUME`      | `"/logs"`       | Diretório com arquivos .log            |
| `BATCH_SIZE`      | `100`           | Logs por batch antes de enviar         |
| `FLUSH_MS`        | `500`           | Intervalo de flush em milissegundos    |
| `BUFFER_SIZE`     | `10000`         | Capacidade do ring buffer offline      |
| `TLS_ENABLED`     | `false`         | Usa TLS na conexão gRPC                |
| `METRICS_PORT`    | `":2112"`       | Porta do servidor de métricas          |

---

## buffer/buffer.go — Ring Buffer

**O que é um ring buffer?** É uma fila circular de tamanho fixo. Quando está cheio, o item mais antigo é descartado para dar lugar ao novo.

**Analogia Python:**
```python
from collections import deque

class RingBuffer:
    def __init__(self, maxsize):
        self.buffer = deque(maxlen=maxsize)  # deque com maxlen funciona igual!
        self.dropped_count = 0
    
    def push(self, item):
        if len(self.buffer) == self.buffer.maxlen:
            self.dropped_count += 1  # item mais antigo vai ser descartado
        self.buffer.append(item)
    
    def drain_all(self):
        items = list(self.buffer)
        self.buffer.clear()
        return items
```

**Por que usar isso?** Quando o servidor está offline, os logs continuam chegando. Em vez de perder tudo ou travar o sistema, o agent armazena em memória até a conexão voltar. Se o buffer encher, os logs mais antigos são descartados.

### Métodos:
- **`Push(entry)`** — adiciona ao buffer; se cheio, descarta o mais antigo
- **`DrainAll()`** — remove e retorna todos os itens (usado quando a conexão volta)
- **`Len()`** — quantos itens tem atualmente

### Thread safety:
O buffer usa `sync.Mutex` — equivalente ao `threading.Lock()` do Python. Isso garante que múltiplas goroutines possam acessar o buffer sem corromper os dados.

---

## parser/parser.go — Parser de Log

**O que faz:** Converte uma linha de texto do arquivo .log em um `LogEntry` estruturado.

### Formato esperado dos logs:

**Com documento (CNPJ/CPF):**
```
2024-01-15 14:32:01 - [INFO] - [consulta.cnpj] - [DOC:12345678000195] Consultando CNPJ
```

**Sem documento:**
```
2024-01-15 14:32:01 - [WARNING] - [worker.main] - Iniciando processamento
```

### Como funciona:
O parser usa **expressões regulares** (igual ao Python `re` module) para extrair cada campo.

```go
// Equivalente Python:
# import re
# pattern = re.compile(r'(?P<timestamp>\d{4}-...) - \[(?P<level>\w+)\] - ...')
# match = pattern.match(line)
```

### ParseFilename — extrai metadados do nome do arquivo:
Os arquivos têm nomes como:
- `dadosBasicos_worker_consulta_<uuid>_2024-01-15_14-30-00.log` → `worker_type="core"`, `task_id=<uuid>`
- `dadosBasicos_quick_2024-01-15_14-30-00.log` → `worker_type="quick"`

### ParseLine — converte linha em LogEntry:
1. Testa o regex `reWithDoc` (com documento)
2. Se não bater, testa `reWithoutDoc` (sem documento)
3. Converte o timestamp para Unix milliseconds
4. Preenche `LogEntry` com todos os campos
5. Adiciona metadados do arquivo em `Extra` map

### ParseLevel — mapeia nível de texto para inteiro:
```
"DEBUG"    → 0 (LogLevel_DEBUG)
"INFO"     → 1 (LogLevel_INFO)
"WARNING"  → 2 (LogLevel_WARNING)
"ERROR"    → 3 (LogLevel_ERROR)
"CRITICAL" → 4 (LogLevel_FATAL)
```

---

## watcher/watcher.go — Monitor de Arquivos

**O que faz:** Monitora um diretório e "segue" (tail) cada arquivo `.log` novo.

**Analogia Python com watchdog:**
```python
class Handler(FileSystemEventHandler):
    def on_created(self, event):
        if event.src_path.endswith('.log'):
            start_tailing(event.src_path)
```

### Como funciona:

1. **Ao iniciar:** Encontra todos os `.log` existentes e começa a "tailed" do fim (não relê o histórico)
2. **Novo arquivo:** `fsnotify` detecta criação → inicia tail da nova arquivo
3. **Nova linha:** `nxadm/tail` lê a linha → passa ao parser → envia ao channel `entryCh`

### startTail — goroutine por arquivo:
Cada arquivo tem sua própria goroutine de monitoramento. É como criar uma thread Python para cada arquivo:

```python
import threading

def tail_file(filepath):
    with open(filepath, 'r') as f:
        f.seek(0, 2)  # vai para o fim
        while True:
            line = f.readline()
            if line:
                entry = parser.parse_line(line)
                entry_queue.put(entry)
            else:
                time.sleep(0.1)

threading.Thread(target=tail_file, args=(filepath,), daemon=True).start()
```

### fromEnd:
- `fromEnd=true`: arquivos que já existiam → começa do fim (não relê logs antigos)
- `fromEnd=false`: arquivo novo → começa do início

---

## sender/sender.go — Enviador gRPC

**O que faz:** Mantém a conexão com o servidor gRPC e envia os logs em batches.

**Analogia Python:**
```python
import grpc  # ou httpx com streaming

class Sender:
    def start(self):
        while True:
            try:
                with grpc.insecure_channel('server:9090') as channel:
                    self.register()        # apresenta-se ao servidor
                    self.stream_loop()     # envia logs em loop
            except Exception as e:
                print(f"Erro: {e}, reconectando em {backoff}s...")
                time.sleep(backoff)
```

### Fluxo de conexão:
```
1. dial()          → abre conexão TCP com o servidor
2. register()      → envia AgentInfo, recebe ServiceConfig inicial
3. heartbeatLoop() → goroutine que envia stats a cada 10s
4. streamLoop()    → loop principal de envio de batches
```

### streamLoop — o loop principal:
```
┌─────────────────────────────────────────────────────────┐
│ Timer (flushMs)  → envia batch acumulado                │
│ entry chegou     → adiciona ao batch                    │
│ batch cheio      → envia imediatamente                  │
│ ctx.Done()       → flush final + encerra                │
│ erro do servidor → retorna erro → reconecta             │
└─────────────────────────────────────────────────────────┘
```

### JSON codec:
O gRPC normalmente usa **Protobuf** (formato binário), mas este projeto registra um codec JSON custom:
```go
func init() {
    grpcencoding.RegisterCodec(jsonCodec{})
}
```
> Em Python seria como configurar um serializer custom num cliente gRPC.
> Na prática, todos os `LogEntry` são enviados como JSON puro, não como binário.

### Backoff exponencial:
Se a conexão falha, o agent espera antes de tentar novamente:
- Tentativa 1: espera 1s
- Tentativa 2: espera 2s
- Tentativa 3: espera 4s
- ...
- Máximo: 60s

### applyConfig:
O servidor pode enviar configurações atualizadas (batch_size, flush_ms) nas respostas.
O agent aplica essas configurações em tempo real sem precisar reiniciar.

---

## pb/logs.pb.go — Definições de Mensagens

**O que é:** Definições das structs de dados compartilhadas entre agent e servidor.

Em Python seria um conjunto de classes Pydantic:
```python
from pydantic import BaseModel

class LogEntry(BaseModel):
    service_id: str = ""
    level: int = 1  # 0=DEBUG, 1=INFO, 2=WARNING, 3=ERROR, 4=FATAL
    message: str = ""
    timestamp: int = 0
    task_id: str = ""
    documento: str = ""
    module: str = ""
    agent_id: str = ""
    extra: dict[str, str] = {}
```

### Por que tem métodos `Reset()`, `ProtoMessage()`, etc.?
São necessários para satisfazer a interface gRPC do Go. Você não precisa chamá-los diretamente. É como implementar métodos mágicos do Python (`__str__`, `__repr__`) para fazer o gRPC funcionar.

### Por que não tem `protoimpl`?
O projeto usa um **codec JSON custom** em vez do codec Protobuf padrão. Isso significa que as structs não precisam da maquinaria interna do Protobuf (`protoimpl.MessageState`, etc.) — basta ter as tags JSON corretas. Isso simplifica muito o código.

### Tags JSON (`json:"field_name,omitempty"`):
- `json:"service_id"` → o campo se chama `service_id` no JSON (snake_case)
- `omitempty` → se o valor for zero/vazio, não inclui no JSON serializado

---

## pb/logs_grpc.pb.go — Interface gRPC

**O que é:** Define o **contrato** de comunicação entre agent e servidor.

Em Python seria como um cliente de API:
```python
class LogServiceClient:
    def register(self, request: RegisterRequest) -> RegisterResponse: ...
    def stream_logs(self, batches: Iterator[LogBatch]) -> Iterator[StreamResponse]: ...
    def heartbeat(self, request: HeartbeatRequest) -> HeartbeatResponse: ...
```

### Métodos disponíveis:
| Método         | Tipo               | Descrição                                      |
|---------------|--------------------|------------------------------------------------|
| `Register`    | Unary (1 req/resp) | Agent apresenta-se ao servidor                 |
| `StreamLogs`  | Bidirecional       | Agent envia batches; servidor confirma recebimento |
| `Heartbeat`   | Unary              | Agent envia stats; servidor retorna config atual |
| `Subscribe`   | Server streaming   | Cliente recebe logs em tempo real (usado pelo dashboard via gRPC, mas também via WebSocket) |
| `Query`       | Unary              | Busca no histórico                             |
| `UpdateConfig`| Unary              | Atualiza configuração de um serviço            |
