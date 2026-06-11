"""Tests for ApplyPatchTool — unified diff apply logic."""
from __future__ import annotations

import pytest


class TestParsePatch:
    def _tool(self):
        from agent.tools import ApplyPatchTool
        return ApplyPatchTool()

    def test_add_file(self):
        patch = "--- /dev/null\n+++ new_file.py\n@@ -0,0 +1,2 @@\n+line1\n+line2\n"
        files = self._tool()._parse_patch(patch)
        assert len(files) == 1
        assert files[0]["is_add"] is True
        assert files[0]["is_delete"] is False
        assert files[0]["path"] == "new_file.py"

    def test_delete_file(self):
        patch = "--- old.py\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-line\n"
        files = self._tool()._parse_patch(patch)
        assert len(files) == 1
        assert files[0]["is_delete"] is True
        assert files[0]["is_add"] is False
        assert files[0]["path"] == "old.py"

    def test_rename(self):
        patch = "--- a.py\n+++ b.py\n@@ -1,1 +1,1 @@\n line\n"
        files = self._tool()._parse_patch(patch)
        assert len(files) == 1
        assert files[0]["is_rename"] is True

    def test_regular_update(self):
        patch = "--- a.py\n+++ a.py\n@@ -1,1 +1,1 @@\n-old\n+new\n"
        files = self._tool()._parse_patch(patch)
        assert len(files) == 1
        f = files[0]
        assert not f["is_add"]
        assert not f["is_delete"]
        assert not f["is_rename"]

    def test_no_hunks_returns_empty(self):
        # malformed patch without +++ line
        patch = "--- a.py\nsome garbage\n"
        files = self._tool()._parse_patch(patch)
        assert files == []

    def test_leading_slash_stripped(self):
        patch = "--- /dev/null\n+++ /src/foo.py\n@@ -0,0 +1 @@\n+x\n"
        files = self._tool()._parse_patch(patch)
        # leading slash stripped but not Windows drive
        assert not files[0]["path"].startswith("/")

    def test_windows_path_kept(self):
        patch = "--- /dev/null\n+++ /c:/foo/bar.py\n@@ -0,0 +1 @@\n+x\n"
        files = self._tool()._parse_patch(patch)
        # Windows path like c:/foo/bar.py kept intact
        assert "c:" in files[0]["path"].lower() or "bar.py" in files[0]["path"]

    def test_multiple_files(self):
        patch = (
            "--- a.py\n+++ a.py\n@@ -1,1 +1,1 @@\n-old\n+new\n"
            "--- b.py\n+++ b.py\n@@ -1,1 +1,1 @@\n-x\n+y\n"
        )
        files = self._tool()._parse_patch(patch)
        assert len(files) == 2


class TestApplyHunk:
    def _tool(self):
        from agent.tools import ApplyPatchTool
        return ApplyPatchTool()

    def test_replace_context_match(self):
        content = "line1\nOLD\nline3"
        hunk = {
            "old_start": 2,
            "old_count": 1,
            "new_start": 2,
            "new_count": 1,
            "lines": ["-OLD", "+NEW"],
        }
        result = self._tool()._apply_hunk(content, hunk)
        assert result == "line1\nNEW\nline3"

    def test_match_far_from_hint(self):
        content = "a\nb\nc\nOLD\ne"
        hunk = {
            "old_start": 1,  # wrong hint
            "old_count": 1,
            "new_start": 1,
            "new_count": 1,
            "lines": ["-OLD", "+NEW"],
        }
        result = self._tool()._apply_hunk(content, hunk)
        assert result == "a\nb\nc\nNEW\ne"

    def test_pure_addition(self):
        content = "a\nb"
        hunk = {
            "old_start": 2,
            "old_count": 0,
            "new_start": 2,
            "new_count": 1,
            "lines": ["+inserted"],
        }
        result = self._tool()._apply_hunk(content, hunk)
        assert "inserted" in result

    def test_match_fails_returns_none(self):
        content = "x\ny\nz"
        hunk = {
            "old_start": 1,
            "old_count": 1,
            "new_start": 1,
            "new_count": 1,
            "lines": ["-NOTHERE", "+NEW"],
        }
        result = self._tool()._apply_hunk(content, hunk)
        assert result is None

    def test_context_lines_preserved(self):
        content = "ctx\nOLD\nctx2"
        hunk = {
            "old_start": 1,
            "old_count": 3,
            "new_start": 1,
            "new_count": 3,
            "lines": [" ctx", "-OLD", "+NEW", " ctx2"],
        }
        result = self._tool()._apply_hunk(content, hunk)
        assert result == "ctx\nNEW\nctx2"


class TestApplyPatchExecute:
    async def test_empty_patch_text(self):
        from agent.tools import ApplyPatchTool
        result = await ApplyPatchTool().execute({"patch_text": ""})
        assert "error" in result

    async def test_add_new_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        patch = "--- /dev/null\n+++ new.txt\n@@ -0,0 +1,1 @@\n+hello\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "added" in result
        assert (tmp_path / "new.txt").read_text().strip() == "hello"

    async def test_patch_existing_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "src.py"
        f.write_text("line1\nOLD\nline3\n")
        patch = "--- src.py\n+++ src.py\n@@ -1,3 +1,3 @@\n line1\n-OLD\n+NEW\n line3\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "patched" in result
        assert "NEW" in f.read_text()

    async def test_delete_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "del.txt"
        f.write_text("bye")
        patch = "--- del.txt\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-bye\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "deleted" in result
        assert not f.exists()

    async def test_delete_nonexistent_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        patch = "--- missing.txt\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-x\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "not found" in result

    async def test_delete_directory_skipped(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        d = tmp_path / "mydir"
        d.mkdir()
        patch = f"--- mydir\n+++ /dev/null\n@@ -1,1 +0,0 @@\n-x\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "directory" in result

    async def test_rename_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        old = tmp_path / "old.txt"
        old.write_text("content\n")
        # trailing \n on last hunk line means empty context — file needs matching trailing newline
        patch = "--- old.txt\n+++ new.txt\n@@ -1,1 +1,1 @@\n content\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "renamed" in result
        assert not old.exists()
        assert (tmp_path / "new.txt").exists()

    async def test_rename_source_missing(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        patch = "--- ghost.txt\n+++ new.txt\n@@ -1,1 +1,1 @@\n line\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "not found" in result.lower()

    async def test_hunk_match_failure(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "src.py"
        f.write_text("unrelated content\n")
        patch = "--- src.py\n+++ src.py\n@@ -1,1 +1,1 @@\n-NOTHERE\n+something\n"
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "failed to match" in result

    async def test_no_valid_hunks(self):
        from agent.tools import ApplyPatchTool
        result = await ApplyPatchTool().execute({"patch_text": "just garbage text\n"})
        assert "error" in result

    def test_is_dangerous(self):
        from agent.tools import ApplyPatchTool
        assert ApplyPatchTool().is_dangerous({}) is True
