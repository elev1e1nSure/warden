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

    def test_git_prefix_stripped(self):
        patch = "--- a/src/file.py\n+++ b/src/file.py\n@@ -1,1 +1,1 @@\n-old\n+new\n"
        files = self._tool()._parse_patch(patch)
        assert len(files) == 1
        assert files[0]["path"] == "src/file.py"


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

    async def test_multi_hunk_offset_drift(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        f = tmp_path / "src.py"
        f.write_text("line1\nOLD_A\nline3\nOLD_B\nline5\n")
        patch = (
            "--- src.py\n+++ src.py\n"
            "@@ -1,3 +1,3 @@\n line1\n-OLD_A\n+NEW_A\n line3\n"
            "@@ -3,3 +3,3 @@\n line3\n-OLD_B\n+NEW_B\n line5\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "patched" in result
        text = f.read_text()
        assert "NEW_A" in text
        assert "NEW_B" in text
        assert "OLD_A" not in text
        assert "OLD_B" not in text

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


# ── Opencode / Anthropic `*** Begin Patch` format ────────────────────────────


class TestOpencodeParse:
    def _tool(self):
        from agent.tools import ApplyPatchTool
        return ApplyPatchTool()

    def test_update_file(self):
        patch = (
            "*** Begin Patch\n"
            "*** Update File: foo.py\n"
            " line1\n"
            "-OLD\n"
            "+NEW\n"
            " line3\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert len(files) == 1
        f = files[0]
        assert f["kind"] == "update"
        assert f["path"] == "foo.py"
        assert len(f["hunks"]) == 1
        h = f["hunks"][0]
        assert h["old_lines"] == ["line1", "OLD", "line3"]
        assert h["new_lines"] == ["line1", "NEW", "line3"]
        assert h["at_eof"] is False

    def test_add_file(self):
        patch = (
            "*** Begin Patch\n"
            "*** Add File: new.py\n"
            "+hello\n"
            "+world\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert len(files) == 1
        f = files[0]
        assert f["kind"] == "add"
        assert f["path"] == "new.py"
        assert f["hunks"][0]["new_lines"] == ["hello", "world"]

    def test_delete_file(self):
        patch = (
            "*** Begin Patch\n"
            "*** Delete File: gone.py\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert len(files) == 1
        assert files[0]["kind"] == "delete"
        assert files[0]["path"] == "gone.py"

    def test_move_to(self):
        patch = (
            "*** Begin Patch\n"
            "*** Update File: old.go\n"
            " body\n"
            "*** Move to: new.go\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert files[0]["move_to"] == "new.go"

    def test_end_of_file_marker(self):
        patch = (
            "*** Begin Patch\n"
            "*** Update File: foo.py\n"
            " last\n"
            "*** End of File\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert files[0]["hunks"][0]["at_eof"] is True

    def test_multiple_files(self):
        patch = (
            "*** Begin Patch\n"
            "*** Add File: a.py\n"
            "+x\n"
            "*** Add File: b.py\n"
            "+y\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert [f["path"] for f in files] == ["a.py", "b.py"]

    def test_garbage_before_begin_ignored(self):
        patch = (
            "ignore me\n"
            "*** Begin Patch\n"
            "*** Add File: c.py\n"
            "+z\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert len(files) == 1
        assert files[0]["path"] == "c.py"

    def test_path_normalization_strips_a_b_prefix(self):
        patch = (
            "*** Begin Patch\n"
            "*** Update File: a/src/foo.py\n"
            " x\n"
            "*** End Patch\n"
        )
        files = self._tool()._parse_opencode(patch)
        assert files[0]["path"] == "src/foo.py"


class TestOpencodeApply:
    async def test_empty_patch_rejected(self):
        from agent.tools import ApplyPatchTool
        result = await ApplyPatchTool().execute({"patch_text": "*** Begin Patch\n*** End Patch\n"})
        assert "error" in result

    async def test_add_new_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        patch = (
            "*** Begin Patch\n"
            "*** Add File: new.txt\n"
            "+hello\n"
            "+world\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "added" in result
        assert (tmp_path / "new.txt").read_text() == "hello\nworld\n"

    async def test_update_existing_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "src.py").write_text("a\nOLD\nc\n")
        patch = (
            "*** Begin Patch\n"
            "*** Update File: src.py\n"
            " a\n"
            "-OLD\n"
            "+NEW\n"
            " c\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "patched" in result
        assert (tmp_path / "src.py").read_text() == "a\nNEW\nc\n"

    async def test_delete_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "del.txt").write_text("bye\n")
        patch = (
            "*** Begin Patch\n"
            "*** Delete File: del.txt\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "deleted" in result
        assert not (tmp_path / "del.txt").exists()

    async def test_move_renames_file(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "old.go").write_text("package x\n")
        patch = (
            "*** Begin Patch\n"
            "*** Update File: old.go\n"
            " package x\n"
            "*** Move to: new.go\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "renamed" in result
        assert not (tmp_path / "old.go").exists()
        assert (tmp_path / "new.go").read_text() == "package x\n"

    async def test_update_with_eof_marker(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "eof.py").write_text("keep\nlast\n")
        patch = (
            "*** Begin Patch\n"
            "*** Update File: eof.py\n"
            " last\n"
            "+trailing\n"
            "*** End of File\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "patched" in result
        assert (tmp_path / "eof.py").read_text() == "keep\nlast\ntrailing\n"

    async def test_update_missing_file_skipped(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        patch = (
            "*** Begin Patch\n"
            "*** Update File: ghost.py\n"
            " x\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "not found" in result

    async def test_multi_file_patch(self, tmp_path, monkeypatch):
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "a.py").write_text("x\n")
        (tmp_path / "b.py").write_text("y\n")
        patch = (
            "*** Begin Patch\n"
            "*** Update File: a.py\n"
            "-x\n"
            "+X\n"
            "*** Update File: b.py\n"
            "-y\n"
            "+Y\n"
            "*** End Patch\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert (tmp_path / "a.py").read_text() == "X\n"
        assert (tmp_path / "b.py").read_text() == "Y\n"

    async def test_unified_diff_still_works(self, tmp_path, monkeypatch):
        """Regression: legacy unified-diff path must still work alongside opencode format."""
        from agent.tools import ApplyPatchTool
        monkeypatch.chdir(tmp_path)
        (tmp_path / "u.py").write_text("one\nOLD\nthree\n")
        patch = (
            "--- u.py\n"
            "+++ u.py\n"
            "@@ -1,3 +1,3 @@\n"
            " one\n"
            "-OLD\n"
            "+NEW\n"
            " three\n"
        )
        result = await ApplyPatchTool().execute({"patch_text": patch})
        assert "patched" in result
        assert (tmp_path / "u.py").read_text() == "one\nNEW\nthree\n"
