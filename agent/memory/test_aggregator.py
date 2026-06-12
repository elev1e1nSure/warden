import pytest
from pathlib import Path

from agent.memory.store import MemoryStore
from agent.memory.aggregator import MemoryAggregator


@pytest.fixture
def store(tmp_path: Path) -> MemoryStore:
	return MemoryStore(tmp_path / "test.db")


class TestAggregate:
	def test_empty(self, store: MemoryStore) -> None:
		snap = MemoryAggregator.aggregate(store, "s1")
		assert snap["user"] == {}
		assert snap["projects"] == []
		assert snap["preferences"] == {}
		assert "updated_at" in snap

	def test_user_and_preference(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		store.upsert_entry("s1", "preference", "theme", "dark", 1.0)
		snap = MemoryAggregator.aggregate(store, "s1")
		assert snap["user"] == {"name": "Alice"}
		assert snap["preferences"] == {"theme": "dark"}

	def test_project_with_tech(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "project", "name", "warden", 1.0)
		store.upsert_entry("s1", "tech_stack", "python", "python", 1.0)
		store.upsert_entry("s1", "tech_stack", "go", "go", 1.0)
		snap = MemoryAggregator.aggregate(store, "s1")
		assert len(snap["projects"]) == 1
		assert snap["projects"][0]["name"] == "warden"
		assert snap["projects"][0]["tech_stack"] == ["go", "python"]

	def test_different_session(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		store.upsert_entry("s2", "user", "lang", "ru", 1.0)
		snap = MemoryAggregator.aggregate(store, "s1")
		assert snap["user"] == {"name": "Alice"}


class TestMergeSnapshots:
	def test_merge_user_and_preferences(self, store: MemoryStore) -> None:
		prev = {"user": {"name": "Alice"}, "preferences": {"theme": "light"}}
		curr = {"user": {"name": "Bob"}, "preferences": {"lang": "ru"}}
		merged = MemoryAggregator._merge_snapshots(prev, curr)
		assert merged["user"] == {"name": "Bob"}
		assert merged["preferences"] == {"theme": "light", "lang": "ru"}

	def test_merge_projects(self, store: MemoryStore) -> None:
		prev = {"projects": [{"name": "old", "tech_stack": ["go"]}]}
		curr = {"projects": [{"name": "new", "tech_stack": ["python"]}]}
		merged = MemoryAggregator._merge_snapshots(prev, curr)
		names = {p["name"] for p in merged["projects"]}
		assert names == {"old", "new"}

	def test_merge_tech_stack(self, store: MemoryStore) -> None:
		prev = {"tech_stack": ["go"]}
		curr = {"projects": [{"name": "p", "tech_stack": ["python"]}]}
		merged = MemoryAggregator._merge_snapshots(prev, curr)
		assert set(merged["tech_stack"]) == {"go", "python"}

	def test_current_wins(self, store: MemoryStore) -> None:
		prev = {"user": {"name": "Alice"}}
		curr = {"user": {"name": "Bob"}}
		merged = MemoryAggregator._merge_snapshots(prev, curr)
		assert merged["user"]["name"] == "Bob"


class TestFinalize:
	def test_persists_snapshot(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		MemoryAggregator.finalize(store, "s1")
		snap = store.get_latest_snapshot()
		assert snap is not None
		assert snap["user"] == {"name": "Alice"}

	def test_finalize_clears_entries(self, store: MemoryStore) -> None:
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		MemoryAggregator.finalize(store, "s1")
		assert store.get_entries(session_id="s1") == []

	def test_finalize_merges_with_previous(self, store: MemoryStore) -> None:
		store.save_snapshot("s0", {"user": {"lang": "en"}})
		store.upsert_entry("s1", "user", "name", "Alice", 1.0)
		MemoryAggregator.finalize(store, "s1")
		snap = store.get_latest_snapshot()
		assert snap["user"] == {"lang": "en", "name": "Alice"}
