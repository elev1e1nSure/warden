"""Tests for window tools via mocking."""

from __future__ import annotations

from unittest.mock import patch


class TestWindowListTool:
    async def test_not_windows(self):
        from agent.tools.window import WindowListTool

        tool = WindowListTool()
        with patch("agent.tools.window._is_windows", return_value=False):
            result = await tool.execute({})
        assert "windows-only" in result.lower()

    async def test_empty_filter(self):
        from agent.tools.window import WindowListTool

        tool = WindowListTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch("agent.tools.window._enumerate_windows", return_value=[]):
                result = await tool.execute({})
        assert "no windows" in result.lower()

    async def test_filtered(self):
        from agent.tools.window import WindowListTool

        tool = WindowListTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch(
                "agent.tools.window._enumerate_windows",
                return_value=[
                    {"hwnd": 1, "pid": 1, "title": "Notepad", "x": 0, "y": 0, "w": 100, "h": 100},
                    {"hwnd": 2, "pid": 2, "title": "Chrome", "x": 0, "y": 0, "w": 100, "h": 100},
                ],
            ):
                result = await tool.execute({"filter": "note"})
        assert "Notepad" in result
        assert "Chrome" not in result

    async def test_timeout(self):
        import subprocess

        from agent.tools.window import WindowListTool

        tool = WindowListTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch(
                "agent.tools.window._enumerate_windows",
                side_effect=subprocess.TimeoutExpired("cmd", 15),
            ):
                result = await tool.execute({})
        assert "timeout" in result.lower()


class TestWindowFocusTool:
    async def test_not_windows(self):
        from agent.tools.window import WindowFocusTool

        tool = WindowFocusTool()
        with patch("agent.tools.window._is_windows", return_value=False):
            result = await tool.execute({"title": "x"})
        assert "windows-only" in result.lower()

    async def test_no_title_or_hwnd(self):
        from agent.tools.window import WindowFocusTool

        tool = WindowFocusTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            result = await tool.execute({})
        assert "error" in result.lower()

    async def test_window_not_found(self):
        from agent.tools.window import WindowFocusTool

        tool = WindowFocusTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch("agent.tools.window._enumerate_windows", return_value=[]):
                result = await tool.execute({"title": "missing"})
        assert "not found" in result.lower()

    def test_is_dangerous(self):
        from agent.tools.window import WindowFocusTool

        assert WindowFocusTool().is_dangerous({}) is True


class TestWindowManageTool:
    async def test_not_windows(self):
        from agent.tools.window import WindowManageTool

        tool = WindowManageTool()
        with patch("agent.tools.window._is_windows", return_value=False):
            result = await tool.execute({"action": "close", "title": "x"})
        assert "windows-only" in result.lower()

    async def test_no_title_or_hwnd(self):
        from agent.tools.window import WindowManageTool

        tool = WindowManageTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            result = await tool.execute({"action": "close"})
        assert "error" in result.lower()

    async def test_unknown_action(self):
        from agent.tools.window import WindowManageTool

        tool = WindowManageTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch(
                "agent.tools.window._enumerate_windows",
                return_value=[
                    {"hwnd": 1, "title": "x"},
                ],
            ):
                result = await tool.execute({"action": "spin", "title": "x"})
        assert "unknown" in result.lower()

    async def test_resize_invalid(self):
        from agent.tools.window import WindowManageTool

        tool = WindowManageTool()
        with patch("agent.tools.window._is_windows", return_value=True):
            with patch(
                "agent.tools.window._enumerate_windows",
                return_value=[
                    {"hwnd": 1, "title": "x", "x": 0, "y": 0, "w": 100, "h": 100},
                ],
            ):
                result = await tool.execute({"action": "resize", "title": "x", "w": 0, "h": 0})
        assert "error" in result.lower()

    def test_is_dangerous(self):
        from agent.tools.window import WindowManageTool

        assert WindowManageTool().is_dangerous({}) is True
