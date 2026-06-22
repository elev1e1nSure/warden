from __future__ import annotations

import asyncio
import datetime
import urllib.parse
from typing import Any, Dict

from agent.tools.base import Tool
from agent.tools.input import _get_screenshot_dir, _cleanup_old_screenshots


# ── persistent interactive session (shared by browser_click / browser_fill) ──
# Keeps one browser page alive across calls so clicks and fills compound,
# unlike the stateless read/screenshot tools that open a fresh browser each time.
_SESSION: Dict[str, Any] = {"pw": None, "browser": None, "page": None}


async def _close_session() -> None:
	"""Close the persistent browser and playwright instances."""
	browser = _SESSION.get("browser")
	if browser is not None:
		try:
			await browser.close()
		except Exception:
			pass
	pw = _SESSION.get("pw")
	if pw is not None:
		try:
			await pw.stop()
		except Exception:
			pass
	_SESSION.update(pw=None, browser=None, page=None)


async def _get_page():
	"""Return the live interactive page, creating the browser lazily."""
	from playwright.async_api import async_playwright

	page = _SESSION.get("page")
	if page is not None and not page.is_closed():
		return page
	# Clean up a stale session before creating a new one.
	await _close_session()
	pw = await async_playwright().start()
	browser = await pw.chromium.launch(headless=True)
	ctx = await browser.new_context(locale="en-US")
	page = await ctx.new_page()
	_SESSION.update(pw=pw, browser=browser, page=page)
	return page


def _selector(raw: str) -> str:
	"""Accept a CSS/XPath selector or plain text.

	If the string contains no selector metacharacters it's treated as visible
	text and matched via Playwright's text engine. Pass an explicit
	`text=...` or a CSS selector to be unambiguous.
	"""
	raw = raw.strip()
	if raw.startswith(("#", ".", "[", "//")) or raw.startswith("text="):
		return raw
	if any(c in raw for c in "#.[]>=:") or "//" in raw:
		return raw
	# looks like human text → match by visible text
	return f"text={raw}"


async def _page_snapshot(page) -> str:
	try:
		text = await page.evaluate("() => document.body ? document.body.innerText.slice(0, 800) : ''")
	except Exception:
		text = ""
	url = page.url
	out = f"url: {url}"
	if text and text.strip():
		out += "\n" + text.strip()
	return out[:1500]


class BrowserOpenTool(Tool):
	name = "browser_open"
	description = (
		"Open a URL in the user's browser. "
		"Does not read or control the page. For reading and checking web pages use browser_read or browser_screenshot."
	)
	params = {"url": {"type": "string", "description": "URL"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		url = args.get("url", "")
		try:
			import webbrowser
			await asyncio.to_thread(webbrowser.open, url)
			return f"opened: {url}"
		except Exception as e:
			return f"error: {e}"


class BrowserReadTool(Tool):
	name = "browser_read"
	description = (
		"Read page content via Playwright: text and list of links. "
		"Use for site navigation, page checks and data extraction without opening a window for the user."
	)
	params = {"url": {"type": "string", "description": "URL"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		url = args.get("url", "")
		try:
			from playwright.async_api import async_playwright
			async with async_playwright() as pw:
				browser = await pw.chromium.launch(headless=True)
				ctx = await browser.new_context(locale="en-US")
				page = await ctx.new_page()
				await page.goto(url, timeout=20000)
				for sel in [
					'button:has-text("Accept all")',
					'button:has-text("Reject all")',
					'button[aria-label*="Accept"]',
					'#L2AGLb',
					'button:has-text("Agree")',
				]:
					try:
						await page.click(sel, timeout=1500)
						break
					except Exception:
						pass
				try:
					await page.wait_for_load_state("networkidle", timeout=5000)
				except Exception:
					pass
				data = await page.evaluate("""
					() => {
						const text = document.body.innerText.slice(0, 2000);
						const links = [...document.querySelectorAll('a[href]')]
							.map(a => ({text: (a.innerText || a.title || '').trim().slice(0, 80), url: a.href}))
							.filter(l => l.text && l.url && !l.url.startsWith('javascript') && !l.url.startsWith('mailto'))
							.slice(0, 40);
						return {text, links};
					}
				""")
				await browser.close()
			out = data["text"]
			if data["links"]:
				out += "\n\nLinks:\n" + "\n".join(f"• {l['text']}: {l['url']}" for l in data["links"])
			return out[:3000]
		except ImportError:
			return "error: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"error: {e}"


class YouTubeSearchTool(Tool):
	name = "youtube_search"
	description = (
		"Search for videos on YouTube. Returns a list of videos with direct links. "
		"Use instead of google_search for video search."
	)
	params = {"query": {"type": "string", "description": "Search query"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		query = args.get("query", "")
		try:
			from playwright.async_api import async_playwright
			async with async_playwright() as pw:
				browser = await pw.chromium.launch(headless=True)
				try:
					ctx = await browser.new_context(locale="en-US")
					page = await ctx.new_page()
					await page.goto(
						f"https://www.youtube.com/results?search_query={urllib.parse.quote(query)}",
						timeout=20000,
					)
					for sel in [
						'button:has-text("Accept all")',
						'button:has-text("Reject all")',
						'button[aria-label*="Accept"]',
					]:
						try:
							await page.click(sel, timeout=2000)
							break
						except Exception:
							pass
					try:
						await page.wait_for_selector("ytd-video-renderer", timeout=8000)
					except Exception:
						pass
					results = await page.evaluate("""
						() => {
							const items = document.querySelectorAll('ytd-video-renderer');
							return [...items].slice(0, 8).map(item => {
								const a = item.querySelector('a#video-title');
								const meta = item.querySelector('#metadata-line');
								return {
									title: (a?.textContent || '').trim(),
									url: a?.href || '',
									meta: (meta?.textContent || '').trim().replace(/\\s+/g, ' ')
								};
							}).filter(r => r.title && r.url);
						}
					""")
				finally:
					await browser.close()
			if not results:
				return "no results"
			return "\n".join(
				f"{i+1}. {r['title']}{(' · ' + r['meta']) if r['meta'] else ''}\n   {r['url']}"
				for i, r in enumerate(results)
			)
		except ImportError:
			return "error: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"error: {e}"


class BrowserScreenshotTool(Tool):
	name = "browser_screenshot"
	description = "Take a screenshot of a web page in the background via Playwright and return the file path."
	params = {"url": {"type": "string", "description": "URL"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		url = args.get("url", "")
		try:
			from playwright.async_api import async_playwright
			screenshot_dir = _get_screenshot_dir()
			_cleanup_old_screenshots(screenshot_dir, max_age_seconds=300)
			name = screenshot_dir / f"browser_{datetime.datetime.now():%Y%m%d_%H%M%S}.png"
			async with async_playwright() as pw:
				browser = await pw.chromium.launch(headless=True)
				page = await browser.new_page()
				await page.goto(url, timeout=20000)
				await page.screenshot(path=str(name), full_page=True)
				await browser.close()
			return f"saved: {name}"
		except ImportError:
			return "error: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"error: {e}"


class BrowserClickTool(Tool):
	name = "browser_click"
	description = (
		"Click an element on the interactive browser page (Playwright). "
		"Optionally navigate to a URL first. "
		"selector: a CSS/XPath selector or plain visible text. "
		"The session persists, so clicks and fills build on each other."
	)
	params = {
		"selector": {"type": "string", "description": "CSS/XPath selector or visible text to click"},
		"url": {"type": "string", "description": "URL to navigate to first (optional)"},
		"timeout": {"type": "integer", "description": "Timeout in seconds for the action (default 15)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = ["selector"]
		return d

	async def execute(self, args: Dict[str, Any]) -> str:
		selector = str(args.get("selector", "")).strip()
		if not selector:
			return "error: selector is required"
		url = str(args.get("url", "")).strip()
		timeout_ms = min(int(args.get("timeout", 15)), 60) * 1000
		try:
			page = await _get_page()
			if url:
				await page.goto(url, timeout=timeout_ms)
			await page.click(_selector(selector), timeout=timeout_ms)
			try:
				await page.wait_for_load_state("networkidle", timeout=5000)
			except Exception:
				pass
			snap = await _page_snapshot(page)
			return f"clicked: {selector}\n{snap}"
		except ImportError:
			return "error: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"error: {e}"


class BrowserFillTool(Tool):
	name = "browser_fill"
	description = (
		"Fill an input field on the interactive browser page (Playwright). "
		"Optionally navigate to a URL first and/or press Enter after filling. "
		"selector: a CSS/XPath selector or plain visible text/label. "
		"The session persists across calls."
	)
	params = {
		"selector": {"type": "string", "description": "CSS/XPath selector or label/placeholder text of the field"},
		"value": {"type": "string", "description": "Text to type into the field"},
		"url": {"type": "string", "description": "URL to navigate to first (optional)"},
		"submit": {"type": "boolean", "description": "Press Enter after filling (optional)"},
		"timeout": {"type": "integer", "description": "Timeout in seconds for the action (default 15)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = ["selector", "value"]
		return d

	async def execute(self, args: Dict[str, Any]) -> str:
		selector = str(args.get("selector", "")).strip()
		if not selector:
			return "error: selector is required"
		value = str(args.get("value", ""))
		url = str(args.get("url", "")).strip()
		submit = bool(args.get("submit", False))
		timeout_ms = min(int(args.get("timeout", 15)), 60) * 1000
		try:
			page = await _get_page()
			if url:
				await page.goto(url, timeout=timeout_ms)
			await page.fill(_selector(selector), value, timeout=timeout_ms)
			if submit:
				await page.press(_selector(selector), "Enter")
				try:
					await page.wait_for_load_state("networkidle", timeout=5000)
				except Exception:
					pass
			snap = await _page_snapshot(page)
			return f"filled: {selector}\n{snap}"
		except ImportError:
			return "error: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"error: {e}"
