"""Characterization tests for ChatSession behavior."""

from typing import Any, Dict, List
import asyncio

import pytest

from agent.chat import ChatSession
from agent.llm_client import LLMClient, LLMChunk


class FakeLLMClient(LLMClient):
    """Records messages and yields controlled chunks."""

    def __init__(self, chunks: List[LLMChunk] | None = None) -> None:
        self.calls: List[List[Dict[str, Any]]] = []
        self._chunks = chunks or []

    async def list_models(self) -> List[str]:
        return []

    async def chat(
        self,
        model: str,
        messages: List[Dict[str, Any]],
        tools: List[Dict[str, Any]] | None = None,
    ):
        self.calls.append(messages)
        for chunk in self._chunks:
            yield chunk


class TestChatSessionHistory:
    def test_add_user_appends_to_history(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_user("hello")
        assert len(session.history) == 1
        assert session.history[0]["role"] == "user"
        assert session.history[0]["content"] == "hello"

    def test_add_assistant_with_tool_calls(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_assistant("done", tool_calls=[{"id": "1"}])
        assert session.history[0]["role"] == "assistant"
        assert session.history[0]["tool_calls"] == [{"id": "1"}]

    def test_add_tool_result(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_tool_result("read", "content", tool_call_id="call_1")
        assert session.history[0]["role"] == "tool"
        assert session.history[0]["content"] == "content"
        assert session.history[0]["tool_call_id"] == "call_1"

    def test_reset_clears_history_and_tokens(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_user("hello")
        session.token_count = 42
        session.reset()
        assert session.history == []
        assert session.token_count == 0

    def test_token_estimate_scales_with_content(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_user("a" * 400)
        est = session._estimate_tokens()
        assert est == 100  # 400 // 4

    def test_token_estimate_non_negative(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        assert session._estimate_tokens() == 0


class TestChatSessionCompact:
    @pytest.mark.asyncio
    async def test_compact_returns_early_for_short_history(self) -> None:
        session = ChatSession("test", FakeLLMClient())
        session.add_user("hi")
        result = await session.compact()
        assert result["summary"] == "nothing to compact"
        assert result["tokens_before"] == session.token_count

    @pytest.mark.asyncio
    async def test_compact_replaces_history(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(content="summary text")])
        session = ChatSession("test", fake)
        session.add_user("question 1")
        session.add_assistant("answer 1")
        result = await session.compact()
        assert result["summary"] == "summary text"
        assert len(session.history) == 2
        assert session.history[0]["content"] == "[Conversation summary]"
        assert session.history[1]["content"] == "summary text"

    @pytest.mark.asyncio
    async def test_compact_on_error_preserves_history(self) -> None:
        class BrokenLLM(LLMClient):
            async def list_models(self) -> List[str]:
                return []

            async def chat(self, model, messages, tools=None):
                raise RuntimeError("network down")
                yield  # type: ignore[unreachable]

        session = ChatSession("test", BrokenLLM())
        session.add_user("q1")
        session.add_assistant("a1")
        orig_len = len(session.history)
        result = await session.compact()
        assert "error" in result["summary"]
        assert len(session.history) == orig_len


class TestChatSessionStream:
    @pytest.mark.asyncio
    async def test_stream_yields_warden_start(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(content="hi")])
        session = ChatSession("test", fake)
        events = []
        async for ev in session.stream("hello"):
            events.append(ev)
            break  # just first event
        assert events[0] == ("warden_start", {})

    @pytest.mark.asyncio
    async def test_stream_adds_user_message(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(content="response")])
        session = ChatSession("test", fake)
        async for _ in session.stream("hello"):
            pass
        assert session.history[0]["role"] == "user"
        assert session.history[0]["content"] == "hello"

    @pytest.mark.asyncio
    async def test_stream_adds_assistant_message(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(content="response")])
        session = ChatSession("test", fake)
        async for _ in session.stream("hello"):
            pass
        assert session.history[-1]["role"] == "assistant"

    @pytest.mark.asyncio
    async def test_stream_think_content_not_in_history(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(thinking="deep thought")])
        session = ChatSession("test", fake)
        session.thinking_enabled = True
        events = []
        async for ev in session.stream("hello"):
            events.append(ev)
        think_events = [e for e in events if e[0] == "think"]
        assert len(think_events) == 1
        assert think_events[0][1] == "deep thought"

    @pytest.mark.asyncio
    async def test_stream_skips_think_when_disabled(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(thinking="deep thought")])
        session = ChatSession("test", fake)
        session.thinking_enabled = False
        events = []
        async for ev in session.stream("hello"):
            events.append(ev)
        think_events = [e for e in events if e[0] == "think"]
        assert len(think_events) == 0

    @pytest.mark.asyncio
    async def test_stream_token_count_updated(self) -> None:
        fake = FakeLLMClient(chunks=[LLMChunk(content="hello world")])
        session = ChatSession("test", fake)
        orig_count = session.token_count
        async for _ in session.stream("hi"):
            pass
        assert session.token_count > orig_count

    @pytest.mark.asyncio
    async def test_stream_handles_errors_gracefully(self) -> None:
        class BrokenLLM(LLMClient):
            async def list_models(self) -> List[str]:
                return []

            async def chat(self, model, messages, tools=None):
                raise RuntimeError("boom")
                yield  # type: ignore[unreachable]

        session = ChatSession("test", BrokenLLM())
        events = []
        async for ev in session.stream("hello"):
            events.append(ev)
        error_events = [e for e in events if "error" in str(e[1])]
        assert len(error_events) >= 1
