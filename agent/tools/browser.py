from __future__ import annotations

import asyncio
import datetime
import urllib.parse
from typing import Any, Dict

from agent.tools.base import Tool
from agent.tools.input import _get_screenshot_dir, _cleanup_old_screenshots


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
