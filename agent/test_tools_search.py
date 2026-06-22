"""Tests for search tools."""

from __future__ import annotations

from unittest.mock import patch


class TestGoogleSearchTool:
    async def test_no_results(self):
        from agent.tools.search import GoogleSearchTool

        tool = GoogleSearchTool()
        with patch("duckduckgo_search.DDGS") as mock_ddgs:
            mock_ddgs.return_value.__enter__.return_value.text.return_value = []
            result = await tool.execute({"query": "xyz123nonexistent"})
        assert "no results" in result.lower()

    async def test_results_returned(self):
        from agent.tools.search import GoogleSearchTool

        tool = GoogleSearchTool()
        with patch("duckduckgo_search.DDGS") as mock_ddgs:
            mock_ddgs.return_value.__enter__.return_value.text.return_value = [
                {"title": "Test", "href": "http://example.com", "body": "Snippet"}
            ]
            result = await tool.execute({"query": "test"})
        assert "Test" in result
        assert "example.com" in result


class TestWebFetchTool:
    async def test_invalid_url(self):
        from agent.tools.search import WebFetchTool

        tool = WebFetchTool()
        result = await tool.execute({"url": "ftp://example.com"})
        assert "error" in result.lower()

    async def test_html_to_text(self):
        from agent.tools.search import WebFetchTool

        text = WebFetchTool._html_to_text("<p>Hello</p><script>alert(1)</script>")
        assert "Hello" in text
        assert "alert" not in text

    async def test_html_to_markdown_fallback(self):
        from agent.tools.search import WebFetchTool

        with patch.dict("sys.modules", {"html2text": None}):
            import sys

            old = sys.modules.pop("html2text", None)
            try:
                text = WebFetchTool._html_to_markdown("<p>Hello</p>")
                assert "Hello" in text
            finally:
                if old:
                    sys.modules["html2text"] = old
