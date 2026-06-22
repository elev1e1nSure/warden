from pathlib import Path

import pytest

from agent.memory.store import MemoryStore


@pytest.fixture
def store(tmp_path: Path) -> MemoryStore:
    return MemoryStore(tmp_path / "test.db")


class TestContextText:
    def test_empty(self, store: MemoryStore) -> None:
        assert store.get_context_text() == ""

    def test_user_fact(self, store: MemoryStore) -> None:
        store.upsert_entry("s1", "user", "name", "Alice", 0.9)
        ctx = store.get_context_text()
        assert "Alice" in ctx
        assert "[Memory]" in ctx

    def test_tech_stack(self, store: MemoryStore) -> None:
        store.upsert_entry("s1", "tech_stack", "python", "python", 0.8)
        store.upsert_entry("s1", "tech_stack", "go", "go", 0.8)
        ctx = store.get_context_text()
        assert "python" in ctx
        assert "go" in ctx

    def test_preference(self, store: MemoryStore) -> None:
        store.upsert_entry("s1", "preference", "theme", "dark", 0.9)
        ctx = store.get_context_text()
        assert "dark" in ctx

    def test_project(self, store: MemoryStore) -> None:
        store.upsert_entry("s1", "project", "name", "warden", 0.9)
        ctx = store.get_context_text()
        assert "warden" in ctx

    def test_cross_session_facts_visible(self, store: MemoryStore) -> None:
        # Facts from session A must show up in session B (no snapshot required).
        store.upsert_entry("session-a", "user", "name", "Bob", 0.9)
        store.upsert_entry("session-b", "tech_stack", "rust", "rust", 0.8)
        ctx = store.get_context_text()
        assert "Bob" in ctx
        assert "rust" in ctx

    def test_upsert_updates_value(self, store: MemoryStore) -> None:
        store.upsert_entry("s1", "user", "name", "Old", 0.5)
        store.upsert_entry("s2", "user", "name", "New", 0.9)
        ctx = store.get_context_text()
        assert "New" in ctx
        assert "Old" not in ctx
