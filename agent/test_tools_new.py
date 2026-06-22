"""Tests for the tools added in this change — OS-agnostic via mocking."""

from __future__ import annotations

import sys
import types
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

# ── fixtures ──────────────────────────────────────────────────────────────────


@pytest.fixture
def tmp_workspace(tmp_path, monkeypatch):
    monkeypatch.chdir(tmp_path)
    return tmp_path


# ── memory ──────────────────────────────────────────────────────────────────


class TestMemory:
    @pytest.fixture
    def mem(self, tmp_path, monkeypatch):
        from agent.tools import MemoryTool

        monkeypatch.setenv("WARDEN_MEMORY_PATH", str(tmp_path / "mem.json"))
        return MemoryTool()

    async def test_set_get_roundtrip(self, mem):
        assert "saved" in await mem.execute({"action": "set", "key": "name", "value": "Sigma"})
        assert "Sigma" in await mem.execute({"action": "get", "key": "name"})

    async def test_list_and_get_all(self, mem):
        await mem.execute({"action": "set", "key": "a", "value": "1"})
        await mem.execute({"action": "set", "key": "b", "value": "2"})
        listing = await mem.execute({"action": "list"})
        assert "a" in listing and "b" in listing
        all_notes = await mem.execute({"action": "get"})
        assert "a: 1" in all_notes and "b: 2" in all_notes

    async def test_delete(self, mem):
        await mem.execute({"action": "set", "key": "x", "value": "1"})
        assert "deleted" in await mem.execute({"action": "delete", "key": "x"})
        assert "no note" in await mem.execute({"action": "get", "key": "x"})

    async def test_clear(self, mem):
        await mem.execute({"action": "set", "key": "x", "value": "1"})
        await mem.execute({"action": "clear"})
        assert await mem.execute({"action": "list"}) == "(empty)"

    async def test_set_requires_key_and_value(self, mem):
        assert "error" in await mem.execute({"action": "set", "value": "x"})
        assert "error" in await mem.execute({"action": "set", "key": "x"})

    async def test_unknown_action(self, mem):
        assert "error" in await mem.execute({"action": "bogus"})

    async def test_persists_across_instances(self, tmp_path, monkeypatch):
        from agent.tools import MemoryTool

        monkeypatch.setenv("WARDEN_MEMORY_PATH", str(tmp_path / "mem.json"))
        await MemoryTool().execute({"action": "set", "key": "k", "value": "v"})
        assert "v" in await MemoryTool().execute({"action": "get", "key": "k"})


# ── system_info ───────────────────────────────────────────────────────────────


class TestSystemInfo:
    async def test_reports_basics(self):
        from agent.tools import SystemInfoTool

        out = await SystemInfoTool().execute({})
        assert "os:" in out
        assert "python:" in out
        assert "cpu:" in out

    def test_fmt_bytes(self):
        from agent.tools.system import _fmt_bytes

        assert _fmt_bytes(0) == "0.0B"
        assert _fmt_bytes(1536).endswith("KB")
        assert _fmt_bytes(2 * 1024**3).endswith("GB")

    def test_fmt_uptime(self):
        from agent.tools.system import _fmt_uptime

        assert _fmt_uptime(90) == "1m"
        assert _fmt_uptime(3700) == "1h 1m"
        assert _fmt_uptime(90061).startswith("1d")


# ── http_request ──────────────────────────────────────────────────────────────


class _FakeResp:
    def __init__(self, status=200, reason="OK", body="hello"):
        self.status = status
        self.reason = reason
        self._body = body.encode("utf-8")

    def read(self, n=-1):
        return self._body

    def __enter__(self):
        return self

    def __exit__(self, *a):
        return False


class TestHttpRequest:
    async def test_get_ok(self, monkeypatch):
        from agent.tools import HttpRequestTool

        monkeypatch.setattr("urllib.request.urlopen", lambda *a, **k: _FakeResp(200, "OK", "world"))
        out = await HttpRequestTool().execute({"url": "https://api.example.com"})
        assert "HTTP 200 OK" in out
        assert "world" in out

    async def test_post_body(self, monkeypatch):
        from agent.tools import HttpRequestTool

        seen = {}

        def fake_urlopen(req, *a, **k):
            seen["method"] = req.get_method()
            seen["data"] = req.data
            return _FakeResp(201, "Created", "{}")

        monkeypatch.setattr("urllib.request.urlopen", fake_urlopen)
        out = await HttpRequestTool().execute({"url": "https://api.example.com", "method": "POST", "body": '{"a":1}'})
        assert "HTTP 201" in out
        assert seen["method"] == "POST"
        assert seen["data"] == b'{"a":1}'

    async def test_bad_url(self):
        from agent.tools import HttpRequestTool

        assert "error" in await HttpRequestTool().execute({"url": "ftp://nope"})

    async def test_bad_method(self):
        from agent.tools import HttpRequestTool

        out = await HttpRequestTool().execute({"url": "https://x.com", "method": "FOO"})
        assert "unsupported method" in out

    async def test_bad_headers(self):
        from agent.tools import HttpRequestTool

        out = await HttpRequestTool().execute({"url": "https://x.com", "headers": "not-json"})
        assert "error" in out


# ── image_locate ──────────────────────────────────────────────────────────────


def _box(left, top, width, height):
    b = types.SimpleNamespace(left=left, top=top, width=width, height=height)
    return b


class TestImageLocate:
    async def test_missing_path(self):
        from agent.tools import ImageLocateTool

        assert "required" in await ImageLocateTool().execute({})

    async def test_nonexistent_file(self, tmp_path):
        from agent.tools import ImageLocateTool

        out = await ImageLocateTool().execute({"image": str(tmp_path / "nope.png")})
        assert "not found" in out

    async def test_found(self, tmp_path):
        from agent.tools import ImageLocateTool

        f = tmp_path / "t.png"
        f.write_bytes(b"x")
        mock_pg = MagicMock()
        mock_pg.locateOnScreen.return_value = _box(10, 20, 30, 40)
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            out = await ImageLocateTool().execute({"image": str(f)})
        # center of (10,20,30,40) = (25, 40)
        assert "found at (25, 40)" in out

    async def test_not_found(self, tmp_path):
        from agent.tools import ImageLocateTool

        f = tmp_path / "t.png"
        f.write_bytes(b"x")
        mock_pg = MagicMock()
        mock_pg.locateOnScreen.return_value = None
        with patch.dict(sys.modules, {"pyautogui": mock_pg}):
            out = await ImageLocateTool().execute({"image": str(f)})
        assert out == "not found"


# ── ocr ───────────────────────────────────────────────────────────────────────


class TestOcr:
    async def test_non_windows_guard(self, monkeypatch):
        from agent.tools import screen

        monkeypatch.setattr(screen, "_is_windows", lambda: False)
        out = await screen.OcrTool().execute({})
        assert "Windows-only" in out

    async def test_screen_grab_success(self, monkeypatch, tmp_path):
        from agent.tools import screen

        f = tmp_path / "grab.png"
        f.write_bytes(b"x")
        monkeypatch.setattr(screen, "_is_windows", lambda: True)
        monkeypatch.setattr(screen, "_capture_region", lambda region: str(f))

        async def fake_ocr(path):
            return "recognized text"

        monkeypatch.setattr(screen, "_ocr_image", fake_ocr)
        out = await screen.OcrTool().execute({})
        assert "recognized text" in out

    async def test_image_not_found(self, monkeypatch, tmp_path):
        from agent.tools import screen

        monkeypatch.setattr(screen, "_is_windows", lambda: True)
        out = await screen.OcrTool().execute({"image": str(tmp_path / "missing.png")})
        assert "not found" in out


# ── wait_for ──────────────────────────────────────────────────────────────────


class TestWaitFor:
    async def test_bad_type(self):
        from agent.tools import WaitForTool

        assert "error" in await WaitForTool().execute({"type": "nope", "target": "x"})

    async def test_missing_target(self):
        from agent.tools import WaitForTool

        assert "error" in await WaitForTool().execute({"type": "window", "target": ""})

    async def test_window_found(self, monkeypatch):
        from agent.tools import screen, window

        async def fake_enum():
            return [{"title": "My Notepad", "hwnd": 1, "pid": 2}]

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        out = await screen.WaitForTool().execute({"type": "window", "target": "notepad", "timeout": 2})
        assert "found after" in out

    async def test_text_found(self, monkeypatch, tmp_path):
        from agent.tools import screen

        monkeypatch.setattr(screen, "_capture_region", lambda region: "x.png")

        async def fake_ocr(path):
            return "Welcome to Warden"

        monkeypatch.setattr(screen, "_ocr_image", fake_ocr)
        out = await screen.WaitForTool().execute({"type": "text", "target": "warden", "timeout": 2})
        assert "found after" in out

    async def test_timeout(self, monkeypatch):
        from agent.tools import screen, window

        async def fake_enum():
            return []

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        out = await screen.WaitForTool().execute({"type": "window", "target": "ghost", "timeout": 0.3, "interval": 0.1})
        assert "timeout" in out


# ── window tools ──────────────────────────────────────────────────────────────


class TestWindowTools:
    def test_match_window_by_title(self):
        from agent.tools.window import _match_window

        wins = [{"title": "Chrome", "hwnd": 1}, {"title": "Notepad", "hwnd": 2}]
        assert _match_window(wins, "note", None)["hwnd"] == 2
        assert _match_window(wins, "missing", None) is None

    def test_match_window_by_hwnd(self):
        from agent.tools.window import _match_window

        wins = [{"title": "Chrome", "hwnd": 1}, {"title": "Notepad", "hwnd": 2}]
        assert _match_window(wins, None, 1)["title"] == "Chrome"

    async def test_window_list_non_windows(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: False)
        assert "Windows-only" in await window.WindowListTool().execute({})

    async def test_window_list_format(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: True)

        async def fake_enum():
            return [{"pid": 11, "title": "Notepad", "hwnd": 99, "x": 0, "y": 0, "w": 800, "h": 600}]

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        out = await window.WindowListTool().execute({})
        assert "Notepad" in out and "99" in out

    async def test_window_focus(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: True)

        async def fake_enum():
            return [{"pid": 11, "title": "Notepad", "hwnd": 99, "x": 0, "y": 0, "w": 1, "h": 1}]

        ran = {}

        async def fake_run(script, timeout=15):
            ran["script"] = script
            return ""

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        monkeypatch.setattr(window, "_run_ps", fake_run)
        out = await window.WindowFocusTool().execute({"title": "notepad"})
        assert "focused" in out
        assert "99" in ran["script"]

    async def test_window_focus_not_found(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: True)

        async def fake_enum():
            return []

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        out = await window.WindowFocusTool().execute({"title": "ghost"})
        assert "not found" in out

    async def test_window_manage_close(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: True)

        async def fake_enum():
            return [{"pid": 11, "title": "Notepad", "hwnd": 99, "x": 0, "y": 0, "w": 1, "h": 1}]

        async def fake_run(script, timeout=15):
            return ""

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        monkeypatch.setattr(window, "_run_ps", fake_run)
        out = await window.WindowManageTool().execute({"action": "close", "hwnd": 99})
        assert "close" in out

    async def test_window_manage_resize_needs_dims(self, monkeypatch):
        from agent.tools import window

        monkeypatch.setattr(window, "_is_windows", lambda: True)

        async def fake_enum():
            return [{"pid": 11, "title": "N", "hwnd": 99, "x": 0, "y": 0, "w": 0, "h": 0}]

        monkeypatch.setattr(window, "_enumerate_windows", fake_enum)
        out = await window.WindowManageTool().execute({"action": "resize", "hwnd": 99})
        assert "error" in out


# ── notify ────────────────────────────────────────────────────────────────────


class TestNotify:
    async def test_non_windows_guard(self, monkeypatch):
        from agent.tools import system

        monkeypatch.setattr(system, "_is_windows", lambda: False)
        assert "Windows-only" in await system.NotifyTool().execute({"message": "hi"})

    async def test_empty_message(self, monkeypatch):
        from agent.tools import system

        monkeypatch.setattr(system, "_is_windows", lambda: True)
        assert "error" in await system.NotifyTool().execute({"message": ""})

    async def test_fires_process(self, monkeypatch):
        from agent.tools import system

        monkeypatch.setattr(system, "_is_windows", lambda: True)
        called = {}

        def fake_popen(cmd, **kwargs):
            called["cmd"] = cmd
            return MagicMock()

        monkeypatch.setattr(system.subprocess, "Popen", fake_popen)
        out = await system.NotifyTool().execute({"message": "done", "title": "T"})
        assert "notified" in out
        assert called["cmd"][0] == "powershell"


# ── browser_click / browser_fill ───────────────────────────────────────────────


class TestSelector:
    def test_css_passthrough(self):
        from agent.tools.browser import _selector

        assert _selector("#id") == "#id"
        assert _selector(".cls") == ".cls"
        assert _selector("[name=q]") == "[name=q]"
        assert _selector("//button") == "//button"

    def test_explicit_text(self):
        from agent.tools.browser import _selector

        assert _selector("text=Login") == "text=Login"

    def test_plain_text_wrapped(self):
        from agent.tools.browser import _selector

        assert _selector("Login") == "text=Login"
        assert _selector("Sign in") == "text=Sign in"


def _fake_page():
    page = MagicMock()
    page.is_closed.return_value = False
    page.url = "https://example.com/after"
    page.goto = AsyncMock()
    page.click = AsyncMock()
    page.fill = AsyncMock()
    page.press = AsyncMock()
    page.wait_for_load_state = AsyncMock()
    page.evaluate = AsyncMock(return_value="page body text")
    return page


class TestBrowserInteract:
    async def test_click(self, monkeypatch):
        from agent.tools import browser

        page = _fake_page()
        monkeypatch.setattr(browser, "_get_page", AsyncMock(return_value=page))
        out = await browser.BrowserClickTool().execute({"selector": "#go", "url": "https://example.com"})
        assert "clicked: #go" in out
        page.goto.assert_awaited()
        page.click.assert_awaited_with("#go", timeout=15000)

    async def test_click_requires_selector(self):
        from agent.tools import browser

        assert "required" in await browser.BrowserClickTool().execute({"selector": ""})

    async def test_fill_with_submit(self, monkeypatch):
        from agent.tools import browser

        page = _fake_page()
        monkeypatch.setattr(browser, "_get_page", AsyncMock(return_value=page))
        out = await browser.BrowserFillTool().execute({"selector": "#q", "value": "hello", "submit": True})
        assert "filled: #q" in out
        page.fill.assert_awaited()
        page.press.assert_awaited()

    async def test_fill_requires_selector(self):
        from agent.tools import browser

        assert "required" in await browser.BrowserFillTool().execute({"selector": "", "value": "x"})


# ── tool definitions / registry wiring ──────────────────────────────────────────


class TestRegistration:
    def test_all_new_tools_registered(self):
        from agent.tools import REGISTRY

        for name in [
            "image_locate",
            "wait_for",
            "browser_click",
            "browser_fill",
            "window_list",
            "window_focus",
            "window_manage",
            "system_info",
            "http_request",
            "memory",
            "ocr",
            "notify",
        ]:
            assert name in REGISTRY, f"{name} not registered"

    def test_optional_params_not_all_required(self):
        from agent.tools import REGISTRY

        # image_locate has an optional confidence param
        req = REGISTRY["image_locate"].tool_definition()["function"]["parameters"]["required"]
        assert req == ["image"]
