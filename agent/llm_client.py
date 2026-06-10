import os
from abc import ABC, abstractmethod
from typing import AsyncIterator, Any, Dict, List
import dataclasses


@dataclasses.dataclass
class LLMChunk:
	thinking: str = ""
	content: str = ""
	tool_calls: List[Dict[str, Any]] | None = None


class LLMClient(ABC):
	@abstractmethod
	async def chat(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None = None,
	) -> AsyncIterator[LLMChunk]:
		...


class OllamaClient(LLMClient):
	def __init__(self, host: str | None = None) -> None:
		import ollama
		self._client = ollama.AsyncClient(host=host)

	async def chat(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None = None,
	) -> AsyncIterator[LLMChunk]:
		async for chunk in await self._client.chat(
			model=model, messages=messages, tools=tools, stream=True
		):
			msg = chunk.message if hasattr(chunk, "message") else (chunk.get("message") or {})
			thinking = getattr(msg, "thinking", None) or msg.get("thinking", "")
			content = getattr(msg, "content", None) or msg.get("content", "")
			tool_calls = getattr(msg, "tool_calls", None) or msg.get("tool_calls", [])
			yield LLMChunk(thinking=thinking, content=content, tool_calls=tool_calls)


class OpenAIClient(LLMClient):
	def __init__(self, base_url: str) -> None:
		from openai import AsyncOpenAI

		api_key = os.environ.get("OPENROUTER_API_KEY") or os.environ.get("OPENAI_API_KEY") or "sk-no-key"
		# OpenRouter optional headers for rankings
		headers = {}
		if "openrouter.ai" in base_url:
			headers["HTTP-Referer"] = "https://github.com/elev1e1nSure/warden"
			headers["X-Title"] = "warden"
		self._client = AsyncOpenAI(base_url=base_url, api_key=api_key, default_headers=headers)

	def _normalize_messages(self, messages: List[Dict[str, Any]]) -> List[Dict[str, Any]]:
		result: List[Dict[str, Any]] = []
		for msg in messages:
			if msg.get("role") == "tool":
				tool_call_id = msg.get("tool_call_id", f"call_{msg.get('name', 'unknown')}")
				result.append({
					"role": "tool",
					"tool_call_id": tool_call_id,
					"content": str(msg.get("content", "")),
				})
			elif msg.get("role") == "assistant" and msg.get("tool_calls"):
				openai_tool_calls: List[Dict[str, Any]] = []
				for i, tc in enumerate(msg["tool_calls"]):
					try:
						name = tc.function.name
						arguments = tc.function.arguments
						existing_id = tc.id
					except AttributeError:
						func = tc.get("function", {})
						name = func.get("name", "")
						arguments = func.get("arguments", "")
						existing_id = tc.get("id", "")
					tool_call_id = existing_id or f"call_{name}_{i}"
					openai_tool_calls.append({
						"id": tool_call_id,
						"type": "function",
						"function": {"name": name, "arguments": str(arguments)},
					})
				result.append({
					"role": "assistant",
					"content": str(msg.get("content", "")),
					"tool_calls": openai_tool_calls,
				})
			else:
				result.append(dict(msg))
		return result

	async def chat(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None = None,
	) -> AsyncIterator[LLMChunk]:
		openai_messages = self._normalize_messages(messages)
		kwargs: Dict[str, Any] = {}
		if tools:
			kwargs["tools"] = tools
			kwargs["tool_choice"] = "auto"

		stream = await self._client.chat.completions.create(
			model=model,
			messages=openai_messages,
			stream=True,
			**kwargs,
		)

		accumulated_tool_calls: List[Dict[str, Any]] = []

		async for chunk in stream:
			delta = chunk.choices[0].delta
			if delta.tool_calls:
				for tc in delta.tool_calls:
					while len(accumulated_tool_calls) <= tc.index:
						accumulated_tool_calls.append({
							"id": f"call_{tc.index}",
							"type": "function",
							"function": {"name": "", "arguments": ""},
						})
					if tc.function.name:
						accumulated_tool_calls[tc.index]["function"]["name"] = tc.function.name
					if tc.function.arguments:
						accumulated_tool_calls[tc.index]["function"]["arguments"] += tc.function.arguments

			if delta.content:
				yield LLMChunk(content=delta.content)

		if accumulated_tool_calls:
			yield LLMChunk(tool_calls=accumulated_tool_calls)
