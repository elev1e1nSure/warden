import os
from abc import ABC, abstractmethod
from typing import AsyncIterator, Any, Dict, List
import dataclasses


@dataclasses.dataclass
class LLMChunk:
	thinking: str = ""
	content: str = ""
	reasoning: str = ""
	reasoning_details: List[Dict[str, Any]] | None = None
	tool_calls: List[Dict[str, Any]] | None = None
	usage_tokens: int = 0


class LLMClient(ABC):
	@abstractmethod
	async def chat(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None = None,
	) -> AsyncIterator[LLMChunk]:
		...

	@abstractmethod
	async def list_models(self) -> List[str]:
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

	async def list_models(self) -> List[str]:
		result = await self._client.list()
		models = result.models if hasattr(result, "models") else result.get("models", [])
		names: List[str] = []
		for m in models:
			name = getattr(m, "model", None) or (m.get("model") if isinstance(m, dict) else None)
			if name:
				names.append(name)
		return sorted(names)


class OpenAIClient(LLMClient):
	def __init__(self, base_url: str, api_key: str | None = None) -> None:
		from openai import AsyncOpenAI
		import logging

		api_key = api_key or os.environ.get("OPENROUTER_API_KEY") or os.environ.get("OPENAI_API_KEY") or "sk-no-key"
		if api_key == "sk-no-key":
			logging.warning("Using fallback API key 'sk-no-key' - requests may fail")
		headers = {}
		self._is_openrouter = "openrouter.ai" in base_url
		if self._is_openrouter:
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
				assistant_msg: Dict[str, Any] = {
					"role": "assistant",
					"content": str(msg.get("content", "")),
					"tool_calls": openai_tool_calls,
				}
				result.append(assistant_msg)
			elif msg.get("role") == "assistant":
				clean = {k: v for k, v in msg.items() if k not in ("reasoning", "reasoning_details")}
				result.append(clean)
			elif msg.get("role") == "user" and "images" in msg:
				images = msg["images"]
				text = msg.get("content", "")
				content_list: List[Dict[str, Any]] = []
				if text:
					content_list.append({"type": "text", "text": str(text)})
				for img_b64 in images:
					content_list.append({
						"type": "image_url",
						"image_url": {"url": f"data:image/png;base64,{img_b64}"},
					})
				result.append({"role": "user", "content": content_list})
			else:
				result.append(dict(msg))
		return result

	async def _create_stream(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None,
	):
		"""Create a streaming completion, retrying after stripping unsupported features."""
		from openai import APIStatusError

		use_tools = bool(tools)
		use_tool_choice = bool(tools)
		use_reasoning = self._is_openrouter

		while True:
			kw: Dict[str, Any] = {}
			if use_tools and tools:
				kw["tools"] = tools
				if use_tool_choice:
					kw["tool_choice"] = "auto"
			if use_reasoning:
				kw["extra_body"] = {"reasoning": {"enabled": True}}

			try:
				return await self._client.chat.completions.create(
					model=model,
					messages=messages,
					stream=True,
					stream_options={"include_usage": True},
					**kw,
				)
			except APIStatusError as e:
				body = str(e.body or "").lower() + str(getattr(e, "message", "") or "").lower()
				if e.status_code in (400, 404):
					if "tool_choice" in body and use_tool_choice:
						use_tool_choice = False
						continue
					if ("tool" in body or "function" in body) and use_tools:
						use_tools = False
						use_tool_choice = False
						continue
					if "reasoning" in body and use_reasoning:
						use_reasoning = False
						continue
				raise

	async def chat(
		self,
		model: str,
		messages: List[Dict[str, Any]],
		tools: List[Dict[str, Any]] | None = None,
	) -> AsyncIterator[LLMChunk]:
		openai_messages = self._normalize_messages(messages)
		stream = await self._create_stream(model, openai_messages, tools)

		accumulated_tool_calls: List[Dict[str, Any]] = []
		accumulated_reasoning: List[str] = []
		accumulated_reasoning_details: List[Dict[str, Any]] = []

		async for chunk in stream:
			if not chunk.choices:
				if chunk.usage:
					yield LLMChunk(usage_tokens=chunk.usage.total_tokens)
				continue
			delta = chunk.choices[0].delta
			reasoning = getattr(delta, "reasoning", None) or getattr(delta, "reasoning_text", None) or ""
			if reasoning:
				accumulated_reasoning.append(str(reasoning))

			reasoning_details = getattr(delta, "reasoning_details", None) or []
			if reasoning_details:
				accumulated_reasoning_details.extend(list(reasoning_details))

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

		if accumulated_reasoning or accumulated_reasoning_details:
			yield LLMChunk(
				reasoning="".join(accumulated_reasoning),
				reasoning_details=accumulated_reasoning_details or None,
			)

		if accumulated_tool_calls:
			yield LLMChunk(tool_calls=accumulated_tool_calls)

	async def list_models(self) -> List[str]:
		result = await self._client.models.list()
		return sorted(m.id for m in result.data)
