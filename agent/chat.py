import asyncio
import re
from pathlib import Path
from typing import AsyncIterator, List, Dict, Any

from agent.confirmations import ConfirmationManager, QuestionManager
from agent.llm_client import LLMChunk, LLMClient
from agent.prompt import SYSTEM
from agent.safety import assess_tool_call
from agent.tools import REGISTRY, parse_args

_EMOJI_RE = re.compile(
	"["
	"\U0001F1E6-\U0001F1FF"
	"\U0001F300-\U0001FAFF"
	"\U00002700-\U000027BF"
	"\U00002600-\U000026FF"
	"]+",
	flags=re.UNICODE,
)

_TOOLS = [t.tool_definition() for t in REGISTRY.values()]
MAX_ITER = 20

_COMPACT_PROMPT = (
	"Summarize the conversation above in a few sentences. "
	"Keep all key facts, decisions, file paths, and tool results. "
	"Discard chatty filler."
)

_CONTEXT_LIMITS: dict[str, int] = {
	"llama3": 8192,
	"llama2": 4096,
	"mistral": 8192,
	"mixtral": 32768,
	"codellama": 16384,
	"deepseek": 16384,
	"qwen": 32768,
	"gemma": 8192,
	"phi": 4096,
}


def _guess_context_limit(model: str) -> int:
	lower = model.lower()
	for key, limit in _CONTEXT_LIMITS.items():
		if key in lower:
			return limit
	return 128000

def _clean_visible_text(text: str) -> str:
	return _EMOJI_RE.sub("", text)


def _reasoning_details_text(details: list[dict[str, Any]] | None) -> str:
	if not details:
		return ""
	parts: list[str] = []
	for item in details:
		if not isinstance(item, dict):
			continue
		for key in ("text", "summary", "content"):
			value = item.get(key)
			if isinstance(value, str) and value.strip():
				parts.append(value)
				break
	return "".join(parts)


def _resolve_preview(args: dict, fallback: str) -> str:
	if "command" in args:
		return str(args["command"])
	if "path" in args:
		try:
			return str(Path(str(args["path"])).resolve())
		except Exception:
			return str(args["path"])
	return fallback


class ChatSession:
	def __init__(self, model: str, client: LLMClient,
	             confirmation_manager: ConfirmationManager | None = None,
	             question_manager: QuestionManager | None = None) -> None:
		self.model = model
		self.history: List[Dict[str, Any]] = []
		self._client = client
		self.thinking_enabled: bool = True
		self.confirmation_manager = confirmation_manager
		self.question_manager = question_manager
		self.token_count: int = 0
		self.token_limit: int = _guess_context_limit(model)

	def reset(self) -> None:
		self.history = []
		self.token_count = 0

	def _estimate_tokens(self) -> int:
		total = 0
		for msg in self.history:
			content = msg.get("content", "")
			if isinstance(content, str):
				total += len(content) // 4
			elif isinstance(content, list):
				for part in content:
					if isinstance(part, dict):
						total += len(str(part.get("text", ""))) // 4
		return max(total, 0)

	async def compact(self) -> dict:
		if len(self.history) < 2:
			return {"summary": "nothing to compact", "tokens_before": self.token_count, "tokens_after": self.token_count}

		tokens_before = self._estimate_tokens()
		system = SYSTEM + f" Configured model name: {self.model}."
		messages = [{"role": "system", "content": system}] + self.history + [
			{"role": "user", "content": _COMPACT_PROMPT}
		]

		summary = ""
		try:
			async for chunk in self._client.chat(model=self.model, messages=messages):
				if chunk.content:
					summary += chunk.content
		except Exception as e:
			return {"summary": f"error: {e}", "tokens_before": tokens_before, "tokens_after": tokens_before}

		self.history = [
			{"role": "user", "content": "[Conversation summary]"},
			{"role": "assistant", "content": summary},
		]
		self.token_count = self._estimate_tokens()
		return {"summary": summary, "tokens_before": tokens_before, "tokens_after": self.token_count}

	def set_thinking_enabled(self, enabled: bool) -> None:
		self.thinking_enabled = enabled

	def add_user(self, text: str) -> None:
		self.history.append({"role": "user", "content": text})

	def add_assistant(
		self,
		text: str,
		tool_calls: list | None = None,
		reasoning: str = "",
		reasoning_details: list[dict[str, Any]] | None = None,
	) -> None:
		msg: Dict[str, Any] = {"role": "assistant", "content": text}
		if tool_calls:
			msg["tool_calls"] = tool_calls
		if reasoning:
			msg["reasoning"] = reasoning
		if reasoning_details:
			msg["reasoning_details"] = reasoning_details
		self.history.append(msg)

	def add_tool_result(self, tool_name: str, result: str, tool_call_id: str = "") -> None:
		entry: Dict[str, Any] = {"role": "tool", "content": result, "name": tool_name}
		if tool_call_id:
			entry["tool_call_id"] = tool_call_id
		self.history.append(entry)

	async def _call_llm(self, messages: list, result: dict) -> AsyncIterator[tuple]:
		"""Stream LLM response. Yields (type, value) events. Fills result with collected state."""
		full_content = ""
		full_reasoning = ""
		in_think = False
		collected_tool_calls: list = []
		collected_reasoning_details: list[dict[str, Any]] = []

		try:
			async for chunk in self._client.chat(
				model=self.model,
				messages=messages,
				tools=_TOOLS,
			):
				if chunk.tool_calls:
					collected_tool_calls.extend(chunk.tool_calls)

				thinking = chunk.thinking
				content = chunk.content

				if thinking and self.thinking_enabled:
					yield ("think", thinking)

				if chunk.reasoning:
					full_reasoning += chunk.reasoning
					if self.thinking_enabled:
						yield ("think", chunk.reasoning)
				elif chunk.reasoning_details:
					reasoning_text = _reasoning_details_text(chunk.reasoning_details)
					if reasoning_text and self.thinking_enabled:
						yield ("think", reasoning_text)

				if chunk.reasoning_details:
					collected_reasoning_details.extend(chunk.reasoning_details)

				if not content:
					continue

				text_chunk = content
				while text_chunk:
					if not in_think:
						idx = text_chunk.find("<think>")
						if idx == -1:
							clean = _clean_visible_text(text_chunk)
							yield ("token", clean)
							full_content += clean
							text_chunk = ""
						else:
							if idx > 0:
								clean = _clean_visible_text(text_chunk[:idx])
								yield ("token", clean)
								full_content += clean
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
			result["error"] = True

		result["content"] = full_content
		result["reasoning"] = full_reasoning
		result["tool_calls"] = collected_tool_calls
		result["reasoning_details"] = collected_reasoning_details

	async def _execute_tool_call(self, tc, auto_mode: bool) -> AsyncIterator[tuple]:
		"""Execute a single tool call. Yields events and records results in history."""
		try:
			name = tc.function.name
			raw_args = tc.function.arguments
			tool_call_id = tc.id
		except AttributeError:
			func = tc.get("function", {})
			name = func.get("name", "")
			raw_args = func.get("arguments", {})
			tool_call_id = tc.get("id", "")

		tool = REGISTRY.get(name)
		if not tool:
			self.add_tool_result(name, f"error: tool '{name}' not found")
			return

		args = parse_args(raw_args)
		args_str = ", ".join(f"{k}={v}" for k, v in args.items())

		# ── question tool: special interactive flow ──
		if name == "question":
			if self.question_manager is None:
				self.add_tool_result(name, "error: no question manager")
				yield ("tool", {"name": name, "args": args_str, "result": "error: no question manager"})
				return
			questions = args.get("questions", [])
			if not questions:
				self.add_tool_result(name, "error: no questions provided")
				yield ("tool", {"name": name, "args": args_str, "result": "error: no questions"})
				return
			call_id, _ = self.question_manager.register(questions)
			yield ("question", {"id": call_id, "questions": questions})
			answers = await self.question_manager.wait(call_id)
			if answers is None:
				answers = [[] for _ in questions]
			formatted = ", ".join(
				f'"{q.get("question", "")}"="{", ".join(a) if a else "Unanswered"}"'
				for q, a in zip(questions, answers)
			)
			result_str = f"User answered: {formatted}"
			yield ("tool", {"name": name, "args": args_str, "result": result_str})
			self.add_tool_result(name, result_str, tool_call_id)
			return

		# ── regular tool execution with safety ──
		mode = "auto" if auto_mode else "ask"
		decision = assess_tool_call(name, args, mode=mode)
		if decision.risk == "blocked":
			self.add_tool_result(name, f"blocked: {decision.reason}")
			yield ("tool", {"name": name, "args": args_str, "result": f"blocked: {decision.reason}"})
			return

		if decision.risk == "confirm":
			if self.confirmation_manager is None:
				self.add_tool_result(name, "cancelled: no confirmation manager")
				yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
				return
			call_id, _ = self.confirmation_manager.register()
			confirm_payload = {
				"id": call_id,
				"tool": name,
				"risk": decision.risk,
				"title": decision.summary,
				"summary": decision.reason,
				"details": decision.details,
				"args": args_str,
				"preview": _resolve_preview(args, args_str),
				"default": "cancel",
			}
			yield ("confirm", confirm_payload)
			ok = await self.confirmation_manager.wait(call_id)
			if not ok:
				self.add_tool_result(name, "cancelled by user")
				yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
				return

		yield ("tool_start", {"name": name, "args": args_str})
		try:
			result_val = await asyncio.wait_for(tool.execute(args), timeout=60)
		except asyncio.TimeoutError:
			result_val = "error: timeout 60s"
		except RuntimeError as e:
			if "question tool must be handled" in str(e):
				result_val = "error: question tool needs interactive flow"
			else:
				result_val = f"error: {e}"
		except Exception as e:
			result_val = f"error: {e}"
		yield ("tool", {"name": name, "args": args_str, "result": result_val})
		self.add_tool_result(name, result_val, tool_call_id)

	async def stream(self, text: str, auto_mode: bool = False) -> AsyncIterator[tuple[str, Any]]:
		self.add_user(text)
		iter_count = 0

		while iter_count < MAX_ITER:
			iter_count += 1
			yield ("warden_start", {})

			system = SYSTEM + f" Configured model name: {self.model}."
			messages = [{"role": "system", "content": system}] + self.history

			llm_result: dict = {}
			async for event in self._call_llm(messages, llm_result):
				yield event

			if llm_result.get("error"):
				break

			full_content = llm_result.get("content", "")
			full_reasoning = llm_result.get("reasoning", "")
			collected_tool_calls = llm_result.get("tool_calls", [])
			collected_reasoning_details = llm_result.get("reasoning_details", [])

			self.add_assistant(
				full_content,
				collected_tool_calls or None,
				reasoning=full_reasoning,
				reasoning_details=collected_reasoning_details or None,
			)
			self.token_count = self._estimate_tokens()

			if not collected_tool_calls:
				break

			for tc in collected_tool_calls:
				async for event in self._execute_tool_call(tc, auto_mode):
					yield event

		if iter_count >= MAX_ITER:
			yield ("token", "\n[iteration limit reached]")
