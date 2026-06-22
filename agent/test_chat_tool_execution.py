"""Tests for ChatSession._call_llm and _execute_tool_call branches."""

from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

from agent.chat import ChatSession
from agent.confirmations import ConfirmationManager, QuestionManager
from agent.llm_client import LLMChunk, LLMClient
from agent.safety import SafetyDecision

# ── helpers ────────────────────────────────────────────────────────────────────


class FakeClient(LLMClient):
    """LLM client driven by a list of LLMChunk objects."""

    def __init__(self, chunks: list[LLMChunk]):
        self._chunks = chunks

    async def list_models(self) -> list[str]:
        return []

    async def chat(self, model, messages, tools=None):
        for c in self._chunks:
            yield c


def _session(chunks: list[LLMChunk], *, with_managers: bool = True) -> ChatSession:
    client = FakeClient(chunks)
    cm = ConfirmationManager() if with_managers else None
    qm = QuestionManager() if with_managers else None
    return ChatSession(model="test", client=client, confirmation_manager=cm, question_manager=qm)


async def _collect_events(gen) -> list:
    events = []
    async for t, p in gen:
        events.append((t, p))
    return events


# ── _call_llm branches ────────────────────────────────────────────────────────


class TestCallLlm:
    async def test_token_events_emitted(self):
        session = _session([LLMChunk(content="hello world")])
        result = {}
        events = [e async for e in session._call_llm([], result)]
        token_texts = [p for t, p in events if t == "token"]
        assert any("hello" in s for s in token_texts)

    async def test_reasoning_chunk_emits_think(self):
        session = _session([LLMChunk(reasoning="deep thought")])
        result = {}
        events = [e async for e in session._call_llm([], result)]
        think_texts = [p for t, p in events if t == "think"]
        assert "deep thought" in think_texts

    async def test_reasoning_details_text_extracted(self):
        session = _session([LLMChunk(reasoning_details=[{"text": "detail here"}])])
        result = {}
        events = [e async for e in session._call_llm([], result)]
        think_texts = [p for t, p in events if t == "think"]
        assert any("detail" in s for s in think_texts)

    async def test_think_tags_in_content_parsed(self):
        session = _session([LLMChunk(content="before<think>inside</think>after")])
        result = {}
        events = [e async for e in session._call_llm([], result)]
        types = [t for t, _ in events]
        assert "think" in types
        assert "token" in types
        think_texts = [p for t, p in events if t == "think"]
        assert any("inside" in s for s in think_texts)

    async def test_incomplete_think_tag_streams_as_think(self):
        session = _session([LLMChunk(content="start<think>no close tag")])
        result = {}
        events = [e async for e in session._call_llm([], result)]
        think_texts = [p for t, p in events if t == "think"]
        assert any("no close tag" in s for s in think_texts)

    async def test_llm_exception_yields_error_token(self):
        class ErrorClient(LLMClient):
            async def list_models(self) -> list[str]:
                return []

            async def chat(self, model, messages, tools=None):
                raise ConnectionError("socket broke")
                yield  # pragma: no cover — make it async generator

        session = ChatSession(model="test", client=ErrorClient())
        result = {}
        events = [e async for e in session._call_llm([], result)]
        token_texts = [p for t, p in events if t == "token"]
        assert any("connection error" in s for s in token_texts)
        assert result.get("error") is True


# ── _execute_tool_call branches ───────────────────────────────────────────────


def _make_tc(name: str, args: str = "{}", call_id: str = "call_1"):
    tc = MagicMock()
    tc.function.name = name
    tc.function.arguments = args
    tc.id = call_id
    return tc


class TestExecuteToolCall:
    async def test_unknown_tool_records_error(self):
        session = _session([])
        tc = _make_tc("nonexistent_tool_xyz")
        with patch("agent.tool_runner.REGISTRY", {}):
            _ = [e async for e in session._execute_tool_call(tc, auto_mode=False)]
        # unknown tool — no events yielded, but history updated
        assert any("not found" in str(session.history[-1].get("content", "")) for _ in [1])

    async def test_question_tool_no_manager(self):
        session = _session([], with_managers=False)
        tc = _make_tc("question", '{"questions": [{"question": "Q?", "header": "h"}]}')
        fake_qtool = MagicMock()
        fake_qtool.name = "question"
        with patch("agent.tool_runner.REGISTRY", {"question": fake_qtool}):
            _ = [e async for e in session._execute_tool_call(tc, auto_mode=False)]
        assert any("no question manager" in str(e) for e in [session.history[-1]["content"]])

    async def test_safety_blocked_yields_tool_event(self):
        session = _session([])
        tc = _make_tc("file_delete", '{"path": "/evil"}')

        blocked = SafetyDecision(
            risk="blocked",
            reason="dangerous path",
            summary="blocked",
            details=[],
            normalized_args={},
        )
        with patch("agent.tool_runner.assess_tool_call", return_value=blocked):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert len(tool_events) == 1
        assert "blocked" in tool_events[0][1]["result"]

    async def test_safety_confirm_cancelled(self):
        session = _session([])
        tc = _make_tc("file_write", '{"path": "x", "content": "y"}')

        confirm_decision = SafetyDecision(
            risk="confirm",
            reason="file write",
            summary="Modify file",
            details=[],
            normalized_args={},
        )

        async def fake_wait(call_id):
            return False  # user cancelled

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=confirm_decision),
            patch.object(session.confirmation_manager, "wait", side_effect=fake_wait),
        ):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        confirm_events = [e for t, e in events if t == "confirm"]
        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert len(confirm_events) == 1
        assert any("cancelled" in p["result"] for _, p in tool_events)

    async def test_safety_confirm_accepted_executes_tool(self):
        session = _session([])
        tc = _make_tc("file_list", '{"path": "."}')

        confirm_decision = SafetyDecision(
            risk="confirm", reason="file list", summary="List dir", details=[], normalized_args={}
        )

        async def fake_wait(call_id):
            return True  # user approved

        fake_tool = MagicMock()
        fake_tool.execute = AsyncMock(return_value="dir contents")
        fake_tool.name = "file_list"

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=confirm_decision),
            patch.object(session.confirmation_manager, "wait", side_effect=fake_wait),
            patch("agent.tool_runner.REGISTRY", {"file_list": fake_tool}),
        ):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert any("dir contents" in p["result"] for _, p in tool_events)

    async def test_no_confirmation_manager_returns_cancelled(self):
        session = _session([], with_managers=False)
        # manually set no confirmation_manager but valid question_manager not needed here
        session.confirmation_manager = None
        tc = _make_tc("file_write", '{"path": "x", "content": "y"}')

        confirm_decision = SafetyDecision(
            risk="confirm",
            reason="file write",
            summary="Write file",
            details=[],
            normalized_args={},
        )

        with patch("agent.tool_runner.assess_tool_call", return_value=confirm_decision):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert any("cancelled" in p["result"] for _, p in tool_events)

    async def test_tool_timeout_returns_error(self):
        session = _session([])
        tc = _make_tc("powershell", '{"command": "sleep 999"}')

        safe_decision = SafetyDecision(risk="safe", reason="ok", summary="safe", details=[], normalized_args={})

        slow_tool = MagicMock()
        slow_tool.name = "powershell"
        slow_tool.execute = AsyncMock(side_effect=TimeoutError())

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=safe_decision),
            patch("agent.tool_runner.REGISTRY", {"powershell": slow_tool}),
            patch("asyncio.wait_for", side_effect=TimeoutError()),
        ):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert any("timeout" in p["result"] for _, p in tool_events)

    async def test_question_tool_runtime_error_handled(self):
        """QuestionTool.execute() raises RuntimeError — should return error message."""
        session = _session([])
        tc = _make_tc("powershell", '{"command": "x"}')

        safe_decision = SafetyDecision(risk="safe", reason="ok", summary="safe", details=[], normalized_args={})

        error_tool = MagicMock()
        error_tool.name = "powershell"
        error_tool.execute = AsyncMock(
            side_effect=RuntimeError("question tool must be handled by chat loop, not executed directly")
        )

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=safe_decision),
            patch("agent.tool_runner.REGISTRY", {"powershell": error_tool}),
        ):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert any("question tool" in p["result"] for _, p in tool_events)

    async def test_auto_mode_safe_decision_executes_directly(self):
        session = _session([])
        tc = _make_tc("file_list", '{"path": "."}')

        # In auto mode, confirm→safe is handled by assess_tool_call logic
        safe_decision = SafetyDecision(risk="safe", reason="ok", summary="safe", details=[], normalized_args={})

        fake_tool = MagicMock()
        fake_tool.name = "file_list"
        fake_tool.execute = AsyncMock(return_value="files here")

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=safe_decision),
            patch("agent.tool_runner.REGISTRY", {"file_list": fake_tool}),
        ):
            events = [e async for e in session._execute_tool_call(tc, auto_mode=True)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert any("files here" in p["result"] for _, p in tool_events)

    async def test_dict_style_tool_call(self):
        """Tool call as dict (not object with .function attribute)."""
        session = _session([])
        tc_dict = {
            "function": {"name": "file_list", "arguments": '{"path": "."}'},
            "id": "call_dict",
        }

        safe_decision = SafetyDecision(risk="safe", reason="ok", summary="safe", details=[], normalized_args={})
        fake_tool = MagicMock()
        fake_tool.name = "file_list"
        fake_tool.execute = AsyncMock(return_value="result")

        with (
            patch("agent.tool_runner.assess_tool_call", return_value=safe_decision),
            patch("agent.tool_runner.REGISTRY", {"file_list": fake_tool}),
        ):
            events = [e async for e in session._execute_tool_call(tc_dict, auto_mode=False)]

        tool_events = [(t, p) for t, p in events if t == "tool"]
        assert len(tool_events) == 1
