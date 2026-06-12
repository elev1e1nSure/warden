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
		"""Aggregate and persist snapshot for a session, merging with prior long-term memory."""
		snapshot = cls.aggregate(store, session_id)
		prev = store.get_latest_snapshot()
		if prev:
			snapshot = cls._merge_snapshots(prev, snapshot)
		store.save_snapshot(session_id, snapshot)
		store.clear_entries(session_id)

	@staticmethod
	def _merge_snapshots(prev: dict[str, Any], curr: dict[str, Any]) -> dict[str, Any]:
		"""Merge two snapshots, letting current values win on key conflicts."""
		result: dict[str, Any] = {**prev, **curr}
		result["updated_at"] = curr.get("updated_at", datetime.now(timezone.utc).isoformat())

		# Merge simple dict fields
		result["user"] = {**prev.get("user", {}), **curr.get("user", {})}
		result["preferences"] = {**prev.get("preferences", {}), **curr.get("preferences", {})}

		# Merge projects by name
		prev_projects = {p["name"]: p for p in prev.get("projects", []) if "name" in p}
		curr_projects = {p["name"]: p for p in curr.get("projects", []) if "name" in p}
		merged_projects = {**prev_projects, **curr_projects}
		result["projects"] = list(merged_projects.values())

		# Merge tech_stack sets
		prev_ts = set(prev.get("tech_stack", []))
		curr_ts = set(curr.get("tech_stack", []))
		for p in prev.get("projects", []):
			prev_ts.update(p.get("tech_stack", []))
		for p in curr.get("projects", []):
			curr_ts.update(p.get("tech_stack", []))
		all_ts = prev_ts | curr_ts
		if all_ts:
			result["tech_stack"] = sorted(all_ts)
		elif "tech_stack" in result:
			del result["tech_stack"]

		return result
