import asyncio
import json
import os

from aiohttp import web
from aiohttp.client_exceptions import ClientConnectionResetError

from agent.chat import ChatSession
from agent.llm_client import OllamaClient, OpenAIClient
from agent.ollama_process import OllamaProcessManager
from agent.confirmations import ConfirmationManager, QuestionManager
from agent.logger import info, warn, error, success, request as log_request

_backend: Backend | None = None


class Backend:
	def __init__(self) -> None:
		self.model = os.environ.get("WARDEN_MODEL", "qwen3:8b")
		self.api_url = os.environ.get("WARDEN_API_URL", "")
		if self.api_url:
			self.llm = OpenAIClient(self.api_url)
			self.ollama: OllamaProcessManager | None = None
			info(f"using remote API: {self.api_url}")
		else:
			self.llm = OllamaClient()
			self.ollama = OllamaProcessManager(model=self.model)
		self.confirmation_manager = ConfirmationManager()
		self.question_manager = QuestionManager()
		self.chat = ChatSession(model=self.model, client=self.llm,
		                        confirmation_manager=self.confirmation_manager,
		                        question_manager=self.question_manager)
		self.auto_mode: bool = False

	async def setup(self) -> None:
		if self.ollama is not None:
			ok = await self.ollama.ensure_running()
			if not ok:
				raise RuntimeError("failed to connect to ollama")
			if not self.ollama.has_model():
				await self.ollama.pull_model()

	def set_auto_mode(self, enabled: bool) -> None:
		self.auto_mode = enabled

	def set_thinking_enabled(self, enabled: bool) -> None:
		self.chat.set_thinking_enabled(enabled)




def _get_backend(request: web.Request) -> Backend:
	return request.app["backend"]


async def health(request: web.Request) -> web.Response:
	log_request("GET", "/health", 200)
	return web.Response(text="ok")


async def reset(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	backend.confirmation_manager.cancel_all()
	backend.question_manager.cancel_all()
	backend.chat.reset()
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


async def set_thinking(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	backend.set_thinking_enabled(bool(data.get("enabled", True)))
	status = "enabled" if backend.chat.thinking_enabled else "disabled"
	log_request("POST", "/thinking", 200)
	info(f"thinking {status}")
	return web.Response(text="ok")


async def status(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	provider = "openrouter" if backend.api_url else "ollama"
	data = {
		"model": backend.model,
		"provider": provider,
		"mode": "unleashed" if backend.auto_mode else "leashed",
		"thinking": backend.chat.thinking_enabled,
		"cwd": os.getcwd(),
	}
	log_request("GET", "/status", 200)
	return web.json_response(data)


async def question_handler(request: web.Request) -> web.Response:
	backend = _get_backend(request)
	data = await request.json()
	call_id = data.get("id", "")
	answers = data.get("answers")
	resolved = backend.question_manager.resolve(call_id, answers)
	if resolved:
		log_request("POST", "/question", 200)
		info(f"questions answered: {call_id[:8]}")
		return web.Response(text="ok")
	log_request("POST", "/question", 404)
	return web.Response(status=404, text="not found")


async def tools_list(request: web.Request) -> web.Response:
	from agent.tools import REGISTRY
	log_request("GET", "/tools", 200)
	return web.json_response({"tools": list(REGISTRY.keys())})


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
			await response.write((json.dumps({"type": "done"}) + "\n").encode())
	except (ConnectionResetError, ClientConnectionResetError):
		pass
	except Exception as e:
		if not _client_disconnected(request):
			try:
				await response.write((json.dumps({"type": "error", "text": str(e)}, ensure_ascii=False) + "\n").encode())
			except (ConnectionResetError, ClientConnectionResetError):
				pass

	return response


async def main() -> None:
	global _backend
	info("starting backend...")
	backend = Backend()
	_backend = backend
	await backend.setup()
	if backend.ollama is not None:
		success("ollama ready")
	else:
		success("remote API ready")

	app = web.Application()
	app["backend"] = backend
	app.router.add_get("/health", health)
	app.router.add_post("/reset", reset)
	app.router.add_post("/chat", chat)
	app.router.add_post("/confirm", confirm)
	app.router.add_post("/mode", set_mode)
	app.router.add_post("/thinking", set_thinking)
	app.router.add_get("/status", status)
	app.router.add_get("/tools", tools_list)
	app.router.add_post("/question", question_handler)
	runner = web.AppRunner(app)
	await runner.setup()
	site = web.TCPSite(runner, "localhost", 8765)
	await site.start()
	success("backend on http://localhost:8765")
	await asyncio.Event().wait()


if __name__ == "__main__":
	try:
		asyncio.run(main())
	except KeyboardInterrupt:
		if _backend is not None and _backend.ollama is not None:
			_backend.ollama.shutdown()
