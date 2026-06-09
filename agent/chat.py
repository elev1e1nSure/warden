import asyncio
from typing import AsyncIterator, List, Dict, Any

import ollama

from agent.tools import REGISTRY, PENDING, gen_id, parse_args

SYSTEM = (
	"Ты — warden, личный ИИ-ассистент на компьютере пользователя. "
	"Отвечай по-русски, кратко и по делу. Без смайлов и эмодзи. "
	"Без вступлений и объяснений — сразу к делу. "
	"Используй инструменты самостоятельно, строй цепочки действий до конца задачи. "
	"Не спрашивай разрешения перед каждым шагом — действуй. "
	"Для удаления файлов используй file_delete. "
	"Для поиска видео используй youtube_search, потом browser_open чтобы открыть. "
	"Для чтения страниц и навигации используй browser_read. "
	"Для работы с экраном: screenshot → mouse/keyboard. "
	"Если что-то не нашлось — попробуй другой подход, не останавливайся сразу."
)

_TOOLS = [t.to_ollama() for t in REGISTRY.values()]
MAX_ITER = 20


def _chunk_parts(chunk) -> tuple[str, str]:
	try:
		msg = chunk.message
		return (getattr(msg, "thinking", None) or ""), (getattr(msg, "content", None) or "")
	except AttributeError:
		msg = chunk.get("message") or {}
		return (msg.get("thinking") or ""), (msg.get("content") or "")


def _get_tool_calls(chunk) -> list:
	try:
		msg = chunk.message
		return getattr(msg, "tool_calls", None) or []
	except AttributeError:
		msg = chunk.get("message") or {}
		return msg.get("tool_calls") or []


class ChatSession:
	def __init__(self, model: str) -> None:
		self.model = model
		self.history: List[Dict[str, Any]] = []
		self._client = ollama.AsyncClient()

	def reset(self) -> None:
		self.history = []

	def add_user(self, text: str) -> None:
		self.history.append({"role": "user", "content": text})

	def add_assistant(self, text: str, tool_calls: list | None = None) -> None:
		msg: Dict[str, Any] = {"role": "assistant", "content": text}
		if tool_calls:
			msg["tool_calls"] = tool_calls
		self.history.append(msg)

	def add_tool_result(self, tool_name: str, result: str) -> None:
		self.history.append({"role": "tool", "content": result, "name": tool_name})

	async def stream(self, text: str, auto_mode: bool = False) -> AsyncIterator[tuple[str, Any]]:
		self.add_user(text)
		iter_count = 0

		while iter_count < MAX_ITER:
			iter_count += 1
			yield ("warden_start", {})

			messages = [{"role": "system", "content": SYSTEM}] + self.history
			full_content = ""
			in_think = False
			collected_tool_calls: list = []

			try:
				async for chunk in await self._client.chat(
					model=self.model,
					messages=messages,
					tools=_TOOLS,
					stream=True,
				):
					tcs = _get_tool_calls(chunk)
					if tcs:
						collected_tool_calls.extend(tcs)
						continue

					thinking, content = _chunk_parts(chunk)

					if thinking:
						yield ("think", thinking)
						continue

					if not content:
						continue

					text_chunk = content
					while text_chunk:
						if not in_think:
							idx = text_chunk.find("<think>")
							if idx == -1:
								yield ("token", text_chunk)
								full_content += text_chunk
								text_chunk = ""
							else:
								if idx > 0:
									yield ("token", text_chunk[:idx])
									full_content += text_chunk[:idx]
								text_chunk = text_chunk[idx + 7:]
								in_think = True
						else:
							idx = text_chunk.find("</think>")
							if idx == -1:
								yield ("think", text_chunk)
								text_chunk = ""
							else:
								if idx > 0:
									yield ("think", text_chunk[:idx])
								text_chunk = text_chunk[idx + 8:]
								in_think = False
			except Exception as e:
				yield ("token", f"\nошибка соединения: {e}")
				break

			self.add_assistant(full_content, collected_tool_calls or None)

			if not collected_tool_calls:
				break

			for tc in collected_tool_calls:
				try:
					name = tc.function.name
					raw_args = tc.function.arguments
				except AttributeError:
					name = tc.get("function", {}).get("name", "")
					raw_args = tc.get("function", {}).get("arguments", {})

				tool = REGISTRY.get(name)
				if not tool:
					self.add_tool_result(name, f"ошибка: инструмент '{name}' не найден")
					continue

				args = parse_args(raw_args)
				args_str = ", ".join(f"{k}={v}" for k, v in args.items())

				if tool.is_dangerous(args) and not auto_mode:
					call_id = gen_id()
					event = asyncio.Event()
					PENDING[call_id] = {"event": event, "ok": False}
					yield ("confirm", {"id": call_id, "tool": name, "args": args_str})
					await event.wait()
					ok = PENDING.pop(call_id, {}).get("ok", False)
					if not ok:
						self.add_tool_result(name, "отменено пользователем")
						yield ("tool", {"name": name, "args": args_str, "result": "отменено"})
						continue

				yield ("tool_start", {"name": name, "args": args_str})
				try:
					result = await asyncio.wait_for(tool.execute(args), timeout=60)
				except asyncio.TimeoutError:
					result = "ошибка: таймаут 60с"
				except Exception as e:
					result = f"ошибка: {e}"
				yield ("tool", {"name": name, "args": args_str, "result": result})
				self.add_tool_result(name, result)

		if iter_count >= MAX_ITER:
			yield ("token", "\n[достигнут лимит итераций]")
