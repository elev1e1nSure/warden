import asyncio
from typing import AsyncIterator, List, Dict, Any

import ollama

from agent.confirmations import ConfirmationManager
from agent.safety import assess_tool_call
from agent.tools import REGISTRY, parse_args

SYSTEM = (
	"You are warden, a detached guardian inside the user's computer. "
	"Respond in the user's language, informally, briefly and calmly. "
	"Tone is heavy, terse, focused. Don't try to be liked. "
	"No report style, no listing capabilities, no stating the obvious. "
	"Don't voice internal reasoning, thought chains or interim ideas. "
	"No action-report form or enumerating what you do. "
	"No introductions, no extra politeness, no formal 'you'. Straight to the point. "
	"Don't tell about internal checks or background actions unless asked. "
	"Don't repeat or interpret service tokens, modes or commands unless directly asked. "
	"Answer short everyday questions briefly, dryly, like a human. "
	"Use tools autonomously and carry action chains through to completion. "
	"Don't ask permission before each step — just act. "
	"For screen work: take a screenshot first, then move cursor and click coordinates. Never click blindly. "
	"Don't claim you pressed, opened or typed something unless you used the matching tool. "
	"For websites use browser_read and browser_screenshot as the main Playwright path. browser_open is only to open a URL for the user. "
	"For file deletion use file_delete. "
	"For video search use youtube_search, then browser_open to open it. "
	"For reading pages and navigation use browser_read. "
	"If something isn't found — try another approach, don't stop immediately. "
	"Shell runtime: PowerShell (Windows PowerShell). Use the 'powershell' tool. "
	"For syntax, operators, and safe command patterns read `.warden/powershell-reference.md` via file_read."
)

_TOOLS = [t.to_ollama() for t in REGISTRY.values()]
MAX_ITER = 20


def _extract_message(chunk: Any) -> dict:
	try:
		msg = chunk.message
		return {
			"thinking": getattr(msg, "thinking", None) or "",
			"content": getattr(msg, "content", None) or "",
			"tool_calls": getattr(msg, "tool_calls", None) or [],
		}
	except AttributeError:
		msg = chunk.get("message") or {}
		return {
			"thinking": msg.get("thinking") or "",
			"content": msg.get("content") or "",
			"tool_calls": msg.get("tool_calls") or [],
		}


def _chunk_parts(chunk: Any) -> tuple[str, str]:
	msg = _extract_message(chunk)
	return msg["thinking"], msg["content"]


def _get_tool_calls(chunk: Any) -> list:
	return _extract_message(chunk)["tool_calls"]


class ChatSession:
	def __init__(self, model: str, confirmation_manager: ConfirmationManager | None = None) -> None:
		self.model = model
		self.history: List[Dict[str, Any]] = []
		self._client = ollama.AsyncClient()
		self.thinking_enabled: bool = True
		self.confirmation_manager = confirmation_manager

	def reset(self) -> None:
		self.history = []

	def set_thinking_enabled(self, enabled: bool) -> None:
		self.thinking_enabled = enabled

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
						if self.thinking_enabled:
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
				yield ("token", f"\nconnection error: {e}")
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
					self.add_tool_result(name, f"error: tool '{name}' not found")
					continue

				args = parse_args(raw_args)
				args_str = ", ".join(f"{k}={v}" for k, v in args.items())

				# Safety assessment — the model proposes, the policy decides
				decision = assess_tool_call(name, args)
				if decision.risk == "blocked":
					self.add_tool_result(name, f"blocked: {decision.reason}")
					yield ("tool", {"name": name, "args": args_str, "result": f"blocked: {decision.reason}"})
					continue

				if decision.risk == "confirm" and not auto_mode:
					if self.confirmation_manager is None:
						self.add_tool_result(name, "cancelled: no confirmation manager")
						yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
						continue
					call_id, event = self.confirmation_manager.register()
					confirm_payload = {
						"id": call_id,
						"tool": name,
						"risk": decision.risk,
						"title": decision.summary,
						"summary": decision.reason,
						"details": decision.details,
						"args": args_str,
						"preview": str(args.get("command", args.get("path", args_str))),
						"default": "cancel",
					}
					yield ("confirm", confirm_payload)
					await event.wait()
					resolved = self.confirmation_manager.pop(call_id)
					if resolved is None or not resolved.get("ok", False):
						self.add_tool_result(name, "cancelled by user")
						yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
						continue
					# Re-verify after confirm: tool name + args must still match
					post_decision = assess_tool_call(name, args)
					if post_decision.risk == "blocked":
						self.add_tool_result(name, f"blocked after confirm: {post_decision.reason}")
						yield ("tool", {"name": name, "args": args_str, "result": f"blocked: {post_decision.reason}"})
						continue

				yield ("tool_start", {"name": name, "args": args_str})
				try:
					result = await asyncio.wait_for(tool.execute(args), timeout=60)
				except asyncio.TimeoutError:
					result = "error: timeout 60s"
				except Exception as e:
					result = f"error: {e}"
				yield ("tool", {"name": name, "args": args_str, "result": result})
				self.add_tool_result(name, result)

		if iter_count >= MAX_ITER:
			yield ("token", "\n[iteration limit reached]")
