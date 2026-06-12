import pytest
from pathlib import Path

from agent.memory.store import MemoryStore


@pytest.fixture
def store(tmp_path: Path) -> MemoryStore:
	db = tmp_path / "test.db"
	return MemoryStore(db)


class TestInit:
	def test_creates_tables(self, store: MemoryStore) -> None:
		with store._conn() as conn:
			tables = conn.execute(
				"SELECT name FROM sqlite_master WHERE type='table'"
			).fetchall()
			names = {t[0] for t in tables}
		assert "memory_entries" in names
		assert "memory_state" in names
		assert "memory_snapshots" in names


class TestEnabled:
	def test_default_disabled(self, store: MemoryStore) -> None:
		assert store.get_enabled() is False

	def test_set_enabled(self, store: MemoryStore) -> None:
		store.set_enabled(True)
		assert store.get_enabled() is True
		store.set_enabled(False)
		assert store.get_enabled() is False


class TestEntries:
	def test_upsert_and_get(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "lang", "ru", 0.9)
		entries = store.get_entries()
		assert len(entries) == 1
		assert entries[0]["category"] == "user"
		assert entries[0]["key"] == "lang"
		assert entries[0]["value"] == "ru"
		assert entries[0]["confidence"] == 0.9

	def test_upsert_updates_duplicate(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "lang", "ru", 0.5)
		store.upsert_entry("s2", "user", "lang", "en", 0.95)
		entries = store.get_entries()
		assert len(entries) == 1
		assert entries[0]["value"] == "en"
		assert entries[0]["confidence"] == 0.95

	def test_get_by_session(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "project", "name", "warden", 1.0)
		store.upsert_entry("s2", "project", "desc", "other", 1.0)
		result = store.get_entries(session_id="s1")
		assert len(result) == 1
		assert result[0]["value"] == "warden"

	def test_get_by_category(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		store.upsert_entry("s1", "preference", "theme", "dark", 1.0)
		assert len(store.get_entries(category="user")) == 1

	def test_clear_all(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "a", "k", "v", 1.0)
		deleted = store.clear_entries()
		assert deleted == 1
		assert store.get_entries() == []

	def test_clear_by_session(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "a", "k1", "v1", 1.0)
		store.upsert_entry("s2", "a", "k2", "v2", 1.0)
		deleted = store.clear_entries(session_id="s1")
		assert deleted == 1
		assert len(store.get_entries()) == 1


class TestSnapshots:
	def test_save_and_get(self, store: MemoryStore) -> None:
		store.save_snapshot("s1", {"user": {"name": "Alice"}})
		snap = store.get_latest_snapshot()
		assert snap == {"user": {"name": "Alice"}}

	def test_latest_returns_most_recent(self, store: MemoryStore) -> None:
		store.save_snapshot("s1", {"a": 1})
		store.save_snapshot("s2", {"a": 2})
		assert store.get_latest_snapshot() == {"a": 2}

	def test_none_when_empty(self, store: MemoryStore) -> None:
		assert store.get_latest_snapshot() is None


class TestDeleteEntry:
	def test_delete_existing(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		deleted = store.delete_entry("name")
		assert deleted == 1
		assert store.get_entries() == []

	def test_delete_missing(self, store: MemoryStore) -> None:
		assert store.delete_entry("missing") == 0


class TestContextWithSnapshot:
	def test_snapshot_included(self, store: MemoryStore) -> None:
		store.save_snapshot("s1", {"user": {"name": "Alice"}})
		ctx = store.get_context_text()
		assert "Alice" in ctx
		assert "[Memory]" in ctx

	def test_current_session_overlay(self, store: MemoryStore) -> None:
		store.save_snapshot("s1", {"user": {"name": "Alice"}})
		store.upsert_entry("s2", "user", "name", "Bob", 0.9)
		ctx = store.get_context_text(session_id="s2")
		assert "Alice" in ctx
		assert "Bob" in ctx
		assert "current session" in ctx

	def test_confidence_filter(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 0.3)
		ctx = store.get_context_text(session_id="s1", min_confidence=0.5)
		assert "Alice" not in ctx


class TestStats:
	def test_stats(self, store: MemoryStore) -> None:
		store.set_enabled(True)
		store.upsert_entry("s1", "user", "k", "v", 1.0)
		store.save_snapshot("s1", {})
		stats = store.get_stats()
		assert stats["enabled"] is True
		assert stats["entries"] == 1
		assert stats["snapshots"] == 1
		assert stats["db_size"] > 0
