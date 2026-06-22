"""Tests for screen tools via mocking."""

from __future__ import annotations

from unittest.mock import patch


class TestImageLocateTool:
    async def test_missing_path(self):
        from agent.tools.screen import ImageLocateTool

        tool = ImageLocateTool()
        result = await tool.execute({"image": ""})
        assert "error" in result.lower()

    async def test_not_found(self, tmp_workspace):
        from agent.tools.screen import ImageLocateTool

        tool = ImageLocateTool()
        result = await tool.execute({"image": str(tmp_workspace / "missing.png")})
        assert "error" in result.lower()

    async def test_invalid_confidence(self, tmp_workspace):
        from agent.tools.screen import ImageLocateTool

        (tmp_workspace / "img.png").write_bytes(b"png")
        tool = ImageLocateTool()
        result = await tool.execute({"image": str(tmp_workspace / "img.png"), "confidence": "abc"})
        assert "error" in result.lower()

    async def test_confidence_out_of_range(self, tmp_workspace):
        from agent.tools.screen import ImageLocateTool

        (tmp_workspace / "img.png").write_bytes(b"png")
        tool = ImageLocateTool()
        result = await tool.execute({"image": str(tmp_workspace / "img.png"), "confidence": 2.0})
        assert "error" in result.lower()


class TestOcrTool:
    async def test_not_windows(self):
        from agent.tools.screen import OcrTool

        tool = OcrTool()
        with patch("agent.tools.screen._is_windows", return_value=False):
            result = await tool.execute({})
        assert "windows-only" in result.lower()

    async def test_image_not_found(self):
        from agent.tools.screen import OcrTool

        tool = OcrTool()
        with patch("agent.tools.screen._is_windows", return_value=True):
            result = await tool.execute({"image": "missing.png"})
        assert "not found" in result.lower()


class TestWaitForTool:
    async def test_invalid_type(self):
        from agent.tools.screen import WaitForTool

        tool = WaitForTool()
        result = await tool.execute({"type": "invalid", "target": "x"})
        assert "error" in result.lower()

    async def test_empty_target(self):
        from agent.tools.screen import WaitForTool

        tool = WaitForTool()
        result = await tool.execute({"type": "window", "target": ""})
        assert "error" in result.lower()

    async def test_timeout(self):
        from agent.tools.screen import WaitForTool

        tool = WaitForTool()
        with patch("time.monotonic", side_effect=[0, 0, 5]):
            with patch("asyncio.sleep", return_value=None):
                with patch.object(tool, "_check", return_value=False):
                    result = await tool.execute({"type": "window", "target": "x", "timeout": 1})
                assert "timeout" in result.lower()
