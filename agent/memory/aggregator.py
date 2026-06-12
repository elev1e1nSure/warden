from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from agent.memory.store import MemoryStore


class MemoryAggregator:
	"""Builds a structured memory snapshot from session entries."""

	@classmethod
	def aggregate(cls, store: MemoryStore, session_id: str) -> dict[str, Any]:
		entries = store.get_entries(session_id=session_id)
		result: dict[str, Any] = {
			"user": {},
			"projects": [],
			"preferences": {},
			"updated_at": datetime.now(timezone.utc).isoformat(),
		}
		projects: dict[str, dict[str, Any]] = {}
		tech_stack: set[str] = set()

		for e in entries:
			cat = e["category"]
			key = e["key"]
			val = e["value"]
			if cat == "user":
				result["user"][key] = val
			elif cat == "preference":
				result["preferences"][key] = val
			elif cat == "project":
				if key not in projects:
					projects[key] = {"name": val}
				else:
					projects[key]["name"] = val
			elif cat == "tech_stack":
				tech_stack.add(val)

		# Attach tech_stack to the most recently mentioned project, or keep global
		project_list = list(projects.values())
		if project_list and tech_stack:
			project_list[-1]["tech_stack"] = sorted(tech_stack)
		elif tech_stack:
			result["tech_stack"] = sorted(tech_stack)

		result["projects"] = project_list
		return result

	@classmethod
	def finalize(cls, store: MemoryStore, session_id: str) -> None:
		"""Aggregate and persist snapshot for a session."""
		snapshot = cls.aggregate(store, session_id)
		store.save_snapshot(session_id, snapshot)
