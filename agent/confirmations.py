import asyncio
import time
import uuid
from typing import Dict

_TIMEOUT_SECONDS = 300  # 5 minutes


class _PendingManager:
    """Shared base: UUID registration, timeout, cancel_all."""

    def __init__(self) -> None:
        self._pending: Dict[str, dict] = {}

    def cancel_all(self) -> None:
        for call_id in list(self._pending):
            self._cancel_entry(call_id)

    def _is_expired(self, entry: dict) -> bool:
        return time.time() - entry.get("created_at", 0) > _TIMEOUT_SECONDS

    def _cancel_entry(self, call_id: str) -> None:
        entry = self._pending.pop(call_id, None)
        if entry:
            self._set_cancelled_defaults(entry)
            entry["resolved"] = True
            entry["event"].set()

    def _set_cancelled_defaults(self, entry: dict) -> None:
        """Subclasses set the appropriate default for a cancelled entry."""

    def _make_entry(self, **extra) -> tuple[str, asyncio.Event]:
        call_id = str(uuid.uuid4())
        event = asyncio.Event()
        self._pending[call_id] = {
            "event": event,
            "created_at": time.time(),
            "resolved": False,
            **extra,
        }
        return call_id, event

    async def _wait_for(self, call_id: str) -> dict | None:
        entry = self._pending.get(call_id)
        if entry is None:
            return None
        try:
            await asyncio.wait_for(entry["event"].wait(), timeout=_TIMEOUT_SECONDS)
        except asyncio.TimeoutError:
            self._cancel_entry(call_id)
        return self._pending.pop(call_id, None)

    def _active_count(self) -> int:
        expired = [cid for cid, e in self._pending.items() if self._is_expired(e)]
        for cid in expired:
            self._cancel_entry(cid)
        return len(self._pending)


class ConfirmationManager(_PendingManager):
    """Holds pending user confirmations for dangerous tool calls."""

    def _set_cancelled_defaults(self, entry: dict) -> None:
        entry["ok"] = False

    def register(self) -> tuple[str, asyncio.Event]:
        return self._make_entry(ok=False)

    def resolve(self, call_id: str, ok: bool) -> bool:
        entry = self._pending.get(call_id)
        if entry and not entry.get("resolved", False):
            entry["ok"] = ok
            entry["resolved"] = True
            entry["event"].set()
            return True
        return False

    def get(self, call_id: str) -> dict | None:
        entry = self._pending.get(call_id)
        if entry is not None and self._is_expired(entry):
            self._cancel_entry(call_id)
            return None
        return entry

    def pop(self, call_id: str) -> dict | None:
        entry = self._pending.pop(call_id, None)
        if entry is not None and self._is_expired(entry):
            self._cancel_entry(call_id)
            return None
        return entry

    async def wait(self, call_id: str) -> bool:
        resolved = await self._wait_for(call_id)
        return bool(resolved and resolved.get("ok", False))

    def active_count(self) -> int:
        return self._active_count()


class QuestionManager(_PendingManager):
    """Holds pending user question prompts."""

    def _set_cancelled_defaults(self, entry: dict) -> None:
        entry["answers"] = []

    def register(self, questions: list) -> tuple[str, asyncio.Event]:
        return self._make_entry(questions=questions, answers=None)

    def resolve(self, call_id: str, answers: list[list[str]] | None) -> bool:
        entry = self._pending.get(call_id)
        if entry and not entry.get("resolved", False):
            entry["answers"] = answers or []
            entry["resolved"] = True
            entry["event"].set()
            return True
        return False

    def pop(self, call_id: str) -> dict | None:
        entry = self._pending.pop(call_id, None)
        if entry is not None and self._is_expired(entry):
            answers = entry.get("answers")
            self._cancel_entry(call_id)
            return {"answers": answers} if answers else None
        return entry

    async def wait(self, call_id: str) -> list[list[str]] | None:
        resolved = await self._wait_for(call_id)
        return resolved.get("answers") if resolved else None

    def pending_count(self) -> int:
        return self._active_count()
