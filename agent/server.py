import asyncio
import json
import os
import sys

# Установка UTF-8 кодировки для Windows
if sys.platform == "win32":
	import io
	sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
	sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')

from aiohttp import web
from aiohttp.client_exceptions import ClientConnectionResetError

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from agent.chat import ChatSession
from agent.ollama_client import OllamaClient
from agent.tools import PENDING
from agent.logger import info, warn, error, success, request, tool


class Backend:
	def __init__(self, model: str = "qwen3:8b") -> None:
		self.model = model
		self.ollama = OllamaClient(model=model)
		self.chat = ChatSession(model=model)
		self.auto_mode: bool = False

	async def setup(self) -> None:
		ok = await self.ollama.ensure_running()
		if not ok:
			raise RuntimeError("failed to connect to ollama")
		if not self.ollama.has_model():
			await self.ollama.pull_model()


backend = Backend()


async def health(request: web.Request) -> web.Response:
	request("GET", "/health", 200)
	return web.Response(text="ok")


async def reset(request: web.Request) -> web.Response:
	backend.chat.reset()
	request("POST", "/reset", 200)
	info("session reset")
	return web.Response(text="ok")


async def set_mode(request: web.Request) -> web.Response:
	data = await request.json()
	backend.auto_mode = bool(data.get("auto", False))
	mode = "AUTO" if backend.auto_mode else "SAFE"
	request("POST", "/mode", 200)
	info(f"mode changed to {mode}")
	return web.Response(text="ok")


async def set_thinking(request: web.Request) -> web.Response:
	data = await request.json()
	backend.chat.thinking_enabled = bool(data.get("enabled", True))
	status = "enabled" if backend.chat.thinking_enabled else "disabled"
	request("POST", "/thinking", 200)
	info(f"thinking {status}")
	return web.Response(text="ok")


async def confirm(request: web.Request) -> web.Response:
	data = await request.json()
	call_id = data.get("id", "")
	ok = bool(data.get("ok", False))
	entry = PENDING.get(call_id)
	if entry:
		entry["ok"] = ok
		entry["event"].set()
		request("POST", "/confirm", 200)
		action = "confirmed" if ok else "cancelled"
		info(f"action {action}")
		return web.Response(text="ok")
	request("POST", "/confirm", 404)
	warn(f"confirm not found: {call_id}")
	return web.Response(status=404, text="not found")


async def chat(request: web.Request) -> web.StreamResponse:
	data = await request.json()
	text = data.get("text", "")
	request("POST", "/chat")
	info(f"user: {text[:50]}..." if len(text) > 50 else f"user: {text}")

	response = web.StreamResponse(
		status=200,
		headers={"Content-Type": "application/x-ndjson"},
	)
	await response.prepare(request)

	try:
		async for type_, payload in backend.chat.stream(text, auto_mode=backend.auto_mode):
			if request.transport.is_closing():
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
				msg = {"type": "confirm", "id": payload["id"], "tool": payload["tool"], "args": payload["args"]}
			else:
				continue
			try:
				await response.write((json.dumps(msg, ensure_ascii=False) + "\n").encode())
			except (ConnectionResetError, ClientConnectionResetError):
				break
		if not request.transport.is_closing():
			await response.write((json.dumps({"type": "done"}) + "\n").encode())
	except (ConnectionResetError, ClientConnectionResetError):
		pass
	except Exception as e:
		if not request.transport.is_closing():
			try:
				await response.write((json.dumps({"type": "error", "text": str(e)}, ensure_ascii=False) + "\n").encode())
			except (ConnectionResetError, ClientConnectionResetError):
				pass

	return response


async def main() -> None:
	info("starting backend...")
	await backend.setup()
	success("ollama ready")
	
	app = web.Application()
	app.router.add_get("/health", health)
	app.router.add_post("/reset", reset)
	app.router.add_post("/chat", chat)
	app.router.add_post("/confirm", confirm)
	app.router.add_post("/mode", set_mode)
	app.router.add_post("/thinking", set_thinking)
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
		backend.ollama.shutdown()
