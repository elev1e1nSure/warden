from __future__ import annotations

import json
import os
import sqlite3
from pathlib import Path
from typing import Any


def _db_path() -> Path:
	override = os.environ.get("WARDEN_MEMORY_DB")
	if override:
		return Path(override)
	return Path.home() / ".warden" / "memory.db"


class MemoryStore:
	"""SQLite-backed persistent memory layer."""

	def __init__(self, db_path: Path | None = None) -> None:
		self.db_path = db_path or _db_path()
		self.db_path.parent.mkdir(parents=True, exist_ok=True)
		self._init_db()

	def _conn(self) -> sqlite3.Connection:
		return sqlite3.connect(str(self.db_path))

	def _init_db(self) -> None:
		with self._conn() as conn:
			conn.execute(
				"""
				CREATE TABLE IF NOT EXISTS memory_state (
					key TEXT PRIMARY KEY,
					value TEXT NOT NULL,
					updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
				)
				"""
			)
			conn.execute(
				"""
				CREATE TABLE IF NOT EXISTS memory_entries (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					session_id TEXT NOT NULL,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
					category TEXT NOT NULL,
					key TEXT NOT NULL,
					value TEXT NOT NULL,
					confidence REAL NOT NULL DEFAULT 1.0,
					source_message_id TEXT
				)
				"""
			)
			conn.execute(
				"""
				CREATE UNIQUE INDEX IF NOT EXISTS idx_entries_cat_key
				ON memory_entries(category, key)
				"""
			)
			conn.execute(
				"""
				CREATE TABLE IF NOT EXISTS memory_snapshots (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					session_id TEXT NOT NULL,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
					data TEXT NOT NULL
				)
				"""
			)
			conn.commit()

	def set_enabled(self, enabled: bool) -> None:
		with self._conn() as conn:
			conn.execute(
				"""
				INSERT INTO memory_state(key, value, updated_at)
				VALUES ('enabled', ?, CURRENT_TIMESTAMP)
				ON CONFLICT(key) DO UPDATE SET
					value = excluded.value,
					updated_at = CURRENT_TIMESTAMP
				""",
				("1" if enabled else "0",),
			)
			conn.commit()

	def get_enabled(self) -> bool:
		with self._conn() as conn:
			row = conn.execute(
				"SELECT value FROM memory_state WHERE key = 'enabled'"
			).fetchone()
			return row is not None and row[0] == "1"

	def upsert_entry(
		self,
		session_id: str,
		category: str,
		key: str,
		value: str,
		confidence: float = 1.0,
		source_message_id: str | None = None,
	) -> None:
		with self._conn() as conn:
			conn.execute(
				"""
				INSERT INTO memory_entries
				(session_id, category, key, value, confidence, source_message_id)
				VALUES (?, ?, ?, ?, ?, ?)
				ON CONFLICT(category, key) DO UPDATE SET
					session_id = excluded.session_id,
					value = excluded.value,
					confidence = excluded.confidence,
					source_message_id = excluded.source_message_id,
					timestamp = CURRENT_TIMESTAMP
				""",
				(session_id, category, key, value, confidence, source_message_id),
			)
			conn.commit()

	def get_entries(
		self,
		session_id: str | None = None,
		category: str | None = None,
	) -> list[dict[str, Any]]:
		query = """
			SELECT
				id,
				session_id,
				timestamp,
				category,
				key,
				value,
				confidence,
				source_message_id
			FROM memory_entries
		"""
		conds: list[str] = []
		params: list[Any] = []

		if session_id is not None:
			conds.append("session_id = ?")
			params.append(session_id)
		if category is not None:
			conds.append("category = ?")
			params.append(category)

		if conds:
			query += " WHERE " + " AND ".join(conds)

		query += " ORDER BY timestamp DESC"

		with self._conn() as conn:
			conn.row_factory = sqlite3.Row
			rows = conn.execute(query, params).fetchall()
			return [dict(r) for r in rows]

	def clear_entries(self, session_id: str | None = None) -> int:
		with self._conn() as conn:
			if session_id is not None:
				cur = conn.execute(
					"DELETE FROM memory_entries WHERE session_id = ?",
					(session_id,),
				)
			else:
				cur = conn.execute("DELETE FROM memory_entries")
			conn.commit()
			return cur.rowcount

	def save_snapshot(self, session_id: str, data: dict[str, Any]) -> None:
		with self._conn() as conn:
			conn.execute(
				"INSERT INTO memory_snapshots(session_id, data) VALUES (?, ?)",
				(session_id, json.dumps(data, ensure_ascii=False)),
			)
			conn.commit()

	def get_latest_snapshot(self) -> dict[str, Any] | None:
		with self._conn() as conn:
			row = conn.execute(
				"""
				SELECT data FROM memory_snapshots
				ORDER BY id DESC LIMIT 1
				"""
			).fetchone()
			if row is None:
				return None
			return json.loads(row[0])

	def delete_entry(self, key: str) -> int:
		with self._conn() as conn:
			cur = conn.execute("DELETE FROM memory_entries WHERE key = ?", (key,))
			conn.commit()
			return cur.rowcount

	def get_context_text(
		self,
		session_id: str | None = None,
		min_confidence: float = 0.5,
		max_entries: int = 30,
	) -> str:
		"""Format memory snapshot and current session entries as a context block.

		Starts with the latest long-term snapshot, then overlays entries from
		the current session so newly learned facts are immediately visible.
		"""
		snapshot = self.get_latest_snapshot() or {}
		entries = self.get_entries(session_id=session_id)
		entries = [e for e in entries if e["confidence"] >= min_confidence][:max_entries]

		if not snapshot and not entries:
			return ""

		lines: list[str] = ["[Memory]"]

		# Snapshot (long-term memory)
		user = snapshot.get("user", {})
		for k, v in user.items():
			lines.append(f"- user {k}: {v}")

		projects = snapshot.get("projects", [])
		for p in projects:
			name = p.get("name", "unknown")
			ts = p.get("tech_stack", [])
			if ts:
				lines.append(f"- project {name} (stack: {', '.join(ts)})")
			else:
				lines.append(f"- project: {name}")

		prefs = snapshot.get("preferences", {})
		for k, v in prefs.items():
			lines.append(f"- preference {k}: {v}")

		global_ts = snapshot.get("tech_stack", [])
		if global_ts:
			lines.append(f"- tech stack: {', '.join(global_ts)}")

		# Current session overrides / additions
		if entries:
			by_cat: dict[str, dict[str, str]] = {}
			for e in entries:
				by_cat.setdefault(e["category"], {})[e["key"]] = e["value"]

			for k, v in by_cat.get("user", {}).items():
				lines.append(f"- user {k}: {v} (current session)")
			tech = sorted(by_cat.get("tech_stack", {}).values())
			if tech:
				lines.append(f"- tech stack: {', '.join(tech)} (current session)")
			for k, v in by_cat.get("preference", {}).items():
				lines.append(f"- preference {k}: {v} (current session)")
			for k, v in by_cat.get("project", {}).items():
				lines.append(f"- project: {v} (current session)")

		return "\n".join(lines) if len(lines) > 1 else ""

	def get_stats(self) -> dict[str, Any]:
		with self._conn() as conn:
			enabled = self.get_enabled()
			count = conn.execute(
				"SELECT COUNT(*) FROM memory_entries"
			).fetchone()[0]
			snapshot_count = conn.execute(
				"SELECT COUNT(*) FROM memory_snapshots"
			).fetchone()[0]
		db_size = self.db_path.stat().st_size if self.db_path.exists() else 0
		return {
			"enabled": enabled,
			"entries": count,
			"snapshots": snapshot_count,
			"db_size": db_size,
		}
