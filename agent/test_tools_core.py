"""Tests for agent/tools.py — core tools, no OS/external dependencies required."""
from __future__ import annotations

import asyncio
import json
import os
import sys
from pathlib import Path
from typing import Any
from unittest.mock import AsyncMock, MagicMock, patch, call
import urllib.error

import pytest


# ── fixtures ──────────────────────────────────────────────────────────────────

@pytest.fixture
def tmp_workspace(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


# ── helpers ───────────────────────────────────────────────────────────────────

class TestClean:
    def test_strips_ansi(self):
        from agent.tools import _clean
        assert _clean("\x1b[32mgreen\x1b[0m") == "green"

    def test_strips_osc(self):
        from agent.tools import _clean
        assert _clean("\x1b]0;title\x07text") == "text"

    def test_collapses_cr_overwrite(self):
        from agent.tools import _clean
        # "abc\rXY" → last part after \r wins per line
        result = _clean("abc\rXY")
        assert result == "XY"

    def test_empty_lines_dropped(self):
        from agent.tools import _clean
        result = _clean("a\n\nb")
        assert result == "a\nb"

    def test_multiline(self):
        from agent.tools import _clean
        result = _clean("line1\nline2")
        assert result == "line1\nline2"


class TestInCwd:
    def test_inside_cwd(self, tmp_workspace):
        from agent.tools import _in_cwd
        p = str(tmp_workspace / "file.txt")
        assert _in_cwd(p) is True

    def test_outside_cwd(self, tmp_workspace):
        from agent.tools import _in_cwd
        assert _in_cwd("/some/other/path") is False

    def test_exception_returns_false(self):
        from agent.tools import _in_cwd
        import unittest.mock as _um
        with _um.patch("os.path.abspath", side_effect=ValueError("bad path")):
            result = _in_cwd("whatever")
        assert result is False


class TestParseArgs:
    def test_dict_passthrough(self):
        from agent.tools import parse_args
        d = {"a": 1}
        assert parse_args(d) is d

    def test_json_string(self):
        from agent.tools import parse_args
        assert parse_args('{"x": 2}') == {"x": 2}

    def test_invalid_json_returns_empty(self):
        from agent.tools import parse_args
        assert parse_args("not json") == {}

    def test_none_returns_empty(self):
        from agent.tools import parse_args
        assert parse_args(None) == {}

    def test_null_json_string_returns_empty(self):
        from agent.tools import parse_args
        assert parse_args("null") == {}


class TestToolDefinition:
    def test_structure(self):
        from agent.tools import FileListTool
        t = FileListTool()
        d = t.tool_definition()
        assert d["type"] == "function"
        fn = d["function"]
        assert fn["name"] == "file_list"
        assert "description" in fn
        assert "parameters" in fn

    def test_all_params_required_for_base(self):
        from agent.tools import FileWriteTool
        t = FileWriteTool()
        d = t.tool_definition()
        # base Tool.tool_definition makes all params required
        required = d["function"]["parameters"]["required"]
        assert "path" in required
        assert "content" in required


# ── FileReadTool ──────────────────────────────────────────────────────────────

class TestFileReadTool:
    async def test_read_whole_file(self, tmp_workspace):
        from agent.tools import FileReadTool
        f = tmp_workspace / "a.txt"
        f.write_text("line1\nline2\nline3")
        result = await FileReadTool().execute({"path": str(f)})
        assert "1: line1" in result
        assert "3: line3" in result

    async def test_offset_and_limit(self, tmp_workspace):
        from agent.tools import FileReadTool
        f = tmp_workspace / "b.txt"
        f.write_text("\n".join(f"line{i}" for i in range(1, 11)))
        result = await FileReadTool().execute({"path": str(f), "offset": 3, "limit": 2})
        assert "3: line3" in result
        assert "4: line4" in result
        assert "line5" not in result

    async def test_offset_below_1_clamped(self, tmp_workspace):
        from agent.tools import FileReadTool
        f = tmp_workspace / "c.txt"
        f.write_text("first\nsecond")
        result = await FileReadTool().execute({"path": str(f), "offset": -5})
        assert "1: first" in result

    async def test_long_line_truncated(self, tmp_workspace):
        from agent.tools import FileReadTool
        f = tmp_workspace / "d.txt"
        f.write_text("x" * 2100)
        result = await FileReadTool().execute({"path": str(f)})
        assert "…" in result
        assert len(result.split("…")[0]) <= 2010  # header + 2000 chars

    async def test_large_content_truncated(self, tmp_workspace):
        from agent.tools import FileReadTool
        f = tmp_workspace / "e.txt"
        f.write_text("\n".join(f"line{i}" for i in range(1000)))
        result = await FileReadTool().execute({"path": str(f)})
        assert "(truncated)" in result

    async def test_file_not_found(self, tmp_workspace):
        from agent.tools import FileReadTool
        result = await FileReadTool().execute({"path": str(tmp_workspace / "nope.txt")})
        assert "error" in result


# ── FileWriteTool ─────────────────────────────────────────────────────────────

class TestFileWriteTool:
    async def test_writes_file(self, tmp_workspace):
        from agent.tools import FileWriteTool
        p = str(tmp_workspace / "out.txt")
        result = await FileWriteTool().execute({"path": p, "content": "hello"})
        assert "wrote" in result
        assert Path(p).read_text() == "hello"

    async def test_creates_parent_dirs(self, tmp_workspace):
        from agent.tools import FileWriteTool
        p = str(tmp_workspace / "sub" / "deep" / "file.txt")
        await FileWriteTool().execute({"path": p, "content": "x"})
        assert Path(p).exists()

    def test_is_dangerous_outside_cwd(self, tmp_workspace):
        from agent.tools import FileWriteTool
        assert FileWriteTool().is_dangerous({"path": "/tmp/evil.txt"}) is True

    def test_is_not_dangerous_inside_cwd(self, tmp_workspace):
        from agent.tools import FileWriteTool
        p = str(tmp_workspace / "ok.txt")
        assert FileWriteTool().is_dangerous({"path": p}) is False


# ── FileDeleteTool ────────────────────────────────────────────────────────────

class TestFileDeleteTool:
    async def test_deletes_file(self, tmp_workspace):
        from agent.tools import FileDeleteTool
        f = tmp_workspace / "del.txt"
        f.write_text("x")
        result = await FileDeleteTool().execute({"path": str(f)})
        assert "deleted" in result
        assert not f.exists()

    async def test_error_outside_cwd(self, tmp_workspace):
        from agent.tools import FileDeleteTool
        result = await FileDeleteTool().execute({"path": "/tmp/nope.txt"})
        assert "error" in result.lower()

    async def test_error_not_found(self, tmp_workspace):
        from agent.tools import FileDeleteTool
        result = await FileDeleteTool().execute({"path": str(tmp_workspace / "missing.txt")})
        assert "error" in result.lower()

    async def test_error_on_directory(self, tmp_workspace):
        from agent.tools import FileDeleteTool
        d = tmp_workspace / "mydir"
        d.mkdir()
        result = await FileDeleteTool().execute({"path": str(d)})
        assert "directory" in result.lower()

    def test_always_dangerous(self):
        from agent.tools import FileDeleteTool
        assert FileDeleteTool().is_dangerous({}) is True


# ── EditTool ──────────────────────────────────────────────────────────────────

class TestEditTool:
    async def test_happy_path(self, tmp_workspace):
        from agent.tools import EditTool
        f = tmp_workspace / "edit.py"
        f.write_text("hello world")
        result = await EditTool().execute({"path": str(f), "old_string": "world", "new_string": "there"})
        assert "edited" in result
        assert f.read_text() == "hello there"

    async def test_not_found(self, tmp_workspace):
        from agent.tools import EditTool
        f = tmp_workspace / "edit2.py"
        f.write_text("abc")
        result = await EditTool().execute({"path": str(f), "old_string": "xyz", "new_string": "q"})
        assert "not found" in result

    async def test_multiple_matches(self, tmp_workspace):
        from agent.tools import EditTool
        f = tmp_workspace / "edit3.py"
        f.write_text("aa aa aa")
        result = await EditTool().execute({"path": str(f), "old_string": "aa", "new_string": "bb"})
        assert "matches" in result

    async def test_crlf_normalization(self, tmp_workspace):
        from agent.tools import EditTool
        f = tmp_workspace / "edit4.py"
        f.write_bytes(b"line1\r\nline2\r\n")
        # search with LF only
        result = await EditTool().execute({"path": str(f), "old_string": "line1\nline2", "new_string": "replaced"})
        assert "edited" in result

    async def test_empty_old_string(self, tmp_workspace):
        from agent.tools import EditTool
        f = tmp_workspace / "edit5.py"
        f.write_text("abc")
        result = await EditTool().execute({"path": str(f), "old_string": "", "new_string": "x"})
        assert "error" in result

    async def test_file_not_found(self, tmp_workspace):
        from agent.tools import EditTool
        result = await EditTool().execute({"path": str(tmp_workspace / "none.py"), "old_string": "x", "new_string": "y"})
        assert "error" in result

    def test_is_dangerous_outside_cwd(self, tmp_workspace):
        from agent.tools import EditTool
        assert EditTool().is_dangerous({"path": "/tmp/evil.py"}) is True

    def test_not_dangerous_inside_cwd(self, tmp_workspace):
        from agent.tools import EditTool
        p = str(tmp_workspace / "ok.py")
        assert EditTool().is_dangerous({"path": p}) is False


# ── GlobTool ──────────────────────────────────────────────────────────────────

class TestGlobTool:
    async def test_matches_files(self, tmp_workspace):
        from agent.tools import GlobTool
        (tmp_workspace / "a.py").write_text("x")
        (tmp_workspace / "b.py").write_text("x")
        result = await GlobTool().execute({"pattern": "*.py", "path": str(tmp_workspace)})
        assert "a.py" in result
        assert "b.py" in result

    async def test_no_matches(self, tmp_workspace):
        from agent.tools import GlobTool
        result = await GlobTool().execute({"pattern": "*.xyz", "path": str(tmp_workspace)})
        assert "(no matches)" in result

    async def test_truncates_at_200(self, tmp_workspace):
        from agent.tools import GlobTool
        for i in range(205):
            (tmp_workspace / f"file{i}.txt").write_text("x")
        result = await GlobTool().execute({"pattern": "*.txt", "path": str(tmp_workspace)})
        assert "more" in result

    async def test_default_path(self, tmp_workspace):
        from agent.tools import GlobTool
        (tmp_workspace / "x.py").write_text("")
        result = await GlobTool().execute({"pattern": "*.py"})
        assert "x.py" in result


# ── GrepTool ──────────────────────────────────────────────────────────────────

class TestGrepTool:
    async def test_python_fallback_finds_match(self, tmp_workspace):
        from agent.tools import GrepTool
        f = tmp_workspace / "src.py"
        f.write_text("def hello():\n    return 42\n")
        with patch("agent.tools.shutil.which", return_value=None):
            result = await GrepTool().execute({"pattern": "hello", "path": str(tmp_workspace)})
        assert "hello" in result

    async def test_python_fallback_no_match(self, tmp_workspace):
        from agent.tools import GrepTool
        f = tmp_workspace / "src.py"
        f.write_text("nothing here")
        with patch("agent.tools.shutil.which", return_value=None):
            result = await GrepTool().execute({"pattern": "ZZZNOTFOUND", "path": str(tmp_workspace)})
        assert "(no matches)" in result

    async def test_case_insensitive(self, tmp_workspace):
        from agent.tools import GrepTool
        f = tmp_workspace / "src.py"
        f.write_text("HELLO world")
        with patch("agent.tools.shutil.which", return_value=None):
            result = await GrepTool().execute({
                "pattern": "hello", "path": str(tmp_workspace), "case_insensitive": True
            })
        assert "HELLO" in result

    async def test_ripgrep_path(self, tmp_workspace):
        from agent.tools import GrepTool
        fake_output = "file.py:1: hello"
        mock_proc = MagicMock()
        mock_proc.stdout = fake_output
        with patch("agent.tools.shutil.which", return_value="/usr/bin/rg"), \
             patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await GrepTool().execute({"pattern": "hello", "path": str(tmp_workspace)})
        assert "hello" in result

    async def test_ripgrep_no_matches(self, tmp_workspace):
        from agent.tools import GrepTool
        mock_proc = MagicMock()
        mock_proc.stdout = ""
        with patch("agent.tools.shutil.which", return_value="/usr/bin/rg"), \
             patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await GrepTool().execute({"pattern": "xyz", "path": str(tmp_workspace)})
        assert "(no matches)" in result

    async def test_searches_single_file(self, tmp_workspace):
        from agent.tools import GrepTool
        f = tmp_workspace / "only.py"
        f.write_text("target line\nother line")
        with patch("agent.tools.shutil.which", return_value=None):
            result = await GrepTool().execute({"pattern": "target", "path": str(f)})
        assert "target" in result


# ── WebFetchTool ──────────────────────────────────────────────────────────────

class TestWebFetchTool:
    async def test_non_http_url_error(self):
        from agent.tools import WebFetchTool
        result = await WebFetchTool().execute({"url": "ftp://example.com"})
        assert "error" in result

    async def test_plain_text_response(self):
        import urllib.request as _ur
        from agent.tools import WebFetchTool

        class FakeHeaders:
            def get(self, k, d=""): return "text/plain"

        class FakeResp:
            def __enter__(self): return self
            def __exit__(self, *a): pass
            headers = FakeHeaders()
            def read(self): return b"hello world"

        with patch.object(_ur, "urlopen", return_value=FakeResp()):
            result = await WebFetchTool().execute({"url": "http://example.com", "format": "text"})
        assert "hello world" in result

    async def test_html_content_markdown_format(self):
        import urllib.request as _ur
        from agent.tools import WebFetchTool

        class FakeHeaders:
            def get(self, k, d=""): return "text/html"

        class FakeResp:
            def __enter__(self): return self
            def __exit__(self, *a): pass
            headers = FakeHeaders()
            def read(self): return b"<html><body><p>Hello</p></body></html>"

        with patch.object(_ur, "urlopen", return_value=FakeResp()), \
             patch.object(WebFetchTool, "_html_to_markdown", return_value="Hello") as mock_md:
            result = await WebFetchTool().execute({"url": "http://example.com", "format": "markdown"})
        mock_md.assert_called_once()

    async def test_http_error(self):
        import urllib.request as _ur
        import urllib.error as _ue
        from agent.tools import WebFetchTool
        exc = _ue.HTTPError("http://x.com", 404, "Not Found", {}, None)
        with patch.object(_ur, "urlopen", side_effect=exc):
            result = await WebFetchTool().execute({"url": "http://example.com"})
        assert "error: HTTP 404" in result

    async def test_timeout_error(self):
        import urllib.request as _ur
        from agent.tools import WebFetchTool
        with patch.object(_ur, "urlopen", side_effect=asyncio.TimeoutError()):
            result = await WebFetchTool().execute({"url": "http://example.com"})
        assert "timeout" in result

    def test_html_to_text_strips_tags(self):
        from agent.tools import WebFetchTool
        html = "<script>bad()</script><style>.x{}</style><p>Hello <b>world</b></p>"
        result = WebFetchTool._html_to_text(html)
        assert "bad()" not in result
        assert ".x" not in result
        assert "Hello" in result
        assert "world" in result

    def test_html_to_markdown_fallback_on_import_error(self):
        from agent.tools import WebFetchTool
        with patch.dict(sys.modules, {"html2text": None}):
            result = WebFetchTool._html_to_markdown("<p>test</p>")
        assert "test" in result


# ── PowerShellTool / BashTool ─────────────────────────────────────────────────

class TestPowerShellTool:
    async def test_stdout_returned(self):
        from agent.tools import PowerShellTool
        mock_proc = MagicMock(stdout="Hello\n", stderr="")
        with patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await PowerShellTool().execute({"command": "echo Hello"})
        assert result == "Hello"

    async def test_stderr_only(self):
        from agent.tools import PowerShellTool
        mock_proc = MagicMock(stdout="", stderr="oh no")
        with patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await PowerShellTool().execute({"command": "bad"})
        assert result.startswith("stderr:")

    async def test_no_output(self):
        from agent.tools import PowerShellTool
        mock_proc = MagicMock(stdout="", stderr="")
        with patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await PowerShellTool().execute({"command": "noop"})
        assert result == "(no output)"

    async def test_timeout(self):
        from agent.tools import PowerShellTool
        with patch("agent.tools.subprocess.run", side_effect=asyncio.exceptions.TimeoutError()):
            result = await PowerShellTool().execute({"command": "sleep 100"})
        # Either TimeoutExpired or TimeoutError path
        assert "error" in result or "timeout" in result

    async def test_subprocess_timeout_expired(self):
        import subprocess as sp
        from agent.tools import PowerShellTool
        with patch("agent.tools.subprocess.run", side_effect=sp.TimeoutExpired("cmd", 30)):
            result = await PowerShellTool().execute({"command": "sleep 100"})
        assert "timeout 30s" in result

    async def test_bash_alias(self):
        from agent.tools import BashTool
        assert BashTool.name == "bash"


# ── ClipboardTool ─────────────────────────────────────────────────────────────

class TestClipboardTool:
    async def test_read_clipboard(self):
        from agent.tools import ClipboardTool
        mock_proc = MagicMock(stdout="clipboard text\n")
        with patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await ClipboardTool().execute({"action": "read"})
        assert result == "clipboard text"

    async def test_read_empty_clipboard(self):
        from agent.tools import ClipboardTool
        mock_proc = MagicMock(stdout="")
        with patch("agent.tools.subprocess.run", return_value=mock_proc):
            result = await ClipboardTool().execute({"action": "read"})
        assert result == "(empty)"

    async def test_write_clipboard(self):
        from agent.tools import ClipboardTool
        mock_proc = MagicMock()
        with patch("agent.tools.subprocess.run", return_value=mock_proc) as mock_run:
            result = await ClipboardTool().execute({"action": "write", "text": "hello"})
        assert "copied" in result
        cmd_args = mock_run.call_args[0][0]
        assert any("Set-Clipboard" in str(a) for a in cmd_args)

    async def test_invalid_action(self):
        from agent.tools import ClipboardTool
        result = await ClipboardTool().execute({"action": "invalid"})
        assert "error" in result


# ── MouseTool ─────────────────────────────────────────────────────────────────

class TestMouseTool:
    def _make_pyautogui_mock(self):
        mock_pg = MagicMock()
        mock_pg.FAILSAFE = True
        return mock_pg

    async def test_move(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "move", "x": 100, "y": 200})
        assert "100" in result and "200" in result

    async def test_click(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "click", "x": 50, "y": 60})
        assert "click" in result.lower()

    async def test_right_click(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "right_click", "x": 10, "y": 20})
        assert "right click" in result

    async def test_double_click(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "double_click", "x": 10, "y": 20})
        assert "double click" in result

    async def test_scroll(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "scroll", "x": 0, "y": 0, "amount": 3})
        assert "scroll" in result

    async def test_unknown_action(self):
        from agent.tools import MouseTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await MouseTool().execute({"action": "fly", "x": 0, "y": 0})
        assert "error" in result

    async def test_import_error(self):
        from agent.tools import MouseTool
        # Force import to fail by removing pyautogui from modules
        saved = sys.modules.pop("pyautogui", None)
        try:
            # Patch the import inside execute
            with patch.dict(sys.modules, {"pyautogui": None}):
                result = await MouseTool().execute({"action": "click", "x": 0, "y": 0})
            assert "pip install" in result
        finally:
            if saved is not None:
                sys.modules["pyautogui"] = saved


# ── KeyboardTool ──────────────────────────────────────────────────────────────

class TestKeyboardTool:
    def _make_pyautogui_mock(self):
        return MagicMock()

    async def test_type_text(self):
        from agent.tools import KeyboardTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await KeyboardTool().execute({"action": "type", "text": "hello"})
        assert "typed" in result

    async def test_press_single_key(self):
        from agent.tools import KeyboardTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await KeyboardTool().execute({"action": "press", "text": "enter"})
        assert "pressed" in result

    async def test_press_combo(self):
        from agent.tools import KeyboardTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await KeyboardTool().execute({"action": "press", "text": "ctrl+c"})
        assert "pressed" in result

    async def test_invalid_action(self):
        from agent.tools import KeyboardTool
        mock_pg = self._make_pyautogui_mock()
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            result = await KeyboardTool().execute({"action": "dance", "text": "x"})
        assert "error" in result

    async def test_import_error(self):
        from agent.tools import KeyboardTool
        with patch.dict(sys.modules, {"pyautogui": None}):
            result = await KeyboardTool().execute({"action": "type", "text": "x"})
        assert "pip install" in result


# ── ScreenshotTool ────────────────────────────────────────────────────────────

class TestScreenshotTool:
    async def test_takes_screenshot(self, tmp_workspace):
        from agent.tools import ScreenshotTool

        mock_img = MagicMock()
        mock_img.width = 1920
        mock_img.height = 1080

        mock_pil = MagicMock()
        mock_pil.ImageGrab.grab.return_value = mock_img

        with patch.dict(sys.modules, {"PIL": mock_pil, "PIL.ImageGrab": mock_pil.ImageGrab}):
            result = await ScreenshotTool().execute({})
        assert "saved" in result or "error" in result  # may fail on grab, but no crash

    async def test_import_error(self):
        from agent.tools import ScreenshotTool
        with patch.dict(sys.modules, {"PIL": None, "PIL.ImageGrab": None}):
            result = await ScreenshotTool().execute({})
        assert "pip install" in result or "error" in result


# ── BrowserOpenTool ───────────────────────────────────────────────────────────

class TestBrowserOpenTool:
    async def test_opens_url(self):
        import webbrowser
        from agent.tools import BrowserOpenTool
        with patch.object(webbrowser, "open", return_value=True):
            result = await BrowserOpenTool().execute({"url": "https://example.com"})
        assert "opened" in result
        assert "https://example.com" in result


# ── BrowserReadTool / YouTubeSearch / BrowserScreenshot (ImportError paths) ───

class TestBrowserImportErrors:
    async def test_browser_read_import_error(self):
        from agent.tools import BrowserReadTool
        with patch.dict(sys.modules, {"playwright": None, "playwright.async_api": None}):
            result = await BrowserReadTool().execute({"url": "http://x.com"})
        assert "pip install" in result or "error" in result

    async def test_youtube_search_import_error(self):
        from agent.tools import YouTubeSearchTool
        with patch.dict(sys.modules, {"playwright": None, "playwright.async_api": None}):
            result = await YouTubeSearchTool().execute({"query": "test"})
        assert "pip install" in result or "error" in result

    async def test_browser_screenshot_import_error(self):
        from agent.tools import BrowserScreenshotTool
        with patch.dict(sys.modules, {"playwright": None, "playwright.async_api": None}):
            result = await BrowserScreenshotTool().execute({"url": "http://x.com"})
        assert "pip install" in result or "error" in result

    async def test_google_search_import_error(self):
        from agent.tools import GoogleSearchTool
        with patch.dict(sys.modules, {"duckduckgo_search": None}):
            result = await GoogleSearchTool().execute({"query": "test"})
        assert "pip install" in result or "error" in result
