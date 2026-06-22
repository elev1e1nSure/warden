"""Tests for HTTP request tool."""

from __future__ import annotations

from unittest.mock import MagicMock, patch


class TestHttpRequestTool:
    async def test_invalid_url(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        result = await tool.execute({"url": "ftp://example.com"})
        assert "error" in result.lower()

    async def test_unsupported_method(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        result = await tool.execute({"url": "http://example.com", "method": "FOOBAR"})
        assert "unsupported" in result.lower()

    async def test_headers_string_parsing(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        result = await tool.execute({"url": "http://example.com", "headers": '{"X-Custom": "value"}'})
        # Should not error on valid JSON headers
        assert "error" not in result.lower() or "HTTP" in result

    async def test_headers_invalid_json(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        result = await tool.execute({"url": "http://example.com", "headers": "not json"})
        assert "error" in result.lower()

    async def test_get_success(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        mock_resp = MagicMock()
        mock_resp.status = 200
        mock_resp.reason = "OK"
        mock_resp.read.return_value = b"hello"
        with patch("asyncio.to_thread", return_value=(200, "OK", "hello")):
            result = await tool.execute({"url": "http://example.com"})
        assert "200" in result
        assert "hello" in result

    async def test_url_error(self):
        import urllib.error

        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        with patch("asyncio.to_thread", side_effect=urllib.error.URLError("boom")):
            result = await tool.execute({"url": "http://example.com"})
        assert "error" in result.lower()

    async def test_timeout(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        with patch("asyncio.to_thread", side_effect=TimeoutError()):
            result = await tool.execute({"url": "http://example.com"})
        assert "timeout" in result.lower()

    async def test_ssrf_blocking(self):
        from agent.tools.http import HttpRequestTool

        tool = HttpRequestTool()
        for unsafe_url in [
            "http://127.0.0.1",
            "http://10.0.0.1",
            "http://172.16.0.1",
            "http://192.168.1.1",
            "http://169.254.169.254",
            "http://[::1]",
            "http://localhost",
            "file:///etc/passwd",
        ]:
            result = await tool.execute({"url": unsafe_url})
            assert "blocked" in result.lower() or "error" in result.lower()
