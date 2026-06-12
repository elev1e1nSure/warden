from __future__ import annotations

import asyncio
from pathlib import Path

import agent.tools as tools_mod
from agent.tools import FileListTool, SkillTool, TodoWriteTool


def test_file_list_tool_lists_dirs_and_files() -> None:
	base = Path(".tmp") / "test_tools_file_list"
	base.mkdir(parents=True, exist_ok=True)
	(base / "subdir").mkdir(exist_ok=True)
	(base / "alpha.txt").write_text("hello", encoding="utf-8")

	out = asyncio.run(FileListTool().execute({"path": str(base)}))

	assert "dirs: [subdir]" in out
	assert "alpha.txt" in out


def test_file_list_tool_blocks_outside_workspace(monkeypatch) -> None:
	base = Path(".tmp") / "test_tools_file_list_outside"
	base.mkdir(parents=True, exist_ok=True)
	outside = (Path(".tmp") / "test_tools_file_list_outside_other").resolve()
	outside.mkdir(parents=True, exist_ok=True)
	monkeypatch.chdir(base)

	out = asyncio.run(FileListTool().execute({"path": str(outside)}))
	assert "outside allowed scope" in out


def test_file_list_tool_skips_symlink_escape(monkeypatch) -> None:
	import os
	base = Path(".tmp") / "test_tools_file_list_symlink"
	base.mkdir(parents=True, exist_ok=True)
	outside = (Path(".tmp") / "test_tools_file_list_symlink_other").resolve()
	outside.mkdir(parents=True, exist_ok=True)
	(outside / "secret.txt").write_text("secret", encoding="utf-8")
	link = base / "escape"
	if link.exists() or link.is_symlink():
		link.unlink()
	try:
		os.symlink(str(outside), str(link))
	except OSError:
		import pytest
		pytest.skip("symlink requires elevated privileges on Windows")
	monkeypatch.chdir(base)

	out = asyncio.run(FileListTool().execute({"path": str(base)}))
	assert "secret.txt" not in out


def test_todowrite_tool_persists_state() -> None:
	tools_mod._TODO_STORE.clear()
	tools_mod._TODO_STORE["default"] = []

	out = asyncio.run(
		TodoWriteTool().execute(
			{
				"todos": [
					{"content": "first", "status": "pending", "priority": "high"},
					{"content": "second", "status": "completed", "priority": "low"},
				]
			}
		)
	)

	assert "1 todos" in out
	assert len(tools_mod._TODO_STORE["default"]) == 2
	assert tools_mod._TODO_STORE["default"][0]["content"] == "first"


def test_skill_tool_loads_skill_tree(monkeypatch) -> None:
	base = Path(".tmp") / "test_tools_skill"
	skill_dir = base / ".warden" / "skills" / "demo"
	skill_dir.mkdir(parents=True, exist_ok=True)
	(skill_dir / "SKILL.md").write_text(
		"---\nname: demo\ndescription: A demo skill\n---\n\n# Demo\n\nUse carefully.\n",
		encoding="utf-8",
	)
	(skill_dir / "notes.txt").write_text("extra", encoding="utf-8")

	monkeypatch.setattr(tools_mod.Path, "cwd", classmethod(lambda cls: base))
	monkeypatch.setattr(tools_mod.Path, "home", classmethod(lambda cls: base))

	out = asyncio.run(SkillTool().execute({"name": "demo"}))

	assert '<skill_content name="demo">' in out
	assert "Use carefully." in out
	assert "notes.txt" in out


def test_skill_tool_not_found(monkeypatch) -> None:
	base = Path(".tmp") / "test_tools_skill_missing"
	base.mkdir(parents=True, exist_ok=True)
	monkeypatch.setattr(tools_mod.Path, "cwd", classmethod(lambda cls: base))
	monkeypatch.setattr(tools_mod.Path, "home", classmethod(lambda cls: base))

	out = asyncio.run(SkillTool().execute({"name": "ghost"}))
	assert "error" in out
	assert "ghost" in out
