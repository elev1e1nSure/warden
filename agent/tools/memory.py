from __future__ import annotations

import asyncio
import json
import os
import tempfile
from pathlib import Path
from typing import Any, Dict


from agent.tools.base import Tool


def _memory_path() -> Path:
	"""Location of the long-term memory store (overridable via WARDEN_MEMORY_PATH)."""
	override = os.environ.get("WARDEN_MEMORY_PATH")
	if override:
		return Path(override)
	return Path.home() / ".warden" / "memory.json"


def _load(path: Path) -> dict:
	try:
		raw = path.read_text(encoding="utf-8")
	except FileNotFoundError:
		return {}
	except OSError:
		return {}
	try:
		data = json.loads(raw)
	except json.JSONDecodeError:
		return {}
	return data if isinstance(data, dict) else {}


def _save(path: Path, data: dict) -> None:
	path.parent.mkdir(parents=True, exist_ok=True)
	# atomic write so a crash mid-write can't corrupt the store
	fd, tmp = tempfile.mkstemp(dir=str(path.parent), suffix=".tmp")
	try:
		with os.fdopen(fd, "w", encoding="utf-8") as f:
			json.dump(data, f, ensure_ascii=False, indent=2)
		os.replace(tmp, path)
	finally:
		if os.path.exists(tmp):
			os.unlink(tmp)


class MemoryTool(Tool):
	name = "memory"
	description = (
		"Long-term key/value notes that persist across sessions "
		"(stored in ~/.warden/memory.json). "
		"action: get (one key or all), set (key + value), delete (key), list (keys), clear (all). "
		"Use to remember user facts, preferences, and project details between sessions."
	)
	params = {
		"action": {"type": "string", "description": "get | set | delete | list | clear"},
		"key": {"type": "string", "description": "Note key (required for set/delete, optional for get)"},
		"value": {"type": "string", "description": "Note value (required for set)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = ["action"]
		return d

	async def execute(self, args: Dict[str, Any]) -> str:
		action = str(args.get("action", "")).strip().lower()
		key = str(args.get("key", "")).strip()
		path = _memory_path()
		try:
			return await asyncio.to_thread(self._run, action, key, args.get("value"), path)
		except Exception as e:
			return f"error: {e}"

	def _run(self, action: str, key: str, value: Any, path: Path) -> str:
		data = _load(path)
		if action == "list":
			if not data:
				return "(empty)"
			return "\n".join(sorted(data.keys()))
		if action == "get":
			if not key:
				if not data:
					return "(empty)"
				return "\n".join(f"{k}: {v}" for k, v in sorted(data.items()))
			if key not in data:
				return f"(no note for '{key}')"
			return f"{key}: {data[key]}"
		if action == "set":
			if not key:
				return "error: key is required for set"
			if value is None:
				return "error: value is required for set"
			data[key] = value if isinstance(value, str) else json.dumps(value, ensure_ascii=False)
			_save(path, data)
			return f"saved: {key}"
		if action == "delete":
			if not key:
				return "error: key is required for delete"
			if key not in data:
				return f"(no note for '{key}')"
			del data[key]
			_save(path, data)
			return f"deleted: {key}"
		if action == "clear":
			_save(path, {})
			return "cleared all notes"
		return "error: action must be get, set, delete, list, or clear"
