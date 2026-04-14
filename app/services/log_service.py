"""
log_service.py — Adaptado do servi\u00e7o original da API_DadosBasicos.

Gera logs no formato esperado pelo logstream agent:
  %(asctime)s [%(levelname)s] [uuid=%(uuid)s] [%(filename)s:%(lineno)d] %(funcName)s() - %(message)s

O agent parseia esse formato via regex e extrai: timestamp, level, uuid,
module, documento (CNPJ/CPF), message, task_id.
"""

import logging
import os
from datetime import datetime
from contextlib import contextmanager
from contextvars import ContextVar

import pytz
from pytz import timezone


# === Contexto do UUID (task-local / coroutine-local) ===
request_uuid: ContextVar[str] = ContextVar("request_uuid", default="-")


@contextmanager
def bind_uuid(u: str):
    token = request_uuid.set(u)
    try:
        yield
    finally:
        request_uuid.reset(token)


class TZFormatter(logging.Formatter):
    def __init__(self, fmt=None, datefmt=None, tz=None):
        super().__init__(fmt=fmt, datefmt=datefmt)
        self.tz = timezone(tz) if tz else None

    def formatTime(self, record, datefmt=None):
        dt = datetime.fromtimestamp(record.created)
        if self.tz:
            dt = dt.astimezone(self.tz)
        if datefmt:
            s = dt.strftime(datefmt)
        else:
            s = dt.isoformat()
        return s


# Diretorio de logs — configuravel via env (default: /app/logs dentro do container)
LOG_DIR = os.environ.get("LOG_DIR", "/app/logs")
LOG_PREFIX = os.environ.get("LOG_PREFIX", "dados_basicos")

os.makedirs(LOG_DIR, exist_ok=True)


class EndpointFilter(logging.Filter):
    """Filtra logs do endpoint /metrics."""

    def filter(self, record):
        return "GET /metrics" not in record.getMessage()


class ContextUUIDFilter(logging.Filter):
    def filter(self, record):
        record.uuid = request_uuid.get()
        return True


class LogService:
    def __init__(self, log_type: str, log_level=logging.INFO):
        self.log_type = log_type
        self.log_level = log_level
        self.logger = self._create_logger()

    def _create_logger(self):
        brazil_tz = pytz.timezone("America/Sao_Paulo")
        log_filename = datetime.now(brazil_tz).strftime(
            f"{LOG_PREFIX}_{self.log_type.lower()}_%Y-%m-%d_%H-%M-%S.log"
        )
        log_file = os.path.join(LOG_DIR, log_filename)

        logger = logging.getLogger(log_file)

        if not logger.hasHandlers():
            logger.handlers.clear()

        logger.setLevel(self.log_level)
        logger.propagate = False
        logger.addFilter(EndpointFilter())
        logger.addFilter(ContextUUIDFilter())

        log_formatter = TZFormatter(
            "%(asctime)s [%(levelname)s] [uuid=%(uuid)s] [%(filename)s:%(lineno)d] %(funcName)s() - %(message)s",
            "%Y-%m-%d %H:%M:%S",
            tz="America/Sao_Paulo",
        )

        file_handler = logging.FileHandler(log_file, encoding="utf-8")
        file_handler.setFormatter(log_formatter)

        console_handler = logging.StreamHandler()
        console_handler.setFormatter(log_formatter)

        logger.addHandler(file_handler)
        logger.addHandler(console_handler)

        return logger

    def get_logger(self):
        return self.logger
