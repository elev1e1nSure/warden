from __future__ import annotations

import asyncio
from typing import Any

from agent.memory.store import MemoryStore
from agent.tools.base import Tool


class MemoryTool(Tool):
    name = "memory"
    description = (
        "Read or write persistent memory facts that survive across sessions. "
        "action: get (one key or all), set (key + value), delete (key), list (keys), clear (all). "
        "Use to remember user facts, preferences, and project details between sessions."
    )
    params = {
        "action": {"type": "string", "description": "get | set | delete | list | clear"},
        "key": {
            "type": "string",
            "description": "Note key (required for set/delete, optional for get)",
        },
        "value": {"type": "string", "description": "Note value (required for set)"},
    }

    def __init__(self) -> None:
        self._store = MemoryStore()

    def tool_definition(self) -> dict:
        d = super().tool_definition()
        d["function"]["parameters"]["required"] = ["action"]
        return d

    async def execute(self, args: dict[str, Any]) -> str:
        action = str(args.get("action", "")).strip().lower()
        key = str(args.get("key", "")).strip()
        value = args.get("value")
        try:
            return await asyncio.to_thread(self._run, action, key, value)
        except Exception as e:
            return f"error: {e}"

    def _run(self, action: str, key: str, value: Any) -> str:
        if action == "list":
            entries = self._store.get_entries()
            if not entries:
                return "(empty)"
            return "\n".join(sorted({f"{e['category']}/{e['key']}: {e['value']}" for e in entries}))

        if action == "get":
            entries = self._store.get_entries()
            if not key:
                if not entries:
                    return "(empty)"
                return "\n".join(sorted(f"{e['category']}/{e['key']}: {e['value']}" for e in entries))
            matches = [e for e in entries if key.lower() in e["key"].lower()]
            if not matches:
                return f"(no note for '{key}')"
            return "\n".join(f"{e['category']}/{e['key']}: {e['value']}" for e in matches)

        if action == "set":
            if not key:
                return "error: key is required for set"
            if value is None:
                return "error: value is required for set"
            val = value if isinstance(value, str) else str(value)
            self._store.upsert_entry(
                session_id="tool",
                category="memory",
                key=key,
                value=val,
                confidence=1.0,
            )
            return f"saved: {key}"

        if action == "delete":
            if not key:
                return "error: key is required for delete"
            deleted = self._store.delete_entry(key)
            if deleted == 0:
                return f"(no note for '{key}')"
            return f"deleted: {key}"

        if action == "clear":
            count = self._store.clear_entries()
            return f"cleared {count} notes"

        return "error: action must be get, set, delete, list, or clear"
