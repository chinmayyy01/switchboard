from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import Any


@dataclass(frozen=True)
class SessionConfig:
    call_id: str
    tenant_id: str
    tenant_name: str
    services: list[str] = field(default_factory=list)
    windows: list[str] = field(default_factory=list)
    disclosure: str = ""
    manager_number: str = ""
    persona: dict[str, Any] = field(default_factory=dict)
    token: str = ""

    @property
    def agent_name(self) -> str:
        return str(self.persona.get("agent_name") or "Alex")

    @property
    def tone(self) -> str:
        return str(self.persona.get("tone") or "warm, brief, and natural")


@dataclass(frozen=True)
class Settings:
    go_api_url: str
    agent_api_key: str
    recording_bucket: str
    recording_region: str


def settings() -> Settings:
    return Settings(
        go_api_url=os.getenv("GO_API_URL", "http://127.0.0.1:8090").rstrip("/"),
        agent_api_key=os.getenv("AGENT_API_KEY", ""),
        recording_bucket=os.getenv("RECORDING_S3_BUCKET", ""),
        recording_region=os.getenv("RECORDING_S3_REGION", "ap-south-1"),
    )


def session_config(payload: dict[str, Any], call_id: str) -> SessionConfig:
    tenant = payload.get("tenant") or {}
    return SessionConfig(
        call_id=payload.get("call_id") or call_id,
        tenant_id=str(tenant.get("id") or ""),
        tenant_name=str(tenant.get("name") or "the business"),
        services=list(tenant.get("services") or []),
        windows=list(tenant.get("windows") or ["morning", "afternoon", "evening"]),
        disclosure=str(tenant.get("disclosure") or ""),
        manager_number=str(tenant.get("manager_number") or ""),
        persona=dict(tenant.get("persona") or {}),
        token=str(payload.get("token") or ""),
    )
