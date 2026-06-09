import asyncio
import json
import os
import sys
from typing import AsyncIterator

from aiohttp import web

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from agent.chat import ChatSession
from agent.ollama_client import OllamaClient


class Backend:
	def __init__(self, model: str = "qwen3:8b") -> None:
		self.model = model
		self.ollama = OllamaClient(model=model)
		self.chat = ChatSession(model=model)

	async def setup(self) -> None:
		ok = await self.ollama.ensure_running()
		if not ok:
			raise RuntimeError("failed to connect to ollama")
		if not self.ollama.has_model():
			await self.ollama.pull_model()

	async def chat_stream(self, text: str) -> AsyncIterator[dict]:
		for token in self.chat.stream(text):
			yield {"type": "token", "text": token}
		yield {"type": "done"}


backend = Backend()


async def health(request: web.Request) -> web.Response:
	return web.Response(text="ok")


async def chat(request: web.Request) -> web.StreamResponse:
	data = await request.json()
	text = data.get("text", "")

	response = web.StreamResponse(
		status=200,
		headers={"Content-Type": "application/x-ndjson"},
	)
	await response.prepare(request)

	try:
		async for msg in backend.chat_stream(text):
			line = json.dumps(msg) + "\n"
			await response.write(line.encode())
	except Exception as e:
		line = json.dumps({"type": "error", "text": str(e)}) + "\n"
		await response.write(line.encode())

	return response


async def main() -> None:
	await backend.setup()
	app = web.Application()
	app.router.add_get("/health", health)
	app.router.add_post("/chat", chat)
	runner = web.AppRunner(app)
	await runner.setup()
	site = web.TCPSite(runner, "localhost", 8765)
	await site.start()
	print("backend on http://localhost:8765")
	await asyncio.Event().wait()


if __name__ == "__main__":
	try:
		asyncio.run(main())
	except KeyboardInterrupt:
		backend.ollama.shutdown()
