"""Tests for the archive, process, and move tools."""

from __future__ import annotations

import os
import tarfile
import zipfile

import pytest


@pytest.fixture
def tmp_workspace(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


# ── archive ──────────────────────────────────────────────────────────────────


class TestArchive:
    async def test_create_and_list_zip(self, tmp_workspace):
        from agent.tools import ArchiveTool

        (tmp_workspace / "a.txt").write_text("hello")
        (tmp_workspace / "b.txt").write_text("world")
        t = ArchiveTool()
        r = await t.execute(
            {
                "action": "create",
                "path": "out.zip",
                "sources": ["a.txt", "b.txt"],
            }
        )
        assert "created" in r.lower(), r
        assert (tmp_workspace / "out.zip").exists()
        r2 = await t.execute({"action": "list", "path": "out.zip"})
        assert "a.txt" in r2
        assert "b.txt" in r2

    async def test_extract_zip(self, tmp_workspace):
        from agent.tools import ArchiveTool

        # Build a zip with subdirectory entry
        with zipfile.ZipFile(tmp_workspace / "src.zip", "w") as zf:
            zf.writestr("top.txt", "x")
            zf.writestr("nested/inner.txt", "y")
        t = ArchiveTool()
        r = await t.execute(
            {
                "action": "extract",
                "path": "src.zip",
                "dest": "out",
            }
        )
        assert "extracted" in r.lower()
        assert (tmp_workspace / "out" / "top.txt").read_text() == "x"
        assert (tmp_workspace / "out" / "nested" / "inner.txt").read_text() == "y"

    async def test_create_tar_gz_and_list(self, tmp_workspace):
        from agent.tools import ArchiveTool

        (tmp_workspace / "x.txt").write_text("payload")
        t = ArchiveTool()
        r = await t.execute(
            {
                "action": "create",
                "path": "out.tar.gz",
                "sources": ["x.txt"],
            }
        )
        assert "created" in r.lower()
        # tar.gz should be a valid tar
        with tarfile.open(tmp_workspace / "out.tar.gz", "r:gz") as tf:
            names = [m.name for m in tf.getmembers()]
        assert any(n.endswith("x.txt") for n in names), names

    async def test_unknown_extension(self, tmp_workspace):
        from agent.tools import ArchiveTool

        t = ArchiveTool()
        r = await t.execute({"action": "list", "path": "foo.7z"})
        assert "error" in r

    async def test_zip_slip_blocked(self, tmp_workspace):
        from agent.tools import ArchiveTool

        # Manually craft a zip with a path-traversal member
        with zipfile.ZipFile(tmp_workspace / "evil.zip", "w") as zf:
            zf.writestr("../escaped.txt", "boom")
        t = ArchiveTool()
        r = await t.execute(
            {
                "action": "extract",
                "path": "evil.zip",
                "dest": "out",
            }
        )
        assert "error" in r.lower() and "slip" in r.lower(), r

    async def test_extract_outside_workspace_blocked(self, tmp_workspace):
        from agent.tools import ArchiveTool

        with zipfile.ZipFile(tmp_workspace / "ok.zip", "w") as zf:
            zf.writestr("a.txt", "x")
        t = ArchiveTool()
        r = await t.execute(
            {
                "action": "extract",
                "path": "ok.zip",
                "dest": "..",
            }
        )
        assert "error" in r.lower()

    def test_is_dangerous_only_for_mutations(self):
        from agent.tools import ArchiveTool

        assert ArchiveTool().is_dangerous({"action": "list"}) is False
        assert ArchiveTool().is_dangerous({"action": "extract"}) is True
        assert ArchiveTool().is_dangerous({"action": "create"}) is True


# ── process ──────────────────────────────────────────────────────────────────


class TestProcessList:
    async def test_returns_table(self, tmp_workspace, monkeypatch):
        import agent.tools.process as process_mod
        from agent.tools import ProcessListTool

        # Stub out the platform-specific list fn so the test is OS-agnostic.
        # It must be a coroutine, mirroring the real implementation.
        t = ProcessListTool()

        async def fake_list(_f):
            return [("1234", "python.exe"), ("5678", "node.exe")]

        monkeypatch.setattr(process_mod, "_is_windows", lambda: True)
        monkeypatch.setattr(t, "_list_windows", fake_list)
        r = await t.execute({})
        assert "1234" in r and "python.exe" in r
        assert "5678" in r and "node.exe" in r

    async def test_filter(self, tmp_workspace, monkeypatch):
        import agent.tools.process as process_mod
        from agent.tools import ProcessListTool

        t = ProcessListTool()

        async def fake_list(f):
            rows = [("1", "python.exe"), ("2", "chrome.exe")]
            return [(p, n) for p, n in rows if not f or f in n.lower()]

        monkeypatch.setattr(process_mod, "_is_windows", lambda: True)
        monkeypatch.setattr(t, "_list_windows", fake_list)
        r = await t.execute({"filter": "python"})
        assert "python.exe" in r
        assert "chrome.exe" not in r

    async def test_empty_filter(self, tmp_workspace, monkeypatch):
        import agent.tools.process as process_mod
        from agent.tools import ProcessListTool

        t = ProcessListTool()

        async def fake_list(_f):
            return []

        monkeypatch.setattr(process_mod, "_is_windows", lambda: True)
        monkeypatch.setattr(t, "_list_windows", fake_list)
        r = await t.execute({})
        assert "no processes" in r


class TestProcessKill:
    async def test_refuses_pid_zero(self):
        from agent.tools import ProcessKillTool

        r = await ProcessKillTool().execute({"pid": 0})
        assert "error" in r.lower()

    async def test_refuses_self(self):
        from agent.tools import ProcessKillTool

        r = await ProcessKillTool().execute({"pid": os.getpid()})
        assert "self" in r.lower()

    async def test_refuses_non_int(self):
        from agent.tools import ProcessKillTool

        r = await ProcessKillTool().execute({"pid": "abc"})
        assert "integer" in r.lower()

    def test_is_dangerous(self):
        from agent.tools import ProcessKillTool

        assert ProcessKillTool().is_dangerous({}) is True


# ── move / copy ──────────────────────────────────────────────────────────────


class TestFileMove:
    async def test_renames_within_workspace(self, tmp_workspace):
        from agent.tools import FileMoveTool

        (tmp_workspace / "old.txt").write_text("x")
        r = await FileMoveTool().execute({"src": "old.txt", "dest": "new.txt"})
        assert "moved" in r.lower()
        assert not (tmp_workspace / "old.txt").exists()
        assert (tmp_workspace / "new.txt").read_text() == "x"

    async def test_refuses_outside_workspace(self, tmp_workspace):
        from agent.tools import FileMoveTool

        r = await FileMoveTool().execute({"src": "..\\evil", "dest": "new.txt"})
        assert "outside" in r.lower()

    async def test_refuses_missing_source(self, tmp_workspace):
        from agent.tools import FileMoveTool

        r = await FileMoveTool().execute({"src": "nope.txt", "dest": "x.txt"})
        assert "not found" in r.lower()

    async def test_refuses_move_into_self(self, tmp_workspace):
        from agent.tools import FileMoveTool

        (tmp_workspace / "a").mkdir()
        (tmp_workspace / "a" / "f.txt").write_text("x")
        r = await FileMoveTool().execute({"src": "a", "dest": "a/inside"})
        assert "itself" in r.lower()

    def test_is_dangerous_outside(self, tmp_workspace):
        from agent.tools import FileMoveTool

        assert FileMoveTool().is_dangerous({"src": "..\\x", "dest": "y"}) is True
        assert FileMoveTool().is_dangerous({"src": "a", "dest": "b"}) is False


class TestFileCopy:
    async def test_copies_within_workspace(self, tmp_workspace):
        from agent.tools import FileCopyTool

        (tmp_workspace / "src.txt").write_text("payload")
        r = await FileCopyTool().execute({"src": "src.txt", "dest": "dest.txt"})
        assert "copied" in r.lower()
        assert (tmp_workspace / "src.txt").exists()  # source preserved
        assert (tmp_workspace / "dest.txt").read_text() == "payload"

    async def test_creates_intermediate_dirs(self, tmp_workspace):
        from agent.tools import FileCopyTool

        (tmp_workspace / "src.txt").write_text("x")
        r = await FileCopyTool().execute({"src": "src.txt", "dest": "deep/nested/dest.txt"})
        assert "copied" in r.lower()
        assert (tmp_workspace / "deep" / "nested" / "dest.txt").exists()

    async def test_refuses_directory_source(self, tmp_workspace):
        from agent.tools import FileCopyTool

        (tmp_workspace / "d").mkdir()
        r = await FileCopyTool().execute({"src": "d", "dest": "copy"})
        assert "directory" in r.lower()
