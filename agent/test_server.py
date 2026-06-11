"""Tests for agent/server.py HTTP endpoints."""
from __future__ import annotations

import asyncio
import json
from typing import AsyncIterator
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from aiohttp import web

import agent.server as server_module
from agent.confirmations import ConfirmationManager, QuestionManager


# ── test app factory ──────────────────────────────────────────────────────────

def _make_backend(auto_mode: bool = False, api_url: str = "") -> MagicMock:
    backend = MagicMock()
    backend.model = "test-model"
    backend.api_url = api_url
    backend.auto_mode = auto_mode
    backend.confirmation_manager = ConfirmationManager()
    backend.question_manager = QuestionManager()
    # ChatSession-like attributes
    backend.chat.token_count = 0
    backend.chat.token_limit = 8192
    backend.chat.reset = MagicMock()
    backend.chat.compact = AsyncMock(return_value={
        "tokens_before": 100,
        "tokens_after": 50,
        "summary": "compacted",
    })
    backend.set_auto_mode = MagicMock(side_effect=lambda v: setattr(backend, "auto_mode", v))
    return backend


def _make_app(backend: MagicMock, shutdown_event: asyncio.Event | None = None) -> web.Application:
    app = web.Application()
    app["backend"] = backend
    app.router.add_get("/health", server_module.health)
    app.router.add_post("/reset", server_module.reset)
    app.router.add_post("/mode", server_module.set_mode)
    app.router.add_get("/status", server_module.status)
    app.router.add_post("/shutdown", server_module.shutdown_handler)
    app.router.add_post("/compact", server_module.compact_handler)
    app.router.add_post("/question", server_module.question_handler)
    app.router.add_get("/tools", server_module.tools_list)
    app.router.add_post("/confirm", server_module.confirm)
    app.router.add_post("/chat", server_module.chat)
    return app


# ── basic endpoints ───────────────────────────────────────────────────────────

async def test_health(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.get("/health")
    assert resp.status == 200
    text = await resp.text()
    assert text == "ok"


async def test_reset(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/reset")
    assert resp.status == 200
    backend.chat.reset.assert_called_once()


async def test_set_mode_auto(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/mode", json={"auto": True})
    assert resp.status == 200
    assert backend.auto_mode is True


async def test_set_mode_safe(aiohttp_client):
    backend = _make_backend(auto_mode=True)
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/mode", json={"auto": False})
    assert resp.status == 200
    assert backend.auto_mode is False


async def test_status_ollama(aiohttp_client):
    backend = _make_backend(api_url="")
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.get("/status")
    assert resp.status == 200
    data = await resp.json()
    assert data["provider"] == "ollama"
    assert data["model"] == "test-model"
    assert "token_count" in data
    assert "cwd" in data


async def test_status_openrouter(aiohttp_client):
    backend = _make_backend(api_url="https://openrouter.ai/api/v1")
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.get("/status")
    data = await resp.json()
    assert data["provider"] == "openrouter"


async def test_tools_list(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.get("/tools")
    assert resp.status == 200
    data = await resp.json()
    assert "tools" in data
    assert isinstance(data["tools"], list)
    assert len(data["tools"]) > 0


# ── compact ───────────────────────────────────────────────────────────────────

async def test_compact(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/compact")
    assert resp.status == 200
    data = await resp.json()
    assert data["tokens_before"] == 100
    assert data["tokens_after"] == 50


# ── shutdown ──────────────────────────────────────────────────────────────────

async def test_shutdown_sets_event(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    evt = asyncio.Event()
    # Patch the module-level _shutdown_event
    original = server_module._shutdown_event
    server_module._shutdown_event = evt
    try:
        client = await aiohttp_client(app)
        resp = await client.post("/shutdown")
        assert resp.status == 200
        await asyncio.sleep(0.01)  # let call_soon fire
        assert evt.is_set()
    finally:
        server_module._shutdown_event = original


# ── confirm ───────────────────────────────────────────────────────────────────

async def test_confirm_ok(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)

    # Register a pending confirmation
    call_id, event = backend.confirmation_manager._make_entry(ok=None)

    resp = await client.post("/confirm", json={"id": call_id, "ok": True})
    assert resp.status == 200


async def test_confirm_not_found(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/confirm", json={"id": "nonexistent", "ok": True})
    assert resp.status == 404


# ── question ──────────────────────────────────────────────────────────────────

async def test_question_ok(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)

    call_id, event = backend.question_manager._make_entry(answers=None)

    resp = await client.post("/question", json={"id": call_id, "answers": [["yes"]]})
    assert resp.status == 200


async def test_question_not_found(aiohttp_client):
    backend = _make_backend()
    app = _make_app(backend)
    client = await aiohttp_client(app)
    resp = await client.post("/question", json={"id": "bad-id", "answers": []})
    assert resp.status == 404


# ── chat endpoint ─────────────────────────────────────────────────────────────

async def _fake_stream_all_types(text, auto_mode=False):
    yield ("warden_start", {})
    yield ("token", "hello ")
    yield ("token", "world")
    yield ("think", "thinking deeply")
    yield ("tool_start", {"name": "file_list", "args": '{"path": "."}'})
    yield ("tool", {"name": "file_list", "args": '{"path": "."}', "result": "ok"})
    yield ("confirm", {
        "id": "c1",
        "tool": "file_delete",
        "risk": "confirm",
        "title": "Delete file",
        "summary": "Deletes a file",
        "details": ["file.txt"],
        "args": '{"path": "file.txt"}',
        "preview": "",
    })
    yield ("question", {"id": "q1", "questions": [{"question": "Are you sure?", "header": "confirm"}]})


async def test_chat_all_event_types(aiohttp_client):
    backend = _make_backend()
    backend.chat.stream = _fake_stream_all_types
    app = _make_app(backend)
    client = await aiohttp_client(app)

    resp = await client.post("/chat", json={"text": "hello"})
    assert resp.status == 200

    body = await resp.read()
    lines = [l for l in body.decode().splitlines() if l.strip()]
    events = [json.loads(l) for l in lines]
    types = {e["type"] for e in events}

    assert "warden_start" in types
    assert "token" in types
    assert "think" in types
    assert "tool_start" in types
    assert "tool" in types
    assert "confirm" in types
    assert "question" in types
    assert "done" in types


async def test_chat_done_has_token_info(aiohttp_client):
    backend = _make_backend()

    async def _simple_stream(text, auto_mode=False):
        yield ("token", "hi")

    backend.chat.stream = _simple_stream
    app = _make_app(backend)
    client = await aiohttp_client(app)

    resp = await client.post("/chat", json={"text": "test"})
    body = await resp.read()
    lines = [l for l in body.decode().splitlines() if l.strip()]
    events = [json.loads(l) for l in lines]
    done = next((e for e in events if e["type"] == "done"), None)
    assert done is not None
    assert "token_count" in done
    assert "token_limit" in done


async def test_chat_error_yields_error_event(aiohttp_client):
    backend = _make_backend()

    async def _error_stream(text, auto_mode=False):
        raise RuntimeError("boom")
        yield  # make it a generator

    backend.chat.stream = _error_stream
    app = _make_app(backend)
    client = await aiohttp_client(app)

    resp = await client.post("/chat", json={"text": "test"})
    body = await resp.read()
    lines = [l for l in body.decode().splitlines() if l.strip()]
    events = [json.loads(l) for l in lines]
    error_events = [e for e in events if e["type"] == "error"]
    assert len(error_events) > 0
    assert "boom" in error_events[0]["text"]


# ── _client_disconnected ──────────────────────────────────────────────────────

def test_client_disconnected_true():
    transport = MagicMock()
    transport.is_closing.return_value = True
    request = MagicMock()
    request.transport = transport
    assert server_module._client_disconnected(request) is True


def test_client_disconnected_false():
    transport = MagicMock()
    transport.is_closing.return_value = False
    request = MagicMock()
    request.transport = transport
    assert server_module._client_disconnected(request) is False


def test_client_disconnected_no_transport():
    request = MagicMock()
    request.transport = None
    assert server_module._client_disconnected(request) is False
