import os
from pathlib import Path

import pytest

from agent.tools.memory import MemoryTool


@pytest.fixture
def tool(tmp_path: Path) -> MemoryTool:
    os.environ["WARDEN_MEMORY_DB"] = str(tmp_path / "test.db")
    t = MemoryTool()
    return t


@pytest.mark.anyio
async def test_memory_set_and_get(tool: MemoryTool) -> None:
    result = await tool.execute({"action": "set", "key": "test_key", "value": "test_value"})
    assert "saved" in result

    result = await tool.execute({"action": "get", "key": "test_key"})
    assert "test_value" in result


@pytest.mark.anyio
async def test_memory_list(tool: MemoryTool) -> None:
    await tool.execute({"action": "set", "key": "k1", "value": "v1"})
    await tool.execute({"action": "set", "key": "k2", "value": "v2"})
    result = await tool.execute({"action": "list"})
    assert "k1" in result
    assert "k2" in result


@pytest.mark.anyio
async def test_memory_delete(tool: MemoryTool) -> None:
    await tool.execute({"action": "set", "key": "del_key", "value": "val"})
    result = await tool.execute({"action": "delete", "key": "del_key"})
    assert "deleted" in result

    result = await tool.execute({"action": "get", "key": "del_key"})
    assert "no note" in result


@pytest.mark.anyio
async def test_memory_clear(tool: MemoryTool) -> None:
    await tool.execute({"action": "set", "key": "k", "value": "v"})
    result = await tool.execute({"action": "clear"})
    assert "cleared" in result

    result = await tool.execute({"action": "list"})
    assert "(empty)" in result
