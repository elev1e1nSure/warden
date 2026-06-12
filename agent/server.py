from __future__ import annotations

import asyncio
import json
import os

from aiohttp import web
from aiohttp.client_exceptions import ClientConnectionResetError

from agent.chat import ChatSession
from agent.llm_client import OllamaClient, OpenAIClient
from agent.memory.aggregator import MemoryAggregator
from agent.memory.store import MemoryStore
from agent.ollama_process import OllamaProcessManager
from agent.confirmations import ConfirmationManager, QuestionManager
from agent.logger import info, warn, error, success, request as log_request
from agent.tools import _get_screenshot_dir, _cleanup_old_screenshots

class Backend:
	def __init__(self) -> None:
		try:
			_cleanup_old_screenshots(_get_screenshot_dir(), max_age_seconds=0)
		except Exception:
			pass
		self.model: str = os.environ.get("WARDEN_MODEL", "")
		self.api_url: str = os.environ.get("WARDEN_API_URL", "")
		self.api_key: str = os.environ.get("OPENROUTER_API_KEY", "")
		self.llm: OllamaClient | OpenAIClient | None = None
		self.ollama: OllamaProcessManager | None = None
		self.chat: ChatSession | None = None
		self.auto_mode: bool = False
		self.confirmation_manager = ConfirmationManager()
		self.question_manager = QuestionManager()
		self.memory_store = MemoryStore()
		if self.memory_store.get_enabled():
			info("memory enabled")
		if self.model and self.api_url:
			self._init_openrouter(self.api_url, self.api_key, self.model)
		elif self.model:
			self._init_ollama(self.model)

	def _new_chat(self) -> None:
		if self.chat is not None and self.memory_store is not None:
			MemoryAggregator.finalize(self.memory_store, self.chat.session_id)
		self.chat = ChatSession(
			model=self.model,
			client=self.llm,
			confirmation_manager=self.confirmation_manager,
			question_manager=self.question_manager,
			memory_store=self.memory_store,
		)

	def _init_openrouter(self, api_url: str, api_key: str, model: str) -> None:
		self.llm = OpenAIClient(api_url, api_key=api_key or None)
		self.api_url = api_url
		self.api_key = api_key
		self.model = model
		self.ollama = None
		self._new_chat()

	def _init_ollama(self, model: str) -> None:
		self.llm = OllamaClient()
		self.model = model
		self.api_url = ""
		self.api_key = ""
		self.ollama = OllamaProcessManager(model=model)
		self._new_chat()

	async def setup(self) -> None:
		if self.ollama is None:
			return
		ok = await self.ollama.ensure_running()
		if not ok:
			raise RuntimeError("failed to connect to ollama")
		if not self.ollama.has_model():
			await self.ollama.pull_model()

	def set_auto_mode(self, enabled: bool) -> None:
		self.auto_mode = enabled


def _get_backend(request: web.Request) -> Backend:
	return request.app["backend"]


async def health(request: web.Request) -> web.Response:
	log_request("GET", "/health", 200)
	return web.Response(text="ok")


async def reset(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	backend.confirmation_manager.cancel_all()
	backend.question_manager.cancel_all()
	if backend.chat is not None:
		backend.chat.reset()
	try:
		_cleanup_old_screenshots(_get_screenshot_dir(), max_age_seconds=0)
	except Exception:
		pass
	log_request("POST", "/reset", 200)
	info("session reset")
	return web.Response(text="ok")


async def set_mode(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	backend.set_auto_mode(bool(data.get("auto", False)))
	mode = "AUTO" if backend.auto_mode else "SAFE"
	log_request("POST", "/mode", 200)
	info(f"mode changed to {mode}")
	return web.Response(text="ok")


async def status(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = {
		"model": backend.model,
		"provider": "openrouter" if backend.api_url else "ollama",
		"mode": "auto" if backend.auto_mode else "ask",
		"cwd": os.getcwd(),
		"token_count": backend.chat.token_count if backend.chat else 0,
		"token_limit": backend.chat.token_limit if backend.chat else 0,
	}
	log_request("GET", "/status", 200)
	return web.json_response(data)


async def shutdown_handler(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	backend.confirmation_manager.cancel_all()
	backend.question_manager.cancel_all()
	if backend.chat is not None and backend.memory_store is not None:
		MemoryAggregator.finalize(backend.memory_store, backend.chat.session_id)
	log_request("POST", "/shutdown", 200)
	info("graceful shutdown requested")
	shutdown_event = request.app.get("shutdown_event")
	if shutdown_event is not None:
		asyncio.get_event_loop().call_soon(shutdown_event.set)
	return web.Response(text="ok")


async def compact_handler(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	if backend.chat is None:
		return web.json_response({"error": "not connected"}, status=400)
	log_request("POST", "/compact")
	result = await backend.chat.compact()
	info(f"compacted: {result['tokens_before']} → {result['tokens_after']} tokens")
	return web.json_response(result)


async def memory_state_get(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	stats = backend.memory_store.get_stats()
	log_request("GET", "/memory/state", 200)
	return web.json_response(stats)


async def memory_state_post(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	enabled = bool(data.get("enabled", False))
	backend.memory_store.set_enabled(enabled)
	action = "enabled" if enabled else "disabled"
	log_request("POST", "/memory/state", 200)
	info(f"memory {action}")
	return web.json_response({"enabled": enabled})


async def memory_clear(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	count = backend.memory_store.clear_entries()
	log_request("POST", "/memory/clear", 200)
	info(f"memory cleared: {count} entries")
	return web.json_response({"cleared": count})


async def memory_snapshot(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	snap = backend.memory_store.get_latest_snapshot()
	log_request("GET", "/memory/snapshot", 200)
	return web.json_response(snap or {})


async def question_handler(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	call_id = data.get("id", "")
	answers = data.get("answers")
	resolved = backend.question_manager.resolve(call_id, answers)
	if resolved:
		log_request("POST", "/question", 200)
		info(f"questions answered: {call_id}")
		return web.Response(text="ok")
	log_request("POST", "/question", 404)
	return web.Response(status=404, text="not found")


async def models_list(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	if backend.llm is None:
		return web.json_response({"models": [], "current": "", "error": "not connected"})
	error = ""
	try:
		models = await backend.llm.list_models()
	except Exception as e:
		warn(f"list_models failed: {e}")
		error = str(e)
		models = []
	log_request("GET", "/models", 200)
	return web.json_response({"models": models, "current": backend.model, "error": error})


async def model_set(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	model = data.get("model", "").strip()
	if not model:
		return web.Response(status=400, text="model required")
	backend.model = model
	if backend.llm is not None:
		backend._new_chat()
	info(f"model changed to {model}")
	log_request("POST", "/model/set", 200)
	return web.Response(text="ok")


async def connect_handler(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	provider = data.get("provider", "").strip()
	api_key = data.get("api_key", "").strip()
	model = data.get("model", "").strip()

	if not model:
		return web.json_response({"ok": False, "error": "model name is required"})

	if provider == "openrouter":
		if not api_key:
			return web.json_response({"ok": False, "error": "api key is required"})
		api_url = "https://openrouter.ai/api/v1"
		try:
			test_client = OpenAIClient(api_url, api_key=api_key)
			await asyncio.wait_for(test_client.list_models(), timeout=10.0)
		except asyncio.TimeoutError:
			return web.json_response({"ok": False, "error": "connection timed out — check your internet"})
		except Exception as e:
			msg = str(e).lower()
			if any(x in msg for x in ("401", "unauthorized", "api key", "authentication", "invalid_api_key", "forbidden")):
				return web.json_response({"ok": False, "error": "invalid api key — check it at openrouter.ai/keys"})
			return web.json_response({"ok": False, "error": "could not reach openrouter — check your internet"})
		backend._init_openrouter(api_url, api_key, model)

	elif provider == "ollama":
		try:
			test_client = OllamaClient()
			await asyncio.wait_for(test_client.list_models(), timeout=5.0)
		except asyncio.TimeoutError:
			return web.json_response({"ok": False, "error": "ollama is not responding — is it running?"})
		except Exception:
			return web.json_response({"ok": False, "error": "cannot reach ollama — install it from ollama.com and run it"})
		backend._init_ollama(model)
		if backend.ollama is not None:
			try:
				await asyncio.wait_for(backend.setup(), timeout=120.0)
			except Exception as e:
				return web.json_response({"ok": False, "error": f"ollama setup failed: {str(e)[:100]}"})
	else:
		return web.json_response({"ok": False, "error": f"unknown provider: {provider}"})

	log_request("POST", "/connect", 200)
	info(f"connected: {provider} / {model}")
	return web.json_response({"ok": True})


async def tools_list(request: web.Request) -> web.Response:
	from agent.tools import REGISTRY
	log_request("GET", "/tools", 200)
	return web.json_response({"tools": list(REGISTRY.keys())})


async def skills_list(request: web.Request) -> web.Response:
	from agent.skills import discover_skills
	skills = discover_skills()
	log_request("GET", "/skills", 200)
	return web.json_response({
		"skills": [
			{
				"name": s.name,
				"description": s.description,
				"location": s.location,
			}
			for s in skills
		]
	})


async def skill_get(request: web.Request) -> web.Response:
	from agent.skills import find_skill, wrap_skill_content, _validate_name
	name = request.match_info.get("name", "")
	if not _validate_name(name):
		log_request("GET", f"/skill/{name}", 400)
		return web.json_response({"error": "invalid skill name"}, status=400)
	skill = find_skill(name)
	if skill is None:
		log_request("GET", f"/skill/{name}", 404)
		return web.json_response({"error": "skill not found"}, status=404)
	log_request("GET", f"/skill/{name}", 200)
	return web.json_response({
		"name": skill.name,
		"content": wrap_skill_content(skill),
	})


async def confirm(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	call_id = data.get("id", "")
	ok = bool(data.get("ok", False))
	resolved = backend.confirmation_manager.resolve(call_id, ok)
	if resolved:
		log_request("POST", "/confirm", 200)
		action = "confirmed" if ok else "cancelled"
		info(f"action {action}")
		return web.Response(text="ok")
	log_request("POST", "/confirm", 404)
	warn(f"confirm not found: {call_id}")
	return web.Response(status=404, text="not found")


def _client_disconnected(request: web.Request) -> bool:
	transport = request.transport
	return transport is not None and transport.is_closing()


async def chat(request: web.Request) -> web.StreamResponse:
	backend = _get_backend(request)
	data = await request.json()
	text = data.get("text", "")
	log_request("POST", "/chat")
	info(f"user: {text[:50]}..." if len(text) > 50 else f"user: {text}")

	response = web.StreamResponse(
		status=200,
		headers={"Content-Type": "application/x-ndjson"},
	)
	await response.prepare(request)

	if backend.chat is None:
		await response.write(json.dumps({"type": "token", "text": "not connected — run /connect to get started"}).encode() + b"\n")
		await response.write(json.dumps({"type": "done", "token_count": 0, "token_limit": 0}).encode() + b"\n")
		return response

	try:
		async for type_, payload in backend.chat.stream(text, auto_mode=backend.auto_mode):
			if _client_disconnected(request):
				backend.confirmation_manager.cancel_all()
				backend.question_manager.cancel_all()
				break
			if type_ == "warden_start":
				msg: dict = {"type": "warden_start"}
			elif type_ in ("token", "think"):
				msg = {"type": type_, "text": payload}
			elif type_ == "tool_start":
				msg = {"type": "tool_start", "name": payload["name"], "args": payload["args"]}
			elif type_ == "tool":
				msg = {"type": "tool", "name": payload["name"], "args": payload["args"], "result": payload["result"]}
				if payload.get("diff"):
					msg["diff"] = payload["diff"]
			elif type_ == "confirm":
				msg = {
					"type": "confirm",
					"id": payload["id"],
					"tool": payload["tool"],
					"risk": payload.get("risk", "confirm"),
					"title": payload.get("title", "Dangerous action"),
					"summary": payload.get("summary", ""),
					"details": payload.get("details", []),
					"args": payload["args"],
					"preview": payload.get("preview", ""),
					"default": payload.get("default", "cancel"),
				}
			elif type_ == "question":
				msg = {
					"type": "question",
					"id": payload["id"],
					"questions": payload["questions"],
				}
			else:
				continue
			try:
				await response.write((json.dumps(msg, ensure_ascii=False) + "\n").encode())
			except (ConnectionResetError, ClientConnectionResetError):
				break
		if not _client_disconnected(request):
			done_msg = {
				"type": "done",
				"token_count": backend.chat.token_count,
				"token_limit": backend.chat.token_limit,
			}
			await response.write((json.dumps(done_msg) + "\n").encode())
	except (ConnectionResetError, ClientConnectionResetError):
		pass
	except Exception as e:
		if not _client_disconnected(request):
			try:
				await response.write((json.dumps({"type": "error", "text": str(e)}, ensure_ascii=False) + "\n").encode())
			except (ConnectionResetError, ClientConnectionResetError):
				pass

	return response


async def main() -> Backend:
	info("starting backend...")
	backend = Backend()
	await backend.setup()
	if backend.ollama is not None:
		success("ollama ready")
	else:
		success("remote API ready")

	shutdown_event = asyncio.Event()
	app = web.Application()
	app["backend"] = backend
	app["shutdown_event"] = shutdown_event
	app.router.add_get("/health", health)
	app.router.add_post("/reset", reset)
	app.router.add_post("/chat", chat)
	app.router.add_post("/confirm", confirm)
	app.router.add_post("/mode", set_mode)
	app.router.add_get("/status", status)
	app.router.add_get("/tools", tools_list)
	app.router.add_get("/skills", skills_list)
	app.router.add_get("/skill/{name}", skill_get)
	app.router.add_get("/models", models_list)
	app.router.add_post("/model/set", model_set)
	app.router.add_post("/connect", connect_handler)
	app.router.add_post("/question", question_handler)
	app.router.add_post("/compact", compact_handler)
	app.router.add_get("/memory/state", memory_state_get)
	app.router.add_post("/memory/state", memory_state_post)
	app.router.add_post("/memory/clear", memory_clear)
	app.router.add_get("/memory/snapshot", memory_snapshot)
	app.router.add_post("/shutdown", shutdown_handler)
	runner = web.AppRunner(app)
	await runner.setup()
	site = web.TCPSite(runner, "localhost", 8765)
	await site.start()
	success("backend on http://localhost:8765")
	await shutdown_event.wait()
	await runner.cleanup()
	return backend


if __name__ == "__main__":
	backend = None
	try:
		backend = asyncio.run(main())
	except KeyboardInterrupt:
		if backend is not None and backend.ollama is not None:
			backend.ollama.shutdown()
