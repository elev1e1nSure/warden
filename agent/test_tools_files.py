"""Tests for file tools."""

from __future__ import annotations

from unittest.mock import patch


class TestFileReadTool:
    async def test_read_with_offset_and_limit(self, tmp_workspace):
        from agent.tools import FileReadTool

        (tmp_workspace / "test.txt").write_text("line1\nline2\nline3\nline4\n")
        tool = FileReadTool()
        result = await tool.execute({"path": "test.txt", "offset": 2, "limit": 2})
        assert "2: line2" in result
        assert "3: line3" in result
        assert "1: line1" not in result

    async def test_read_large_file_skipped(self, tmp_workspace):
        from agent.tools import FileReadTool

        path = tmp_workspace / "big.bin"
        path.write_bytes(b"x" * (51 * 1024 * 1024))
        tool = FileReadTool()
        result = await tool.execute({"path": str(path)})
        assert "too large" in result.lower()

    async def test_read_truncated_line(self, tmp_workspace):
        from agent.tools import FileReadTool

        (tmp_workspace / "long.txt").write_text("x" * 3000 + "\n")
        tool = FileReadTool()
        result = await tool.execute({"path": "long.txt"})
        assert "\u2026" in result

    async def test_read_not_found(self):
        from agent.tools import FileReadTool

        tool = FileReadTool()
        result = await tool.execute({"path": "nonexistent.txt"})
        assert "error" in result.lower()


class TestGlobTool:
    async def test_no_matches(self, tmp_workspace):
        from agent.tools import GlobTool

        tool = GlobTool()
        result = await tool.execute({"pattern": "*.xyz"})
        assert "no matches" in result.lower()

    async def test_matches_sorted(self, tmp_workspace):
        from agent.tools import GlobTool

        (tmp_workspace / "a.py").write_text("a")
        (tmp_workspace / "b.py").write_text("b")
        tool = GlobTool()
        result = await tool.execute({"pattern": "*.py"})
        assert "a.py" in result
        assert "b.py" in result

    async def test_too_many_matches(self, tmp_workspace):
        from agent.tools import GlobTool

        for i in range(250):
            (tmp_workspace / f"f{i}.txt").write_text("x")
        tool = GlobTool()
        result = await tool.execute({"pattern": "*.txt"})
        assert "... and" in result


class TestGrepTool:
    async def test_no_matches(self, tmp_workspace):
        from agent.tools import GrepTool

        (tmp_workspace / "a.txt").write_text("hello world")
        tool = GrepTool()
        result = await tool.execute({"pattern": "xyz", "path": str(tmp_workspace)})
        assert "no matches" in result.lower()

    async def test_fallback_python(self, tmp_workspace):
        from agent.tools import GrepTool

        (tmp_workspace / "a.txt").write_text("hello world\nfoo bar\n")
        tool = GrepTool()
        with patch("shutil.which", return_value=None):
            result = await tool.execute({"pattern": "foo", "path": str(tmp_workspace)})
        assert "foo bar" in result

    async def test_case_insensitive(self, tmp_workspace):
        from agent.tools import GrepTool

        (tmp_workspace / "a.txt").write_text("Hello World\n")
        tool = GrepTool()
        with patch("shutil.which", return_value=None):
            result = await tool.execute({"pattern": "hello", "path": str(tmp_workspace), "case_insensitive": True})
        assert "Hello World" in result

    async def test_single_file(self, tmp_workspace):
        from agent.tools import GrepTool

        (tmp_workspace / "a.txt").write_text("target line\n")
        tool = GrepTool()
        with patch("shutil.which", return_value=None):
            result = await tool.execute({"pattern": "target", "path": str(tmp_workspace / "a.txt")})
        assert "target line" in result


class TestEditTool:
    async def test_not_found(self, tmp_workspace):
        from agent.tools import EditTool

        tool = EditTool()
        result = await tool.execute({"path": "missing.txt", "old_string": "x", "new_string": "y"})
        assert "not found" in result.lower()

    async def test_empty_old_string(self, tmp_workspace):
        from agent.tools import EditTool

        (tmp_workspace / "a.txt").write_text("hello")
        tool = EditTool()
        result = await tool.execute({"path": "a.txt", "old_string": "", "new_string": "y"})
        assert "error" in result.lower()

    async def test_multiple_matches(self, tmp_workspace):
        from agent.tools import EditTool

        (tmp_workspace / "a.txt").write_text("abc abc abc")
        tool = EditTool()
        result = await tool.execute({"path": "a.txt", "old_string": "abc", "new_string": "x"})
        assert "3 times" in result.lower()


class TestFileWriteTool:
    async def test_write_new_file(self, tmp_workspace):
        from agent.tools import FileWriteTool

        tool = FileWriteTool()
        result = await tool.execute({"path": "new.txt", "content": "hello"})
        assert "wrote" in result.lower()
        assert (tmp_workspace / "new.txt").read_text() == "hello"

    async def test_write_creates_dirs(self, tmp_workspace):
        from agent.tools import FileWriteTool

        tool = FileWriteTool()
        result = await tool.execute({"path": "deep/nested/file.txt", "content": "x"})
        assert "wrote" in result.lower()
        assert (tmp_workspace / "deep" / "nested" / "file.txt").read_text() == "x"


class TestFileDeleteTool:
    async def test_delete_outside_blocked(self, tmp_workspace):
        from agent.tools import FileDeleteTool

        tool = FileDeleteTool()
        result = await tool.execute({"path": "../outside.txt"})
        assert "outside" in result.lower()

    async def test_delete_not_found(self, tmp_workspace):
        from agent.tools import FileDeleteTool

        tool = FileDeleteTool()
        result = await tool.execute({"path": "missing.txt"})
        assert "not found" in result.lower()

    async def test_delete_directory_blocked(self, tmp_workspace):
        from agent.tools import FileDeleteTool

        (tmp_workspace / "d").mkdir()
        tool = FileDeleteTool()
        result = await tool.execute({"path": "d"})
        assert "directory" in result.lower()


class TestFileListTool:
    async def test_empty_dir(self, tmp_workspace):
        from agent.tools import FileListTool

        tool = FileListTool()
        result = await tool.execute({"path": str(tmp_workspace)})
        assert result == "(empty)"

    async def test_lists_files_and_dirs(self, tmp_workspace):
        from agent.tools import FileListTool

        (tmp_workspace / "a.txt").write_text("hello")
        (tmp_workspace / "subdir").mkdir()
        tool = FileListTool()
        result = await tool.execute({"path": str(tmp_workspace)})
        assert "a.txt" in result
        assert "[subdir]" in result

    async def test_outside_workspace_blocked(self, tmp_workspace):
        from agent.tools import FileListTool

        tool = FileListTool()
        result = await tool.execute({"path": "C:/Windows"})
        assert "error" in result.lower()
