"""Tests for browser tools via mocking."""
from __future__ import annotations

from unittest.mock import AsyncMock, MagicMock, patch

import pytest


class TestBrowserOpenTool:
	async def test_open_success(self):
		from agent.tools.browser import BrowserOpenTool
		tool = BrowserOpenTool()
		with patch("asyncio.to_thread", return_value=None):
			result = await tool.execute({"url": "http://example.com"})
		assert "opened" in result.lower()

	async def test_open_error(self):
		from agent.tools.browser import BrowserOpenTool
		tool = BrowserOpenTool()
		with patch("asyncio.to_thread", side_effect=RuntimeError("boom")):
			result = await tool.execute({"url": "http://example.com"})
		assert "error" in result.lower()


class TestBrowserClickTool:
	async def test_empty_selector(self):
		from agent.tools.browser import BrowserClickTool
		tool = BrowserClickTool()
		result = await tool.execute({"selector": ""})
		assert "error" in result.lower()


class TestBrowserFillTool:
	async def test_empty_selector(self):
		from agent.tools.browser import BrowserFillTool
		tool = BrowserFillTool()
		result = await tool.execute({"selector": "", "value": "hello"})
		assert "error" in result.lower()


class TestSelector:
	def test_css_selector(self):
		from agent.tools.browser import _selector
		assert _selector("#id") == "#id"
		assert _selector(".class") == ".class"

	def test_text_selector(self):
		from agent.tools.browser import _selector
		assert _selector("Hello") == "text=Hello"

	def test_xpath_selector(self):
		from agent.tools.browser import _selector
		assert _selector("//div") == "//div"
