import asyncio
import json
import re
import uuid
from collections.abc import AsyncIterator
from typing import Any

from agent.confirmations import ConfirmationManager, QuestionManager
from agent.llm_client import LLMClient
from agent.memory.aggregator import MemoryAggregator
from agent.memory.extractor import MemoryExtractor
from agent.memory.store import MemoryStore
from agent.prompt import build_system
from agent.skills import Skill, find_skill, wrap_skill_content
from agent.tool_runner import execute_tool_call
from agent.tools import REGISTRY
from agent.tools.misc import _TODO_STORE

_EMOJI_RE = re.compile(
    "[\U0001f1e6-\U0001f1ff\U0001f300-\U0001faff\U00002700-\U000027bf\U00002600-\U000026ff]+",
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
    note = " [note: attached image not sent — model cannot view images]"
    result = []
    for msg in messages:
        if msg.get("images"):
            m = {k: v for k, v in msg.items() if k != "images"}
            m["content"] = str(msg.get("content", "")) + note
            result.append(m)
        elif isinstance(msg.get("content"), list):
            filtered = [p for p in msg["content"] if not (isinstance(p, dict) and p.get("type") == "image_url")]
            text_parts = " ".join(
                p.get("text", "") for p in filtered if isinstance(p, dict) and p.get("type") == "text"
            ).strip()
            m = dict(msg)
            m["content"] = (text_parts or str(msg.get("content", ""))) + note
            result.append(m)
        else:
            result.append(msg)
    return result


def _is_vision_error(e: Exception) -> bool:
    s = str(e).lower()
    return any(
        kw in s
        for kw in (
            "image",
            "vision",
            "multimodal",
            "does not support",
            "unsupported content",
            "image_url",
            "not support image",
        )
    )


_CONTEXT_LIMITS: dict[str, int] = {
    # Anthropic
    "claude-3.5": 200000,
    "claude-3.7": 200000,
    "claude-4": 200000,
    "claude-opus": 200000,
    "claude-sonnet": 200000,
    "claude-haiku": 200000,
    # OpenAI
    "gpt-4o": 128000,
    "gpt-4.1": 1000000,
    "gpt-4.5": 128000,
    "gpt-4-turbo": 128000,
    "gpt-4": 8192,
    "gpt-3.5-turbo": 16385,
    "o1": 200000,
    "o3": 200000,
    "o4-mini": 200000,
    # Google
    "gemini-2.5": 1048576,
    "gemini-2.0": 1048576,
    "gemini-1.5": 2097152,
    # DeepSeek
    "deepseek-v3": 65536,
    "deepseek-r1": 65536,
    "deepseek-chat": 65536,
    "deepseek-reasoner": 65536,
    # Meta
    "llama-3.1-405b": 131072,
    "llama-3.2": 131072,
    "llama-3": 8192,
    # Mistral
    "mistral-large": 131072,
    "mistral-medium": 32768,
    "mistral-small": 32768,
    "mixtral": 32768,
    # Qwen
    "qwen-2.5": 131072,
    "qwen-2": 131072,
    "qwen-max": 131072,
    "qwq": 131072,
}

_CONTEXT_LIMIT_FALLBACK = 65536


def _guess_context_limit(model: str) -> int:
    lower = model.lower()
    for prefix, limit in _CONTEXT_LIMITS.items():
        if prefix in lower:
            return limit
    if "128k" in lower or "128000" in lower:
        return 128000
    if "64k" in lower or "65536" in lower:
        return 65536
    if "32k" in lower or "32768" in lower:
        return 32768
    if "8k" in lower or "8192" in lower:
        return 8192
    if "4k" in lower or "4096" in lower:
        return 4096
    return _CONTEXT_LIMIT_FALLBACK


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
    def __init__(
        self,
        model: str,
        client: LLMClient,
        confirmation_manager: ConfirmationManager | None = None,
        question_manager: QuestionManager | None = None,
        memory_store: MemoryStore | None = None,
    ) -> None:
        self.model = model
        self.history: list[dict[str, Any]] = []
        self._client = client
        self.confirmation_manager = confirmation_manager
        self.question_manager = question_manager
        self.memory_store = memory_store
        self.session_id: str = str(uuid.uuid4())
        self._extractor = MemoryExtractor()
        self.token_count: int = 0
        self.token_limit: int = _guess_context_limit(model)
        self._cancelled = asyncio.Event()

    def cancel(self) -> None:
        self._cancelled.set()

    def reset_cancellation(self) -> None:
        self._cancelled.clear()

    def is_cancelled(self) -> bool:
        return self._cancelled.is_set()

    def reset(self) -> None:
        if self.memory_store is not None:
            MemoryAggregator.finalize(self.memory_store, self.session_id)
        _TODO_STORE.pop(self.session_id, None)
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
            return {
                "summary": "nothing to compact",
                "tokens_before": self.token_count,
                "tokens_after": self.token_count,
            }

        tokens_before = self._estimate_tokens()
        system = build_system(self.model)
        messages = (
            [{"role": "system", "content": system}] + self.history + [{"role": "user", "content": _COMPACT_PROMPT}]
        )

        summary = ""
        try:
            async for chunk in self._client.chat(model=self.model, messages=messages):
                if chunk.content:
                    summary += chunk.content
        except Exception as e:
            return {
                "summary": f"error: {e}",
                "tokens_before": tokens_before,
                "tokens_after": tokens_before,
            }

        tail = []
        for msg in reversed(self.history):
            if msg.get("role") == "assistant" and msg.get("tool_calls"):
                pending_ids = {tc.get("id", "") for tc in msg["tool_calls"]}
                resolved_ids = set()
                for later in self.history[self.history.index(msg) + 1:]:
                    if later.get("role") == "tool" and later.get("tool_call_id") in pending_ids:
                        resolved_ids.add(later.get("tool_call_id"))
                unresolved = pending_ids - resolved_ids
                if unresolved:
                    tail = self.history[self.history.index(msg):]
                break
            if msg.get("role") == "tool":
                continue
            break

        self.history = [
            {"role": "user", "content": "[Conversation summary]"},
            {"role": "assistant", "content": summary},
        ] + tail
        self.token_count = self._estimate_tokens()
        return {
            "summary": summary,
            "tokens_before": tokens_before,
            "tokens_after": self.token_count,
        }

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
        msg: dict[str, Any] = {"role": "assistant", "content": text}
        if tool_calls:
            msg["tool_calls"] = tool_calls
        # Do NOT persist reasoning/reasoning_details in history.
        # Anthropic rejects thinking blocks with invalid signatures
        # when they are round-tripped through JSON serialization.
        self.history.append(msg)

    def add_tool_result(self, tool_name: str, result: str, tool_call_id: str = "") -> None:
        entry: dict[str, Any] = {"role": "tool", "content": result, "name": tool_name}
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
                if self.is_cancelled():
                    break
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
                            text_chunk = text_chunk[idx + 7 :]
                            in_think = True
                    else:
                        idx = text_chunk.find("</think>")
                        if idx == -1:
                            yield ("think", text_chunk)
                            text_chunk = ""
                        else:
                            if idx > 0:
                                yield ("think", text_chunk[:idx])
                            text_chunk = text_chunk[idx + 8 :]
                            in_think = False
        except Exception as e:
            if _is_vision_error(e) and _has_images(messages):
                stripped = _strip_images(messages)
                try:
                    async for chunk in self._client.chat(
                        model=self.model,
                        messages=stripped,
                        tools=_TOOLS,
                    ):
                        if self.is_cancelled():
                            break
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
        self.reset_cancellation()
        iter_count = 0

        memory_tool = REGISTRY.get("memory")
        if memory_tool is not None:
            memory_tool.current_session_id = self.session_id

        while iter_count < MAX_ITER:
            iter_count += 1
            if self.is_cancelled():
                break
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
                if self.is_cancelled():
                    break
                yield event

            if self.is_cancelled() or llm_result.get("error"):
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
                if self.is_cancelled():
                    break
                async for event in self._execute_tool_call(tc, auto_mode):
                    yield event

        if self.is_cancelled():
            yield ("token", "\n[interrupted]")
        elif iter_count >= MAX_ITER:
            yield ("token", "\n[iteration limit reached]")
