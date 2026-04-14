"""
Mock Dependencies — Simula servicos externos dos quais a API depende.

Cada servico tem um endpoint /health que o healthmon do agent monitora.
Endpoints de admin permitem simular quedas, latencia e caos.

Servicos simulados:
  - receita:   Receita Federal (consulta CNPJ)
  - bureau_1:  Bureau de credito primario
  - bureau_2:  Bureau de credito secundario
  - cache:     Cache distribuido (Redis externo)
  - enrichment: API de enriquecimento de dados

Admin:
  POST /admin/down/{service_id}      — derruba servico
  POST /admin/up/{service_id}        — restaura servico
  POST /admin/slow/{service_id}      — adiciona latencia (simula degradacao)
  POST /admin/chaos                  — modo caos: derruba servicos aleatorios
  POST /admin/recover-all            — restaura todos
  GET  /admin/status                 — status de todos os servicos
"""

import asyncio
import random
import time
from contextlib import asynccontextmanager
from datetime import datetime

import pytz
from fastapi import FastAPI, Response
from pydantic import BaseModel, Field


# ---------------------------------------------------------------------------
# Estado dos servicos
# ---------------------------------------------------------------------------
class ServiceState:
    def __init__(self, service_id: str, name: str, healthy: bool = True):
        self.service_id = service_id
        self.name = name
        self.healthy = healthy
        self.latency_ms: int = 0          # latencia simulada
        self.error_rate: float = 0.0      # % de requests que falham
        self.last_check: str = ""
        self.down_since: str | None = None
        self.total_checks: int = 0
        self.total_failures: int = 0


SERVICES: dict[str, ServiceState] = {
    "receita": ServiceState("receita", "Receita Federal"),
    "bureau_1": ServiceState("bureau_1", "Bureau de Credito 1"),
    "bureau_2": ServiceState("bureau_2", "Bureau de Credito 2"),
    "cache": ServiceState("cache", "Cache Distribuido"),
    "enrichment": ServiceState("enrichment", "API Enriquecimento"),
}


# ---------------------------------------------------------------------------
# Chaos engine — derruba servicos periodicamente
# ---------------------------------------------------------------------------
chaos_active: bool = False
chaos_task: asyncio.Task | None = None


async def chaos_loop():
    """Periodicamente derruba e restaura servicos aleatorios."""
    while True:
        try:
            await asyncio.sleep(random.uniform(15, 45))

            # Escolhe 1-2 servicos para derrubar
            targets = random.sample(list(SERVICES.keys()), k=random.randint(1, 2))
            tz = pytz.timezone("America/Sao_Paulo")
            now = datetime.now(tz).isoformat()

            for sid in targets:
                svc = SERVICES[sid]
                if svc.healthy:
                    svc.healthy = False
                    svc.down_since = now

            # Espera um tempo e restaura
            await asyncio.sleep(random.uniform(10, 30))

            for sid in targets:
                svc = SERVICES[sid]
                if not svc.healthy:
                    svc.healthy = True
                    svc.down_since = None
                    svc.latency_ms = 0

        except asyncio.CancelledError:
            break


# ---------------------------------------------------------------------------
# Lifespan
# ---------------------------------------------------------------------------
@asynccontextmanager
async def lifespan(app: FastAPI):
    yield
    global chaos_task
    if chaos_task and not chaos_task.done():
        chaos_task.cancel()
        try:
            await chaos_task
        except asyncio.CancelledError:
            pass


# ---------------------------------------------------------------------------
# App
# ---------------------------------------------------------------------------
app = FastAPI(
    title="Mock Dependencies",
    description="Simula servicos externos com controle de disponibilidade",
    version="1.0.0",
    lifespan=lifespan,
)


# ---------------------------------------------------------------------------
# Health endpoints — o healthmon do agent chama estes
# ---------------------------------------------------------------------------
@app.get("/{service_id}/health")
async def service_health(service_id: str, response: Response):
    svc = SERVICES.get(service_id)
    if not svc:
        response.status_code = 404
        return {"error": f"service '{service_id}' not found"}

    svc.total_checks += 1
    tz = pytz.timezone("America/Sao_Paulo")
    svc.last_check = datetime.now(tz).isoformat()

    # Simula latencia
    if svc.latency_ms > 0:
        await asyncio.sleep(svc.latency_ms / 1000.0)

    # Simula erro rate
    if svc.error_rate > 0 and random.random() < svc.error_rate:
        svc.total_failures += 1
        response.status_code = 503
        return {
            "service": svc.name,
            "status": "degraded",
            "error": "intermittent failure (simulated error rate)",
        }

    if not svc.healthy:
        svc.total_failures += 1
        response.status_code = 503
        return {
            "service": svc.name,
            "status": "unhealthy",
            "down_since": svc.down_since,
        }

    return {
        "service": svc.name,
        "status": "healthy",
        "latency_ms": svc.latency_ms,
    }


# Endpoint raiz para health check geral do container
@app.get("/health")
async def container_health():
    tz = pytz.timezone("America/Sao_Paulo")
    healthy_count = sum(1 for s in SERVICES.values() if s.healthy)
    return {
        "status": "healthy",
        "timestamp": datetime.now(tz).isoformat(),
        "services_up": healthy_count,
        "services_total": len(SERVICES),
        "chaos_active": chaos_active,
    }


# ---------------------------------------------------------------------------
# Admin endpoints — controle manual
# ---------------------------------------------------------------------------
class SlowRequest(BaseModel):
    latency_ms: int = Field(2000, ge=0, le=30000, description="Latencia em ms")


class ErrorRateRequest(BaseModel):
    rate: float = Field(0.5, ge=0.0, le=1.0, description="Taxa de erro (0.0 a 1.0)")


class ChaosRequest(BaseModel):
    enabled: bool = Field(True)


@app.post("/admin/down/{service_id}")
async def take_down(service_id: str, response: Response):
    svc = SERVICES.get(service_id)
    if not svc:
        response.status_code = 404
        return {"error": f"service '{service_id}' not found"}

    tz = pytz.timezone("America/Sao_Paulo")
    svc.healthy = False
    svc.down_since = datetime.now(tz).isoformat()
    return {"service": svc.name, "status": "DOWN", "down_since": svc.down_since}


@app.post("/admin/up/{service_id}")
async def bring_up(service_id: str, response: Response):
    svc = SERVICES.get(service_id)
    if not svc:
        response.status_code = 404
        return {"error": f"service '{service_id}' not found"}

    svc.healthy = True
    svc.down_since = None
    svc.latency_ms = 0
    svc.error_rate = 0.0
    return {"service": svc.name, "status": "UP"}


@app.post("/admin/slow/{service_id}")
async def make_slow(service_id: str, req: SlowRequest, response: Response):
    svc = SERVICES.get(service_id)
    if not svc:
        response.status_code = 404
        return {"error": f"service '{service_id}' not found"}

    svc.latency_ms = req.latency_ms
    return {
        "service": svc.name,
        "status": "DEGRADED" if req.latency_ms > 0 else "HEALTHY",
        "latency_ms": svc.latency_ms,
    }


@app.post("/admin/error-rate/{service_id}")
async def set_error_rate(service_id: str, req: ErrorRateRequest, response: Response):
    svc = SERVICES.get(service_id)
    if not svc:
        response.status_code = 404
        return {"error": f"service '{service_id}' not found"}

    svc.error_rate = req.rate
    return {
        "service": svc.name,
        "error_rate": svc.error_rate,
    }


@app.post("/admin/chaos")
async def toggle_chaos(req: ChaosRequest):
    global chaos_active, chaos_task

    if req.enabled and not chaos_active:
        chaos_active = True
        chaos_task = asyncio.create_task(chaos_loop())
        return {"chaos": "ENABLED", "message": "Servicos serao derrubados/restaurados aleatoriamente"}

    if not req.enabled and chaos_active:
        chaos_active = False
        if chaos_task and not chaos_task.done():
            chaos_task.cancel()
            try:
                await chaos_task
            except asyncio.CancelledError:
                pass
        chaos_task = None
        return {"chaos": "DISABLED"}

    return {"chaos": "ENABLED" if chaos_active else "DISABLED", "message": "No change"}


@app.post("/admin/recover-all")
async def recover_all():
    for svc in SERVICES.values():
        svc.healthy = True
        svc.down_since = None
        svc.latency_ms = 0
        svc.error_rate = 0.0
    return {"status": "all services recovered", "count": len(SERVICES)}


@app.get("/admin/status")
async def admin_status():
    tz = pytz.timezone("America/Sao_Paulo")
    result = []
    for svc in SERVICES.values():
        result.append({
            "service_id": svc.service_id,
            "name": svc.name,
            "healthy": svc.healthy,
            "latency_ms": svc.latency_ms,
            "error_rate": svc.error_rate,
            "down_since": svc.down_since,
            "last_check": svc.last_check,
            "total_checks": svc.total_checks,
            "total_failures": svc.total_failures,
        })
    return {
        "timestamp": datetime.now(tz).isoformat(),
        "chaos_active": chaos_active,
        "services": result,
    }
