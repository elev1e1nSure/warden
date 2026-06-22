import re
import uuid
import json
from typing import AsyncIterator, List, Dict, Any

from agent.confirmations import ConfirmationManager, QuestionManager
from agent.llm_client import LLMChunk, LLMClient
from agent.memory.aggregator import MemoryAggregator
from agent.memory.extractor import MemoryExtractor
from agent.memory.store import MemoryStore
from agent.prompt import build_system
from agent.skills import Skill, find_skill, wrap_skill_content
from agent.tool_runner import execute_tool_call
from agent.tools import REGISTRY

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


def _has_images(messages: list) -> bool:
	for msg in messages:
		if msg.get("images"):
			return True
		content = msg.get("content")
		if isinstance(content, list):
			for part in content:
				if isinstance(part, dict) and part.get("type") == "image_url":
					return True
	return False


def _strip_images(messages: list) -> list:
	result = []
	for msg in messages:
		if msg.get("images"):
			result.append({k: v for k, v in msg.items() if k != "images"})
		elif isinstance(msg.get("content"), list):
			filtered = [p for p in msg["content"] if not (isinstance(p, dict) and p.get("type") == "image_url")]
			m = dict(msg)
			m["content"] = filtered if filtered else ""
			result.append(m)
		else:
			result.append(msg)
	return result


def _is_vision_error(e: Exception) -> bool:
	s = str(e).lower()
	return any(kw in s for kw in (
		"image", "vision", "multimodal", "does not support",
		"unsupported content", "image_url", "not support image",
	))

def _guess_context_limit(model: str) -> int:
	lower = model.lower()
	# rough heuristics without hardcoded lists
	if "128k" in lower or "128000" in lower:
		return 128000
	if "32k" in lower or "32768" in lower:
		return 32768
	if "8k" in lower or "8192" in lower:
		return 8192
	if "4k" in lower or "4096" in lower:
		return 4096
	return 128000

def _clean_visible_text(text: str) -> str:
	return _EMOJI_RE.sub("", text)


def _skill_context_messages(skill: Skill, args: str | None = None) -> list[dict[str, Any]]:
	call_id = f"call_skill_{skill.name.replace('-', '_')}"
	skill_args = {"name": skill.name}
	if args:
		skill_args["args"] = args
	content = wrap_skill_content(skill)
	if args:
		content = f"User provided arguments: {args}\n\n{content}"
	return [
		{
			"role": "assistant",
			"content": "",
			"tool_calls": [
				{
					"id": call_id,
					"type": "function",
					"function": {
						"name": "skill",
						"arguments": json.dumps(skill_args),
					},
				}
			],
		},
		{
			"role": "tool",
			"name": "skill",
			"tool_call_id": call_id,
			"content": content,
		},
	]


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


class ChatSession:
	def __init__(self, model: str, client: LLMClient,
	             confirmation_manager: ConfirmationManager | None = None,
	             question_manager: QuestionManager | None = None,
	             memory_store: MemoryStore | None = None) -> None:
		self.model = model
		self.history: List[Dict[str, Any]] = []
		self._client = client
		self.confirmation_manager = confirmation_manager
		self.question_manager = question_manager
		self.memory_store = memory_store
		self.session_id: str = str(uuid.uuid4())
		self._extractor = MemoryExtractor()
		self.token_count: int = 0
		self.token_limit: int = _guess_context_limit(model)


	def reset(self) -> None:
		if self.memory_store is not None:
			MemoryAggregator.finalize(self.memory_store, self.session_id)
		self.history = []
		self.token_count = 0
		self.session_id = str(uuid.uuid4())

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
		system = build_system(self.model)
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

	def add_user(self, text: str) -> None:
		self.history.append({"role": "user", "content": text})
		if self.memory_store is not None and self.memory_store.get_enabled():
			for fact in self._extractor.extract(text):
				self.memory_store.upsert_entry(
					self.session_id,
					fact.category,
					fact.key,
					fact.value,
					fact.confidence,
				)

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
		# Do NOT persist reasoning/reasoning_details in history.
		# Anthropic rejects thinking blocks with invalid signatures
		# when they are round-tripped through JSON serialization.
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
				if chunk.usage_tokens:
					result["usage_tokens"] = chunk.usage_tokens
					continue

				if chunk.tool_calls:
					collected_tool_calls.extend(chunk.tool_calls)

				thinking = chunk.thinking
				content = chunk.content

				if thinking:
					yield ("think", thinking)
				elif chunk.reasoning:
					full_reasoning += chunk.reasoning
					yield ("think", chunk.reasoning)
				elif chunk.reasoning_details:
					reasoning_text = _reasoning_details_text(chunk.reasoning_details)
					if reasoning_text:
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
			if _is_vision_error(e) and _has_images(messages):
				yield ("token", "\nSorry, this model doesn't support images. Retrying without them...")
				stripped = _strip_images(messages)
				try:
					async for chunk in self._client.chat(
						model=self.model,
						messages=stripped,
						tools=_TOOLS,
					):
						if chunk.usage_tokens:
							result["usage_tokens"] = chunk.usage_tokens
							continue
						if chunk.tool_calls:
							collected_tool_calls.extend(chunk.tool_calls)
						if chunk.content:
							clean = _clean_visible_text(chunk.content)
							yield ("token", clean)
							full_content += clean
				except Exception as e2:
					yield ("token", f"\nconnection error: {e2}")
					result["error"] = True
			else:
				yield ("token", f"\nconnection error: {e}")
				result["error"] = True

		result["content"] = full_content
		result["tool_calls"] = collected_tool_calls

	async def _execute_tool_call(self, tc, auto_mode: bool) -> AsyncIterator[tuple]:
		async for event in execute_tool_call(
			tc,
			auto_mode,
			history=self.history,
			confirmation_manager=self.confirmation_manager,
			question_manager=self.question_manager,
			add_tool_result_fn=self.add_tool_result,
		):
			yield event

	async def stream(
		self,
		text: str,
		auto_mode: bool = False,
		skill_name: str | None = None,
		skill_args: str | None = None,
	) -> AsyncIterator[tuple[str, Any]]:
		turn_context: list[dict[str, Any]] = []
		if skill_name:
			skill = find_skill(skill_name)
			if skill is None:
				yield ("token", f"skill not found: {skill_name}")
				return
			turn_context = _skill_context_messages(skill, skill_args)

		history_insert_at = len(self.history) + 1
		self.add_user(text)
		iter_count = 0

		while iter_count < MAX_ITER:
			iter_count += 1
			yield ("warden_start", {})

			system = build_system(self.model)
			if self.memory_store is not None and self.memory_store.get_enabled():
				mem_ctx = self.memory_store.get_context_text(session_id=self.session_id)
				if mem_ctx:
					system = mem_ctx + "\n\n" + system
			if turn_context:
				history = self.history[:history_insert_at] + turn_context + self.history[history_insert_at:]
			else:
				history = self.history
			messages = [{"role": "system", "content": system}] + history

			llm_result: dict = {}
			async for event in self._call_llm(messages, llm_result):
				yield event

			if llm_result.get("error"):
				break

			full_content = llm_result.get("content", "")
			collected_tool_calls = llm_result.get("tool_calls", [])

			self.add_assistant(
				full_content,
				collected_tool_calls or None,
			)
			usage = llm_result.get("usage_tokens", 0)
			self.token_count = usage if usage > 0 else self._estimate_tokens()

			if not collected_tool_calls:
				break

			for tc in collected_tool_calls:
				async for event in self._execute_tool_call(tc, auto_mode):
					yield event

		if iter_count >= MAX_ITER:
			yield ("token", "\n[iteration limit reached]")
