import asyncio
import uuid
from typing import Dict


class ConfirmationManager:
	"""Holds pending user confirmations for dangerous tool calls."""

	def __init__(self) -> None:
		self._pending: Dict[str, dict] = {}

	def register(self) -> tuple[str, asyncio.Event]:
		call_id = uuid.uuid4().hex[:8]
		event = asyncio.Event()
		self._pending[call_id] = {"event": event, "ok": False}
		return call_id, event

	def resolve(self, call_id: str, ok: bool) -> bool:
		entry = self._pending.get(call_id)
		if entry:
			entry["ok"] = ok
			entry["event"].set()
			return True
		return False

	def get(self, call_id: str) -> dict | None:
		return self._pending.get(call_id)

	def pop(self, call_id: str) -> dict | None:
		return self._pending.pop(call_id, None)
