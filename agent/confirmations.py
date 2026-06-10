import asyncio
import time
import uuid
from typing import Dict

_TIMEOUT_SECONDS = 300  # 5 minutes


class ConfirmationManager:
	"""Holds pending user confirmations for dangerous tool calls.

	Features:
	- Full UUID ids (no short collisions)
	- created_at + auto-cancel on timeout
	- cancel_all for /reset or disconnect
	- Protection against duplicate resolves
	"""

	def __init__(self) -> None:
		self._pending: Dict[str, dict] = {}

	def register(self) -> tuple[str, asyncio.Event]:
		call_id = str(uuid.uuid4())
		event = asyncio.Event()
		self._pending[call_id] = {
			"event": event,
			"ok": False,
			"created_at": time.time(),
			"resolved": False,
		}
		return call_id, event

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

	def cancel_all(self) -> None:
		for call_id, entry in list(self._pending.items()):
			self._cancel_entry(call_id)

	async def wait(self, call_id: str) -> bool:
		"""Wait for confirmation with timeout. Returns True if confirmed, False if cancelled/timed out."""
		entry = self._pending.get(call_id)
		if entry is None:
			return False
		try:
			await asyncio.wait_for(entry["event"].wait(), timeout=_TIMEOUT_SECONDS)
		except asyncio.TimeoutError:
			self._cancel_entry(call_id)
		resolved = self._pending.pop(call_id, None)
		return bool(resolved and resolved.get("ok", False))

	def active_count(self) -> int:
		now = time.time()
		expired = [
			cid for cid, e in self._pending.items()
			if self._is_expired(e)
		]
		for cid in expired:
			self._cancel_entry(cid)
		return len(self._pending)

	def _is_expired(self, entry: dict) -> bool:
		created = entry.get("created_at", 0)
		return time.time() - created > _TIMEOUT_SECONDS

	def _cancel_entry(self, call_id: str) -> None:
		entry = self._pending.pop(call_id, None)
		if entry:
			entry["ok"] = False
			entry["resolved"] = True
			entry["event"].set()


class QuestionManager:
	"""Holds pending user question prompts.

	Similar to ConfirmationManager but stores the questions
	and allows resolving with answer values instead of a bool.
	"""

	def __init__(self) -> None:
		self._pending: Dict[str, dict] = {}

	def register(self, questions: list) -> tuple[str, asyncio.Event]:
		call_id = str(uuid.uuid4())
		event = asyncio.Event()
		self._pending[call_id] = {
			"event": event,
			"questions": questions,
			"answers": None,
			"created_at": time.time(),
			"resolved": False,
		}
		return call_id, event

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

	def cancel_all(self) -> None:
		for call_id, entry in list(self._pending.items()):
			self._cancel_entry(call_id)

	async def wait(self, call_id: str) -> list[list[str]] | None:
		entry = self._pending.get(call_id)
		if entry is None:
			return None
		try:
			await asyncio.wait_for(entry["event"].wait(), timeout=_TIMEOUT_SECONDS)
		except asyncio.TimeoutError:
			self._cancel_entry(call_id)
		resolved = self._pending.pop(call_id, None)
		return resolved.get("answers") if resolved else None

	def pending_count(self) -> int:
		now = time.time()
		return sum(
			1 for e in self._pending.values()
			if not self._is_expired(e)
		)

	def _is_expired(self, entry: dict) -> bool:
		created = entry.get("created_at", 0)
		return time.time() - created > _TIMEOUT_SECONDS

	def _cancel_entry(self, call_id: str) -> None:
		entry = self._pending.pop(call_id, None)
		if entry:
			entry["answers"] = []
			entry["resolved"] = True
			entry["event"].set()
