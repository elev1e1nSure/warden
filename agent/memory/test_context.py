import pytest
from pathlib import Path

from agent.memory.store import MemoryStore


@pytest.fixture
def store(tmp_path: Path) -> MemoryStore:
	return MemoryStore(tmp_path / "test.db")


class TestContextText:
	def test_empty(self, store: MemoryStore) -> None:
		assert store.get_context_text() == ""

	def test_snapshot(self, store: MemoryStore) -> None:
		store.save_snapshot("s1", {
			"user": {"name": "Alice", "preferred_language": "ru"},
			"projects": [{"name": "warden", "tech_stack": ["python", "go"]}],
			"preferences": {"theme": "dark"},
		})
		ctx = store.get_context_text()
		assert "Alice" in ctx
		assert "warden" in ctx
		assert "python" in ctx
		assert "dark" in ctx
		assert "[Memory context]" in ctx
