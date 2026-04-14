"""
POC LogStream — Mock API FastAPI

Simula workers Celery consumindo documentos de filas RabbitMQ.
Cada task da fila cria seu proprio arquivo de log com o task_id no nome,
exatamente como na API_DadosBasicos em producao.

Fluxo real:
  API recebe request -> chord(batch_tasks)(callback)
  CoreTask processa lote de docs (4-5 simultaneos) -> gera sub-tasks
  Cada sub-task (register, update_financeiro, teams) cria seu proprio log file

Arquivos gerados por execucao:
  dados_basicos_worker_consulta_{UUID_core_task}_{TS}.log
  dados_basicos_worker_background_{UUID_register}_{TS}.log
  dados_basicos_worker_background_{UUID_financeiro}_{TS}.log
  dados_basicos_worker_background_{UUID_teams}_{TS}.log
"""

import asyncio
import random
import uuid
from contextlib import asynccontextmanager
from datetime import datetime
from typing import Any

import httpx
import pytz
from fastapi import FastAPI
from pydantic import BaseModel, Field

from app.services.log_service import LogService, bind_uuid


# ---------------------------------------------------------------------------
# Dados mock realistas
# ---------------------------------------------------------------------------
CNPJS = [
    "12345678000195", "98765432000110", "11222333000181",
    "55444333000199", "77888999000155", "33222111000166",
    "44555666000177", "99888777000133", "22111000000144",
    "66777888000122", "10203040000150", "50607080000160",
    "15253545000170", "60708090000180", "25354555000190",
]

CPFS = [
    "12345678901", "98765432100", "11122233344",
    "55566677788", "99988877766", "33344455567",
    "77788899901", "22233344412", "88899900023",
]

NOMES_EMPRESA = [
    "Tech Solutions Ltda", "Comercio Digital SA", "Industria ABC Ltda",
    "Servicos Cloud ME", "Logistica Express SA", "Financeira Norte Ltda",
    "Agro Brasil SA", "Construtora Delta Ltda", "Pharma Vida SA",
    "Energia Solar ME", "Data Corp SA", "Varejo Plus Ltda",
]

SITUACOES_CADASTRAIS = ["ATIVA", "BAIXADA", "INAPTA", "SUSPENSA", "NULA"]

FONTES = {
    "receita": "Receita Federal",
    "bureau_1": "Bureau de Credito 1",
    "bureau_2": "Bureau de Credito 2",
}
FONTE_IDS = list(FONTES.keys())

QUEUES = {
    "core": "core-queue-dados-basicos",
    "quick": "quick-queue-dados-basicos",
    "background": "background-queue-dados-basicos",
}

# Logger da API (unico — nao rotaciona)
logger_api = LogService("api").get_logger()

# ---------------------------------------------------------------------------
# URL das dependencias mock (mesmo docker network)
# ---------------------------------------------------------------------------
DEPS_BASE_URL = "http://mock-dependencies:8001"

# ---------------------------------------------------------------------------
# Estado global
# ---------------------------------------------------------------------------
continuous_task: asyncio.Task | None = None


# ---------------------------------------------------------------------------
# Helpers — verificacao de dependencias
# ---------------------------------------------------------------------------
async def check_dependency(client: httpx.AsyncClient, dep_id: str) -> bool:
    try:
        r = await client.get(f"{DEPS_BASE_URL}/{dep_id}/health", timeout=3.0)
        return r.status_code == 200
    except Exception:
        return False


async def resolve_fonte(client: httpx.AsyncClient, preferred: str = "receita") -> tuple[str, bool]:
    fallback_chain = ["receita", "bureau_1", "bureau_2"]
    if preferred in fallback_chain:
        fallback_chain.remove(preferred)
        fallback_chain.insert(0, preferred)

    for i, fonte_id in enumerate(fallback_chain):
        if await check_dependency(client, fonte_id):
            return fonte_id, (i > 0)

    return "none", True


# ---------------------------------------------------------------------------
# Sub-task: register_consulta (background)
# ---------------------------------------------------------------------------
async def subtask_register_consulta(doc: str, parent_task_id: str):
    """
    Sub-task background: registra a consulta no banco.
    Cria seu proprio arquivo de log.
    """
    sub_id = f"{uuid.uuid4()}_background_task"
    log = LogService(f"worker_background_{sub_id}").get_logger()

    with bind_uuid(sub_id):
        log.info(
            f"[DISPATCH:{sub_id}] Task recebida da fila "
            f"'{QUEUES['background']}' | broker=rabbitmq | "
            f"parent_task={parent_task_id}"
        )
        log.info(f"[DOC:{doc}] Registrando consulta no banco de dados")
        await asyncio.sleep(random.uniform(0.1, 0.4))
        log.info(
            f"[DOC:{doc}] Consulta registrada com sucesso | "
            f"task_id={sub_id}"
        )

    return {"sub_task_id": sub_id, "type": "register_consulta", "doc": doc}


# ---------------------------------------------------------------------------
# Sub-task: update_financeiro (background)
# ---------------------------------------------------------------------------
async def subtask_update_financeiro(doc: str, parent_task_id: str):
    """
    Sub-task background: atualiza dados financeiros.
    Cria seu proprio arquivo de log.
    """
    sub_id = f"{uuid.uuid4()}_background_task"
    log = LogService(f"worker_background_{sub_id}").get_logger()

    async with httpx.AsyncClient() as client:
        enrich_ok = await check_dependency(client, "enrichment")

    with bind_uuid(sub_id):
        log.info(
            f"[DISPATCH:{sub_id}] Task recebida da fila "
            f"'{QUEUES['background']}' | broker=rabbitmq | "
            f"parent_task={parent_task_id}"
        )
        log.info(f"[DOC:{doc}] Atualizando dados financeiros")
        await asyncio.sleep(random.uniform(0.2, 0.6))

        if enrich_ok:
            socios = random.randint(1, 8)
            log.info(
                f"[DOC:{doc}] Enriquecimento completo - "
                f"socios={socios}, "
                f"endereco={'OK' if random.random() > 0.1 else 'NAO_ENCONTRADO'}, "
                f"telefones={random.randint(0, 5)}"
            )
        else:
            log.warning(
                f"[DOC:{doc}] Servico de enriquecimento indisponivel - "
                f"dados financeiros parciais"
            )

        log.info(
            f"[DOC:{doc}] Dados financeiros atualizados | "
            f"task_id={sub_id}"
        )

    return {"sub_task_id": sub_id, "type": "update_financeiro", "doc": doc}


# ---------------------------------------------------------------------------
# Sub-task: enviar_resposta_teams (background — disparada pelo callback)
# ---------------------------------------------------------------------------
async def subtask_enviar_resposta_teams(
    docs_processed: int,
    docs_failed: int,
    parent_task_id: str,
):
    """
    Sub-task background: envia resumo da consulta no Teams.
    Cria seu proprio arquivo de log.
    """
    sub_id = f"{uuid.uuid4()}_background_task"
    log = LogService(f"worker_background_{sub_id}").get_logger()

    with bind_uuid(sub_id):
        log.info(
            f"[DISPATCH:{sub_id}] Task recebida da fila "
            f"'{QUEUES['background']}' | broker=rabbitmq | "
            f"parent_task={parent_task_id}"
        )
        log.info(
            f"Preparando resposta Teams | "
            f"docs_ok={docs_processed} docs_failed={docs_failed}"
        )
        await asyncio.sleep(random.uniform(0.1, 0.3))

        if random.random() < 0.05:
            log.error(
                f"Falha ao enviar resposta Teams | "
                f"HTTPError 429 Too Many Requests | task_id={sub_id}"
            )
        else:
            log.info(
                f"Resposta Teams enviada com sucesso | "
                f"task_id={sub_id}"
            )

    return {"sub_task_id": sub_id, "type": "enviar_resposta_teams"}


# ---------------------------------------------------------------------------
# Core task: processa lote de documentos (chord task)
# ---------------------------------------------------------------------------
async def core_task_consulta_lote(
    client: httpx.AsyncClient,
    docs: list[tuple[str, str]],
) -> dict[str, Any]:
    """
    Simula um core task Celery que processa um lote de documentos.
    Cria UM arquivo de log para toda a task (como na producao).
    Cada documento e processado sequencialmente dentro do lote,
    mas sub-tasks sao disparadas em paralelo.
    """
    task_id = f"{uuid.uuid4()}_core_task"
    log = LogService(f"worker_consulta_{task_id}").get_logger()

    sub_tasks = []
    results = []

    with bind_uuid(task_id):
        log.info(
            f"[DISPATCH:{task_id}] Task recebida da fila "
            f"'{QUEUES['core']}' | broker=rabbitmq | "
            f"docs_no_lote={len(docs)}"
        )

        for doc, doc_type in docs:
            is_cnpj = doc_type == "cnpj"
            preferred_fonte = random.choice(FONTE_IDS)

            log.info(
                f"[DOC:{doc}] Iniciando consulta de "
                f"{'pessoa juridica' if is_cnpj else 'pessoa fisica'}"
            )

            # Validacao
            await asyncio.sleep(random.uniform(0.1, 0.3))
            log.info(f"[DOC:{doc}] Documento validado")

            # Consulta na fonte com circuit breaker
            fonte_id, usou_fallback = await resolve_fonte(client, preferred_fonte)

            if fonte_id == "none":
                log.error(
                    f"[DOC:{doc}] TODAS as fontes indisponiveis - "
                    f"circuit breaker ABERTO para receita, bureau_1, bureau_2"
                )
                log.error(
                    f"[DOC:{doc}] Documento {doc} FALHOU - movendo para retry | "
                    f"retry_count=3/3"
                )
                results.append({
                    "doc": doc, "status": "FAILED",
                    "error": "all_sources_unavailable",
                })
                continue

            fonte_nome = FONTES.get(fonte_id, fonte_id)

            if usou_fallback:
                log.warning(
                    f"[DOC:{doc}] Fonte primaria "
                    f"'{FONTES.get(preferred_fonte, preferred_fonte)}' "
                    f"indisponivel - fallback para '{fonte_nome}'"
                )
            else:
                log.info(f"[DOC:{doc}] Consultando fonte: {fonte_nome}")

            await asyncio.sleep(random.uniform(0.3, 1.0))

            # Resultado
            empresa = random.choice(NOMES_EMPRESA)
            situacao = random.choice(SITUACOES_CADASTRAIS)
            latencia = random.randint(80, 2500)

            log.info(
                f"[DOC:{doc}] Resultado obtido - "
                f"{'Empresa: ' + empresa + ' | ' if is_cnpj else ''}"
                f"Situacao: {situacao} | Fonte: {fonte_nome} | "
                f"latencia={latencia}ms"
            )

            if situacao == "BAIXADA":
                log.warning(
                    f"[DOC:{doc}] Documento com situacao BAIXADA - "
                    f"{'empresa inativa' if is_cnpj else 'cadastro cancelado'}"
                )
            elif situacao == "INAPTA":
                log.warning(
                    f"[DOC:{doc}] Documento INAPTO junto a Receita Federal"
                )

            # Erro aleatorio (instabilidade)
            if random.random() < 0.08:
                log.error(
                    f"[DOC:{doc}] Timeout na consulta complementar - "
                    f"enriquecimento parcial"
                )

            # Cache
            cache_ok = await check_dependency(client, "cache")
            if cache_ok:
                log.info(f"[DOC:{doc}] Resultado salvo no cache | ttl=3600s")
            else:
                log.warning(f"[DOC:{doc}] Cache indisponivel - resultado nao cacheado")

            # Dispara sub-tasks em background (cada uma cria seu proprio log file)
            log.info(
                f"[DOC:{doc}] Disparando sub-tasks background | "
                f"register_consulta + update_financeiro"
            )
            sub_tasks.append(subtask_register_consulta(doc, task_id))
            sub_tasks.append(subtask_update_financeiro(doc, task_id))

            log.info(
                f"[DOC:{doc}] Consulta finalizada | "
                f"fonte={fonte_nome} | situacao={situacao}"
            )

            results.append({
                "doc": doc, "doc_type": doc_type, "status": "OK",
                "fonte": fonte_id, "situacao": situacao, "fallback": usou_fallback,
            })

        # Resumo do lote
        ok = sum(1 for r in results if r["status"] == "OK")
        failed = sum(1 for r in results if r["status"] == "FAILED")
        log.info(
            f"Lote finalizado | total={len(docs)} ok={ok} failed={failed} | "
            f"task_id={task_id}"
        )

    # Sub-tasks rodam em paralelo (como apply_async na producao)
    if sub_tasks:
        await asyncio.gather(*sub_tasks)

    return {
        "task_id": task_id,
        "docs_processed": len(results),
        "ok": ok,
        "failed": failed,
        "results": results,
    }


# ---------------------------------------------------------------------------
# Chord: simula o fluxo completo como o JobDistributorService
# ---------------------------------------------------------------------------
async def simulate_chord(n_docs: int = 5):
    """
    Simula o chord Celery completo:
      1. API recebe request
      2. Divide docs em lotes (BATCH_SIZE)
      3. Cada lote e uma core task separada (com seu log file)
      4. Callback agrega resultados e dispara teams notification
    """
    # Gera documentos
    docs = []
    for _ in range(n_docs):
        if random.random() < 0.7:
            docs.append((random.choice(CNPJS), "cnpj"))
        else:
            docs.append((random.choice(CPFS), "cpf"))

    # API log: request recebido
    request_id = str(uuid.uuid4())
    batch_size = random.choice([3, 4, 5])  # simula BATCH_SIZE variavel

    with bind_uuid(request_id):
        logger_api.info(
            f"POST /api/v1/consultar - request_id={request_id} | "
            f"total_docs={n_docs} | broker=rabbitmq"
        )

        # Divide em lotes
        lotes = [docs[i:i + batch_size] for i in range(0, len(docs), batch_size)]
        logger_api.info(
            f"JobDistributor: {len(lotes)} lotes de ate {batch_size} docs | "
            f"chord dispatched | fila='{QUEUES['core']}'"
        )

    # Executa lotes em paralelo (chord)
    async with httpx.AsyncClient() as client:
        chord_tasks = [core_task_consulta_lote(client, lote) for lote in lotes]
        lote_results = await asyncio.gather(*chord_tasks)

    # Callback: agrega e dispara teams
    total_ok = sum(r["ok"] for r in lote_results)
    total_failed = sum(r["failed"] for r in lote_results)

    callback_id = f"{uuid.uuid4()}_core_task"
    callback_log = LogService(f"worker_consulta_{callback_id}").get_logger()

    with bind_uuid(callback_id):
        callback_log.info(
            f"[DISPATCH:{callback_id}] Callback do chord recebido | "
            f"lotes={len(lote_results)} | fila='{QUEUES['core']}'"
        )
        callback_log.info(
            f"Resultado agregado | total={n_docs} ok={total_ok} "
            f"failed={total_failed}"
        )

        if total_failed > 0:
            callback_log.warning(
                f"{total_failed} documento(s) falharam na consulta"
            )

        callback_log.info(
            f"Disparando sub-task enviar_resposta_teams | "
            f"task_id={callback_id}"
        )

    # Sub-task teams (cria seu proprio log file)
    await subtask_enviar_resposta_teams(total_ok, total_failed, callback_id)

    with bind_uuid(request_id):
        logger_api.info(
            f"Chord completo | request_id={request_id} | "
            f"ok={total_ok} failed={total_failed}"
        )

    return {
        "request_id": request_id,
        "callback_id": callback_id,
        "lotes": len(lote_results),
        "total_ok": total_ok,
        "total_failed": total_failed,
        "details": lote_results,
    }


# ---------------------------------------------------------------------------
# Loop continuo
# ---------------------------------------------------------------------------
async def generate_continuous_logs(interval: float):
    while True:
        try:
            n_docs = random.randint(3, 8)
            await simulate_chord(n_docs)
            await asyncio.sleep(interval)
        except asyncio.CancelledError:
            logger_api.info("Geracao continua de logs interrompida")
            break
        except Exception as e:
            logger_api.error(f"Erro na geracao continua: {e}")
            await asyncio.sleep(1)


# ---------------------------------------------------------------------------
# Lifespan
# ---------------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    logger_api.info("POC LogStream Mock API iniciada")
    logger_api.info(
        "Simulando chord Celery | "
        "broker=rabbitmq | filas: core, quick, background | "
        "cada task cria seu proprio log file"
    )

    global continuous_task
    continuous_task = asyncio.create_task(generate_continuous_logs(8.0))
    logger_api.info(
        "Geracao continua ATIVADA (intervalo=8s, chord com 3-8 docs)"
    )

    yield

    if continuous_task and not continuous_task.done():
        continuous_task.cancel()
        try:
            await continuous_task
        except asyncio.CancelledError:
            pass
    logger_api.info("POC LogStream Mock API encerrada")


# ---------------------------------------------------------------------------
# App
# ---------------------------------------------------------------------------
app = FastAPI(
    title="POC LogStream - Mock API",
    description=(
        "Simula workers Celery com chord: cada task cria seu proprio log file, "
        "sub-tasks (register, financeiro, teams) criam logs separados."
    ),
    version="1.0.0",
    lifespan=lifespan,
)


# ---------------------------------------------------------------------------
# Schemas
# ---------------------------------------------------------------------------
class ChordRequest(BaseModel):
    n_docs: int = Field(5, ge=1, le=50, description="Quantidade de documentos")


class BatchRequest(BaseModel):
    count: int = Field(3, ge=1, le=10, description="Quantidade de chords a executar")
    docs_per_chord: int = Field(5, ge=1, le=20, description="Docs por chord")
    interval: float = Field(3.0, ge=0.5, le=10.0, description="Intervalo entre chords (s)")


class ContinuousRequest(BaseModel):
    interval: float = Field(8.0, ge=2.0, le=60.0, description="Intervalo entre chords (s)")


class ChaosRequest(BaseModel):
    enabled: bool = Field(True)


class SimulationResult(BaseModel):
    status: str
    logs_generated: int
    log_files_created: int = 0
    details: list[dict] = []


# ---------------------------------------------------------------------------
# Endpoints
# ---------------------------------------------------------------------------
@app.get("/health")
async def health():
    tz = pytz.timezone("America/Sao_Paulo")
    return {
        "status": "healthy",
        "service": "poc-logstream-mock-api",
        "timestamp": datetime.now(tz).isoformat(),
        "continuous_active": continuous_task is not None and not continuous_task.done(),
    }


@app.post("/simulate/chord", response_model=SimulationResult)
async def simulate_chord_endpoint(req: ChordRequest):
    """
    Simula chord Celery completo:
    API -> divide em lotes -> core tasks paralelas -> callback -> sub-tasks background.
    Cada task e sub-task cria seu proprio arquivo de log.
    """
    result = await simulate_chord(req.n_docs)

    # Conta quantos log files foram criados:
    # 1 api + N core tasks (lotes) + 1 callback + 2*docs sub-tasks + 1 teams
    n_lotes = result["lotes"]
    n_docs = result["total_ok"]
    files = n_lotes + 1 + (2 * n_docs) + 1  # core tasks + callback + sub-tasks + teams

    return SimulationResult(
        status="ok",
        logs_generated=req.n_docs * 10,
        log_files_created=files,
        details=[result],
    )


@app.post("/simulate/batch", response_model=SimulationResult)
async def simulate_batch(req: BatchRequest):
    """Executa multiplos chords em sequencia."""
    all_results = []
    total_files = 0

    for i in range(req.count):
        result = await simulate_chord(req.docs_per_chord)
        n_lotes = result["lotes"]
        n_docs = result["total_ok"]
        total_files += n_lotes + 1 + (2 * n_docs) + 1
        all_results.append(result)
        if i < req.count - 1:
            await asyncio.sleep(req.interval)

    return SimulationResult(
        status="ok",
        logs_generated=req.count * req.docs_per_chord * 10,
        log_files_created=total_files,
        details=all_results,
    )


@app.post("/simulate/error", response_model=SimulationResult)
async def simulate_error():
    """Simula cenario de falha grave com todas as fontes fora."""
    task_id = f"{uuid.uuid4()}_core_task"
    cnpj = random.choice(CNPJS)
    log = LogService(f"worker_consulta_{task_id}").get_logger()

    with bind_uuid(task_id):
        log.info(
            f"[DISPATCH:{task_id}] Task recebida da fila "
            f"'{QUEUES['core']}' | broker=rabbitmq"
        )
        log.info(f"[DOC:{cnpj}] Iniciando consulta de pessoa juridica")

        await asyncio.sleep(0.3)
        log.error(
            f"[DOC:{cnpj}] ConnectionError: Timeout ao conectar com "
            f"Receita Federal (30s) | circuit_breaker=OPEN"
        )
        log.warning(f"[DOC:{cnpj}] Tentando fallback: Bureau de Credito 1")

        await asyncio.sleep(0.2)
        log.error(
            f"[DOC:{cnpj}] Bureau de Credito 1 retornou HTTP 503 | "
            f"circuit_breaker=HALF_OPEN"
        )
        log.warning(f"[DOC:{cnpj}] Tentando fallback: Bureau de Credito 2")

        await asyncio.sleep(0.2)
        log.error(
            f"[DOC:{cnpj}] Bureau de Credito 2 - Connection refused | "
            f"circuit_breaker=OPEN"
        )
        log.critical(
            f"[DOC:{cnpj}] TODAS as fontes indisponiveis | "
            f"task {task_id} FALHOU apos 3 retries"
        )
        log.critical(
            f"[DOC:{cnpj}] Movendo para dead letter queue | "
            f"dlq=consulta.core.dlq | retry_count=3/3"
        )

    return SimulationResult(
        status="ok",
        logs_generated=8,
        log_files_created=1,
        details=[{"task_id": task_id, "cnpj": cnpj, "type": "error_simulation"}],
    )


@app.post("/simulate/start-continuous", response_model=SimulationResult)
async def start_continuous(req: ContinuousRequest):
    global continuous_task

    if continuous_task and not continuous_task.done():
        continuous_task.cancel()
        try:
            await continuous_task
        except asyncio.CancelledError:
            pass

    continuous_task = asyncio.create_task(generate_continuous_logs(req.interval))
    logger_api.info(
        f"Geracao continua REINICIADA | intervalo={req.interval}s"
    )
    return SimulationResult(
        status="ok", logs_generated=0,
        details=[{"message": f"Continuous started (interval={req.interval}s)"}],
    )


@app.post("/simulate/stop-continuous", response_model=SimulationResult)
async def stop_continuous():
    global continuous_task

    if continuous_task and not continuous_task.done():
        continuous_task.cancel()
        try:
            await continuous_task
        except asyncio.CancelledError:
            pass
        continuous_task = None
        return SimulationResult(
            status="ok", logs_generated=0,
            details=[{"message": "Continuous generation stopped"}],
        )

    return SimulationResult(
        status="ok", logs_generated=0,
        details=[{"message": "No continuous generation was running"}],
    )


@app.post("/simulate/chaos", response_model=SimulationResult)
async def toggle_chaos(req: ChaosRequest):
    """Ativa/desativa chaos mode no servico de dependencias."""
    async with httpx.AsyncClient() as client:
        try:
            r = await client.post(
                f"{DEPS_BASE_URL}/admin/chaos",
                json={"enabled": req.enabled},
                timeout=5.0,
            )
            data = r.json()
            logger_api.warning(
                f"Chaos mode {'ATIVADO' if req.enabled else 'DESATIVADO'} | "
                f"servicos externos serao derrubados aleatoriamente"
            )
            return SimulationResult(
                status="ok", logs_generated=1, details=[data],
            )
        except Exception as e:
            return SimulationResult(
                status="error", logs_generated=0,
                details=[{"error": str(e)}],
            )


@app.get("/simulate/dependencies-status")
async def dependencies_status():
    async with httpx.AsyncClient() as client:
        try:
            r = await client.get(f"{DEPS_BASE_URL}/admin/status", timeout=5.0)
            return r.json()
        except Exception as e:
            return {"error": str(e)}
