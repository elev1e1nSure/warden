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
