"""Tests for agent/llm_client.py."""
from __future__ import annotations

import os
from typing import AsyncIterator
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from agent.llm_client import LLMChunk, OpenAIClient


# ── _normalize_messages (existing + extended) ─────────────────────────────────

def test_normalize_messages_strips_reasoning_fields() -> None:
	client = OpenAIClient.__new__(OpenAIClient)
	messages = [
		{
			"role": "assistant",
			"content": "done",
			"tool_calls": [
				{
					"id": "call_1",
					"function": {"name": "read", "arguments": "{}"},
				}
			],
			"reasoning": "step by step",
			"reasoning_details": [{"type": "reasoning.text", "text": "step by step"}],
		}
	]

	result = OpenAIClient._normalize_messages(client, messages)

	assert "reasoning" not in result[0]
	assert "reasoning_details" not in result[0]
	assert result[0]["tool_calls"][0]["function"]["name"] == "read"


class TestNormalizeMessages:
	def _client(self):
		with patch("openai.AsyncOpenAI"):
			return OpenAIClient("https://openrouter.ai/api/v1")

	def test_tool_message_with_call_id(self):
		c = self._client()
		msgs = [{"role": "tool", "tool_call_id": "c1", "content": "result", "name": "x"}]
		result = c._normalize_messages(msgs)
		assert result[0]["role"] == "tool"
		assert result[0]["tool_call_id"] == "c1"

	def test_tool_message_no_call_id_generates_fallback(self):
		c = self._client()
		msgs = [{"role": "tool", "content": "result", "name": "mytool"}]
		result = c._normalize_messages(msgs)
		assert "tool_call_id" in result[0]
		assert result[0]["tool_call_id"] != ""

	def test_assistant_with_tool_calls_object(self):
		c = self._client()
		tc = MagicMock()
		tc.function.name = "file_read"
		tc.function.arguments = '{"path": "x"}'
		tc.id = "call_123"
		msgs = [{"role": "assistant", "content": "", "tool_calls": [tc]}]
		result = c._normalize_messages(msgs)
		assert result[0]["role"] == "assistant"
		assert result[0]["tool_calls"][0]["function"]["name"] == "file_read"

	def test_assistant_with_tool_calls_dict(self):
		c = self._client()
		tc = {"function": {"name": "run", "arguments": "{}"}, "id": "c2"}
		msgs = [{"role": "assistant", "content": "", "tool_calls": [tc]}]
		result = c._normalize_messages(msgs)
		assert result[0]["tool_calls"][0]["id"] == "c2"

	def test_assistant_tool_call_no_id_generates_fallback(self):
		c = self._client()
		tc = {"function": {"name": "run", "arguments": "{}"}, "id": ""}
		msgs = [{"role": "assistant", "content": "", "tool_calls": [tc]}]
		result = c._normalize_messages(msgs)
		assert result[0]["tool_calls"][0]["id"].startswith("call_")

	def test_regular_messages_pass_through(self):
		c = self._client()
		msgs = [
			{"role": "system", "content": "You are helpful"},
			{"role": "user", "content": "Hello"},
		]
		result = c._normalize_messages(msgs)
		assert result[0]["role"] == "system"
		assert result[1]["content"] == "Hello"


# ── OpenAIClient constructor ──────────────────────────────────────────────────

class TestOpenAIClientInit:
	def test_uses_openrouter_key(self, monkeypatch):
		monkeypatch.setenv("OPENROUTER_API_KEY", "or-key-123")
		monkeypatch.delenv("OPENAI_API_KEY", raising=False)
		with patch("openai.AsyncOpenAI") as mock_cls:
			OpenAIClient("https://openrouter.ai/api/v1")
		_, kwargs = mock_cls.call_args
		assert kwargs["api_key"] == "or-key-123"

	def test_uses_openai_key(self, monkeypatch):
		monkeypatch.delenv("OPENROUTER_API_KEY", raising=False)
		monkeypatch.setenv("OPENAI_API_KEY", "sk-openai-abc")
		with patch("openai.AsyncOpenAI") as mock_cls:
			OpenAIClient("https://api.openai.com/v1")
		_, kwargs = mock_cls.call_args
		assert kwargs["api_key"] == "sk-openai-abc"

	def test_fallback_key_no_env(self, monkeypatch):
		monkeypatch.delenv("OPENROUTER_API_KEY", raising=False)
		monkeypatch.delenv("OPENAI_API_KEY", raising=False)
		with patch("openai.AsyncOpenAI"):
			OpenAIClient("https://api.example.com/v1")  # should not raise

	def test_is_openrouter_flag(self, monkeypatch):
		monkeypatch.setenv("OPENROUTER_API_KEY", "key")
		with patch("openai.AsyncOpenAI"):
			c = OpenAIClient("https://openrouter.ai/api/v1")
		assert c._is_openrouter is True

	def test_not_openrouter(self, monkeypatch):
		monkeypatch.setenv("OPENAI_API_KEY", "key")
		with patch("openai.AsyncOpenAI"):
			c = OpenAIClient("https://api.openai.com/v1")
		assert c._is_openrouter is False


# ── OpenAIClient.chat ─────────────────────────────────────────────────────────

def _make_delta(content="", reasoning="", tool_calls=None, reasoning_details=None):
	delta = MagicMock()
	delta.content = content
	delta.reasoning = reasoning
	delta.reasoning_text = None
	delta.tool_calls = tool_calls or []
	delta.reasoning_details = reasoning_details or []
	return delta


def _make_chunk(delta):
	chunk = MagicMock()
	chunk.choices = [MagicMock(delta=delta)]
	return chunk


async def _fake_stream(chunks):
	for c in chunks:
		yield c


class TestOpenAIClientChat:
	def _make_client(self, is_openrouter=False):
		with patch("openai.AsyncOpenAI") as mock_cls:
			url = "https://openrouter.ai" if is_openrouter else "https://api.openai.com"
			c = OpenAIClient(url)
			mock_instance = mock_cls.return_value
			return c, mock_instance

	async def test_content_chunks_yielded(self):
		c, mock_instance = self._make_client()
		chunks = [
			_make_chunk(_make_delta(content="hello ")),
			_make_chunk(_make_delta(content="world")),
		]
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream(chunks))
		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)
		contents = [r.content for r in results if r.content]
		assert "hello " in contents
		assert "world" in contents

	async def test_reasoning_accumulated_and_yielded(self):
		c, mock_instance = self._make_client()
		chunks = [
			_make_chunk(_make_delta(reasoning="part1")),
			_make_chunk(_make_delta(reasoning="part2")),
		]
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream(chunks))
		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)
		reasoning_chunks = [r for r in results if r.reasoning]
		assert len(reasoning_chunks) == 1
		assert reasoning_chunks[0].reasoning == "part1part2"

	async def test_tool_calls_accumulated_and_yielded(self):
		c, mock_instance = self._make_client()

		tc1 = MagicMock()
		tc1.index = 0
		tc1.function.name = "file_read"
		tc1.function.arguments = '{"pa'

		tc2 = MagicMock()
		tc2.index = 0
		tc2.function.name = ""
		tc2.function.arguments = 'th": "x"}'

		chunks = [
			_make_chunk(_make_delta(tool_calls=[tc1])),
			_make_chunk(_make_delta(tool_calls=[tc2])),
		]
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream(chunks))
		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)
		tc_chunks = [r for r in results if r.tool_calls]
		assert len(tc_chunks) == 1
		assert tc_chunks[0].tool_calls[0]["function"]["name"] == "file_read"
		assert tc_chunks[0].tool_calls[0]["function"]["arguments"] == '{"path": "x"}'

	async def test_openrouter_sends_reasoning_body(self):
		c, mock_instance = self._make_client(is_openrouter=True)
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream([]))
		async for _ in c.chat("model", []):
			pass
		call_kwargs = mock_instance.chat.completions.create.call_args[1]
		assert "extra_body" in call_kwargs
		assert call_kwargs["extra_body"]["reasoning"]["enabled"] is True

	async def test_non_openrouter_no_extra_body(self):
		c, mock_instance = self._make_client(is_openrouter=False)
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream([]))
		async for _ in c.chat("model", []):
			pass
		call_kwargs = mock_instance.chat.completions.create.call_args[1]
		assert "extra_body" not in call_kwargs

	async def test_reasoning_details_accumulated(self):
		c, mock_instance = self._make_client()
		detail = {"text": "detail text"}
		chunks = [_make_chunk(_make_delta(reasoning_details=[detail]))]
		mock_instance.chat.completions.create = AsyncMock(return_value=_fake_stream(chunks))
		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)
		rd_chunks = [r for r in results if r.reasoning_details]
		assert len(rd_chunks) == 1
		assert rd_chunks[0].reasoning_details == [detail]


# ── OllamaClient.chat ─────────────────────────────────────────────────────────

class TestOllamaClientChat:
	def _make_client(self):
		with patch("ollama.AsyncClient") as mock_cls:
			from agent.llm_client import OllamaClient
			c = OllamaClient()
			return c, mock_cls.return_value

	async def test_yields_content_and_thinking(self):
		c, mock_instance = self._make_client()

		msg = MagicMock()
		msg.thinking = "a thought"
		msg.content = "hello"
		msg.tool_calls = None

		fake_chunk = MagicMock()
		fake_chunk.message = msg

		# OllamaClient does `await self._client.chat(...)` then iterates result.
		# So mock must be a coroutine that returns an async iterable.
		async def fake_chat(*a, **kw):
			async def gen():
				yield fake_chunk
			return gen()

		mock_instance.chat = fake_chat

		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)

		assert results[0].thinking == "a thought"
		assert results[0].content == "hello"

	async def test_tool_calls_propagated(self):
		c, mock_instance = self._make_client()

		msg = MagicMock()
		msg.thinking = ""
		msg.content = ""
		msg.tool_calls = [{"name": "file_read"}]

		fake_chunk = MagicMock()
		fake_chunk.message = msg

		async def fake_chat(*a, **kw):
			async def gen():
				yield fake_chunk
			return gen()

		mock_instance.chat = fake_chat

		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)

		assert results[0].tool_calls == [{"name": "file_read"}]

	async def test_empty_stream(self):
		c, mock_instance = self._make_client()

		async def fake_chat(*a, **kw):
			async def gen():
				return
				yield  # pragma: no cover
			return gen()

		mock_instance.chat = fake_chat

		results = []
		async for chunk in c.chat("model", []):
			results.append(chunk)
		assert results == []
