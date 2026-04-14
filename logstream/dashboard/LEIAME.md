# LogStream Dashboard — Documentação Completa

> Para desenvolvedores Python sem experiência em frontend/React.

---

## O que é o Dashboard?

O Dashboard é a **interface visual** do sistema. É uma aplicação web em **React + TypeScript** que:
- Exibe logs em tempo real via WebSocket
- Permite filtrar por serviço, nível, task ID, documento, módulo
- Permite consultar histórico de logs
- Permite configurar TTL e nível mínimo por serviço

**Tecnologias:**
- **React 18** — biblioteca de UI (equivalente ao Jinja2 + JavaScript no Flask, mas muito mais interativo)
- **TypeScript** — JavaScript com tipagem estática (equivalente a usar type hints no Python)
- **Tailwind CSS** — framework de CSS com classes utilitárias
- **Vite** — bundler/transpilador (compila o código para o browser)
- **nginx** — serve os arquivos estáticos + faz proxy reverso para a API

---

## Como funciona no Docker

O Dockerfile tem **dois estágios**:

### Estágio 1: Build
```dockerfile
FROM node:20-alpine AS build
RUN npm install --legacy-peer-deps
RUN npm run build  # gera arquivos estáticos em /app/dist
```

### Estágio 2: Serve
```dockerfile
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
```

### Por que nginx?
O nginx faz duas coisas:
1. Serve os arquivos HTML/JS/CSS do frontend
2. Faz **proxy reverso** para o servidor Go:
   - `/api/*` → `http://log-server:8080/api/*` (REST API)
   - `/ws/*` → `http://log-server:8080/ws/*` (WebSocket)

Isso significa que o browser só faz requisições para um endereço (o dashboard), sem precisar saber onde o servidor Go está.

---

## Estrutura de arquivos

```
dashboard/src/
├── App.tsx              # Componente raiz — roteamento e layout principal
├── main.tsx             # Ponto de entrada React
├── index.css            # Estilos globais (Tailwind + scrollbar custom)
├── types.ts             # Definições de tipos TypeScript
│
├── components/
│   ├── LoginPage.tsx    # Tela de login
│   ├── ServiceList.tsx  # Sidebar com lista de serviços
│   ├── FilterBar.tsx    # Barra de filtros (nível, task_id, etc.)
│   ├── LogLine.tsx      # Uma linha de log formatada
│   ├── LogViewer.tsx    # Viewer de logs ao vivo (WebSocket)
│   ├── HistoryViewer.tsx # Viewer de logs históricos (REST)
│   ├── ConfigPanel.tsx  # Painel de configuração de serviço
│   └── StatsBar.tsx     # Barra de estatísticas no topo
│
├── hooks/
│   ├── useAuth.ts       # Estado de autenticação (token JWT)
│   ├── useServices.ts   # Lista de serviços (polling REST)
│   └── useLogStream.ts  # Conexão WebSocket
│
└── lib/
    ├── api.ts           # Funções para chamar a REST API
    ├── ws.ts            # Gerenciador de conexão WebSocket
    └── ttl.ts           # TTL do token (auto-logout)
```

---

## types.ts — Tipos TypeScript

**O que é TypeScript?** É JavaScript com tipagem estática. Em vez de:
```javascript
// JavaScript — sem tipo, pode dar erro em runtime
function greet(user) {
    return "Hello " + user.name  // e se user for null?
}
```
Você escreve:
```typescript
// TypeScript — erro detectado em tempo de compilação
interface User { name: string }
function greet(user: User): string {
    return "Hello " + user.name
}
```

**Equivalente Python com type hints:**
```python
from typing import TypedDict

class LogEntry(TypedDict):
    id: str
    service_id: str
    level: int          # 0=DEBUG, 1=INFO, 2=WARNING, 3=ERROR, 4=FATAL
    message: str
    timestamp: int      # Unix milliseconds
    task_id: str
    documento: str
    module: str
    worker_type: str
    queue: str
    extra: dict[str, str]
```

### Tipos principais:
- **`LogEntry`** — uma linha de log (espelha o `pb.LogEntry` do servidor)
- **`Service`** — um serviço registrado (`id`, `name`, `status`, `last_seen`, `config`)
- **`ServiceConfig`** — configurações de um serviço
- **`LogFilter`** — filtros ativos na interface
- **`LOG_LEVELS_FILTER`** — lista de níveis disponíveis para filtro

---

## hooks/ — Lógica de Estado

**O que são hooks?** Em React, hooks são funções especiais que gerenciam estado e efeitos colaterais.

**Analogia Python:** São como métodos de uma classe que encapsulam lógica, mas expostos como funções reutilizáveis.

### useAuth.ts — Autenticação
```typescript
const { token, login, logout } = useAuth()
```
- Guarda o JWT no `localStorage` do browser (persiste ao recarregar a página)
- `login(username, password)` → chama `POST /api/auth/login`
- `logout()` → remove o token

### useServices.ts — Lista de Serviços
```typescript
const { services, loading } = useServices(token)
```
- Chama `GET /api/services` a cada 5 segundos (polling)
- Retorna lista de serviços com status online/offline

**Equivalente Python:**
```python
import time
import threading

class ServicePoller:
    def __init__(self, token):
        self.services = []
        self._start_polling(token)
    
    def _start_polling(self, token):
        def poll():
            while True:
                self.services = api.get_services(token)
                time.sleep(5)
        threading.Thread(target=poll, daemon=True).start()
```

### useLogStream.ts — WebSocket
```typescript
const { entries, connected } = useLogStream(token, filter)
```
- Abre conexão WebSocket com o servidor
- Recebe logs em tempo real
- Aplica filtros localmente para evitar reconexão a cada mudança de filtro
- Mantém apenas as últimas N entradas em memória (para não estourar o browser)

---

## components/ — Componentes Visuais

**O que são componentes React?** São funções que retornam HTML. Cada componente recebe dados (`props`) e retorna o que deve ser exibido.

**Analogia Python (Flask/Jinja2):**
```python
# Python/Jinja2:
@app.route('/logs')
def logs_page():
    return render_template('logs.html', entries=entries, filter=filter)
```
```typescript
// React:
function LogViewer({ entries, filter }) {
    return <div>{entries.map(e => <LogLine entry={e} />)}</div>
}
```

### LoginPage.tsx
Formulário simples de usuário e senha. Chama `useAuth.login()` e redireciona para o dashboard.

### ServiceList.tsx
Sidebar esquerda com lista de serviços. Mostra:
- Ponto verde pulsante → serviço online
- Ponto cinza → offline com "visto por último há X minutos"
- Click → seleciona/deseleciona o serviço para filtrar logs

Usa `date-fns` para formatar tempo relativo em português (ex: "há 3 minutos").

### FilterBar.tsx
Barra horizontal com:
- Botões de toggle para cada nível (DEBUG, INFO, WARNING, ERROR, FATAL)
- Campos de texto para Task ID, Documento, Módulo, Busca livre
- Botão "Limpar" quando há filtros ativos

### LogLine.tsx
Renderiza uma única linha de log com:
- Timestamp formatado
- Badge colorido de nível (azul=INFO, amarelo=WARNING, vermelho=ERROR)
- Service name + módulo
- Mensagem do log
- Task ID e documento se presentes

### LogViewer.tsx
Componente principal de logs ao vivo:
- Usa `useLogStream` para receber via WebSocket
- Auto-scroll para o fim quando novos logs chegam
- Opção de pausar o scroll
- Mostra contador de logs recebidos

### HistoryViewer.tsx
Busca de logs históricos:
- Formulário com range de datas, filtros, paginação
- Chama `GET /api/logs` via REST
- Exibe resultados com paginação (Próxima/Anterior)

### ConfigPanel.tsx
Painel de configuração por serviço:
- `ttl_days` → slider ou input numérico
- `min_level` → dropdown de nível mínimo
- `batch_size` / `flush_ms` → configurações do agente
- `enabled` → liga/desliga coleta de logs
- Salva via `PUT /api/services/{id}/config`

### StatsBar.tsx
Barra no topo com:
- Total de serviços / online
- Taxa de logs (logs/seg)
- Conexão WebSocket (verde=conectado, vermelho=desconectado)

---

## lib/api.ts — Cliente REST

**O que faz:** Funções para chamar a REST API do servidor.

**Analogia Python:**
```python
import httpx

async def get_services(token: str) -> list[Service]:
    headers = {"Authorization": f"Bearer {token}"}
    resp = await httpx.get("/api/services", headers=headers)
    return resp.json()
```

### Funções:
- `login(username, password)` → POST /api/auth/login
- `getServices(token)` → GET /api/services
- `getServiceConfig(token, serviceId)` → GET /api/services/{id}/config
- `updateServiceConfig(token, serviceId, config)` → PUT /api/services/{id}/config
- `queryLogs(token, params)` → GET /api/logs?...
- `getStats(token)` → GET /api/stats

---

## lib/ws.ts — Cliente WebSocket

**O que faz:** Gerencia a conexão WebSocket com reconexão automática.

**Analogia Python:**
```python
import websockets
import asyncio

async def connect_and_stream(token, filter, on_message):
    url = f"ws://server/ws/logs?token={token}&..."
    async with websockets.connect(url) as ws:
        async for message in ws:
            entry = json.loads(message)
            on_message(entry)
```

### Funcionalidades:
- Reconexão automática com backoff exponencial
- Parseia cada mensagem JSON recebida como `LogEntry`
- Notifica o componente React via callback

---

## nginx.conf — Proxy Reverso

```nginx
server {
    listen 80;
    
    # Serve arquivos estáticos React
    location / {
        root /usr/share/nginx/html;
        try_files $uri /index.html;  # SPA: redireciona para index.html
    }
    
    # Proxy REST API para o servidor Go
    location /api/ {
        proxy_pass http://log-server:8080;
    }
    
    # Proxy WebSocket para o servidor Go
    location /ws/ {
        proxy_pass http://log-server:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;     # necessário para WebSocket
        proxy_set_header Connection "upgrade";       # necessário para WebSocket
    }
}
```

### Por que `try_files $uri /index.html`?
O React é uma **SPA** (Single Page Application). O roteamento é feito pelo JavaScript, não pelo servidor. Se o usuário acessa `/history` diretamente, o nginx não tem esse arquivo — então serve o `index.html` e o React trata a rota.

---

## Fluxo de dados no dashboard

### Login:
```
Browser → POST /api/auth/login → Server → JWT
Browser → salva JWT no localStorage
```

### Logs ao vivo:
```
Browser → GET /api/services (a cada 5s) → lista de serviços
Browser → WS /ws/logs?token=JWT&service_ids=X → Server
Server → Hub → envia LogEntry JSON → Browser → exibe
```

### Histórico:
```
Browser → GET /api/logs?service_id=X&from=...&to=... → Server
Server → lê arquivos .jsonl do store → retorna JSON
Browser → exibe com paginação
```

### Configuração:
```
Browser → PUT /api/services/{id}/config → Server
Server → salva no SQLite
Server → na próxima HeartbeatResponse → envia config ao agente
Agente → aplica novo batch_size/flush_ms em tempo real
```

---

## Variáveis de ambiente do build

```
VITE_API_URL=""   # URL base da API (vazio = relativo, usa o mesmo origem)
```

`VITE_*` variáveis são embutidas no JavaScript em build time pelo Vite. Não ficam disponíveis em runtime — por isso a URL é vazia (relativa), delegando ao nginx o roteamento.
