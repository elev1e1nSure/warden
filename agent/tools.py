import asyncio
import json
import os
import re
import subprocess
import uuid
from abc import ABC, abstractmethod
from typing import Any, Dict

_ANSI = re.compile(r'\x1b\[[0-9;]*[mGKHFJABCDsu]|\x1b\][^\x07]*\x07|\x1b=|\x1b>')


def _clean(text: str) -> str:
	"""Убирает ANSI-коды и схлопывает \r-перезаписи"""
	text = _ANSI.sub('', text)
	lines = []
	for line in text.split('\n'):
		parts = line.split('\r')
		cleaned = parts[-1].rstrip()
		if cleaned:
			lines.append(cleaned)
	return '\n'.join(lines)


def _in_cwd(path: str) -> bool:
	try:
		return os.path.abspath(path).startswith(os.getcwd())
	except Exception:
		return False


# ── база ──────────────────────────────────────────────────────────────────────

class Tool(ABC):
	name: str
	description: str
	params: Dict[str, Any]

	@abstractmethod
	async def execute(self, args: Dict[str, Any]) -> str: ...

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return False

	def to_ollama(self) -> dict:
		return {
			"type": "function",
			"function": {
				"name": self.name,
				"description": self.description,
				"parameters": {
					"type": "object",
					"properties": self.params,
					"required": list(self.params.keys()),
				},
			},
		}


# ── опасные паттерны для bash ─────────────────────────────────────────────────

_DANGER = re.compile(
	r"\b(rmdir\b|rd\b|format\b|Clear-Content|deltree\b|DROP\s+TABLE|TRUNCATE\s+TABLE|mkfs)\b"
	r"|(-[rR][fF]|-Force\b|/[Ff]\b|--force\b|-Recurse\b)",
	re.IGNORECASE,
)


# ── инструменты ───────────────────────────────────────────────────────────────

class BashTool(Tool):
	name = "bash"
	description = "Выполнить команду PowerShell. Для файлов, процессов, системы."
	params = {"command": {"type": "string", "description": "Команда PowerShell"}}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return bool(_DANGER.search(args.get("command", "")))

	async def execute(self, args: Dict[str, Any]) -> str:
		cmd = args.get("command", "")
		try:
			proc = await asyncio.to_thread(
				subprocess.run,
				["powershell", "-NonInteractive", "-NoProfile", "-Command", cmd],
				capture_output=True, text=True, timeout=30,
			)
			out = _clean((proc.stdout or "").strip())
			err = _clean((proc.stderr or "").strip())
			if not out and err:
				return f"stderr: {err[:500]}"
			if not out:
				return "(нет вывода)"
			return out[:1000] + (f"\nstderr: {err[:200]}" if err else "")
		except subprocess.TimeoutExpired:
			return "ошибка: таймаут 30с"
		except Exception as e:
			return f"ошибка: {e}"


class FileReadTool(Tool):
	name = "file_read"
	description = "Прочитать файл."
	params = {"path": {"type": "string", "description": "Путь к файлу"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		path = args.get("path", "")
		try:
			with open(path, encoding="utf-8") as f:
				content = f.read()
			if len(content) > 4000:
				return content[:4000] + "\n...(обрезано)"
			return content
		except Exception as e:
			return f"ошибка: {e}"


class FileWriteTool(Tool):
	name = "file_write"
	description = "Записать текст в файл. Создаёт папки если нужно."
	params = {
		"path": {"type": "string", "description": "Путь к файлу"},
		"content": {"type": "string", "description": "Содержимое"},
	}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return not _in_cwd(args.get("path", ""))

	async def execute(self, args: Dict[str, Any]) -> str:
		path = args.get("path", "")
		content = args.get("content", "")
		try:
			d = os.path.dirname(os.path.abspath(path))
			if d:
				os.makedirs(d, exist_ok=True)
			with open(path, "w", encoding="utf-8") as f:
				f.write(content)
			return f"записано {len(content)} символов → {path}"
		except Exception as e:
			return f"ошибка: {e}"


class FileDeleteTool(Tool):
	name = "file_delete"
	description = "Удалить файл. Работает только внутри текущей директории."
	params = {"path": {"type": "string", "description": "Путь к файлу"}}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return True

	async def execute(self, args: Dict[str, Any]) -> str:
		path = args.get("path", "")
		try:
			abs_path = os.path.abspath(path)
			if not abs_path.startswith(os.getcwd()):
				return "ошибка: нельзя удалять файлы вне текущей директории"
			if not os.path.exists(abs_path):
				return f"ошибка: не найден: {path}"
			if os.path.isdir(abs_path):
				return "ошибка: это директория — используй bash rmdir"
			os.remove(abs_path)
			return f"удалён: {path}"
		except Exception as e:
			return f"ошибка: {e}"


class FileListTool(Tool):
	name = "file_list"
	description = "Список файлов и папок в директории."
	params = {"path": {"type": "string", "description": "Директория (. для текущей)"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		path = args.get("path", ".")
		try:
			entries = sorted(os.listdir(path))
			dirs, files = [], []
			for e in entries:
				full = os.path.join(path, e)
				if os.path.isdir(full):
					dirs.append(f"[{e}]")
				else:
					size = os.path.getsize(full)
					kb = size / 1024
					files.append(f"{e} ({kb:.1f}KB)" if kb >= 1 else f"{e} ({size}B)")
			parts = []
			if dirs:
				parts.append("папки: " + "  ".join(dirs))
			if files:
				parts.append("файлы: " + "  ".join(files))
			return "\n".join(parts) if parts else "(пусто)"
		except Exception as e:
			return f"ошибка: {e}"


class ClipboardTool(Tool):
	name = "clipboard"
	description = "Читать или записывать буфер обмена."
	params = {
		"action": {"type": "string", "description": "read | write"},
		"text": {"type": "string", "description": "Текст для записи (только для write)"},
	}

	def to_ollama(self) -> dict:
		return {
			"type": "function",
			"function": {
				"name": self.name,
				"description": self.description,
				"parameters": {
					"type": "object",
					"properties": self.params,
					"required": ["action"],
				},
			},
		}

	async def execute(self, args: Dict[str, Any]) -> str:
		action = args.get("action", "read")
		try:
			import subprocess
			if action == "read":
				proc = await asyncio.to_thread(
					subprocess.run,
					["powershell", "-NoProfile", "-Command", "Get-Clipboard"],
					capture_output=True, text=True, timeout=5,
				)
				return (proc.stdout or "").strip() or "(пусто)"
			elif action == "write":
				text = args.get("text", "")
				escaped = text.replace("'", "''")
				cmd = f"Set-Clipboard -Value '{escaped}'"
				await asyncio.to_thread(
					subprocess.run,
					["powershell", "-NoProfile", "-Command", cmd],
					capture_output=True, timeout=5,
				)
				return f"скопировано в буфер: {text[:60]}"
			return "ошибка: action должен быть read или write"
		except Exception as e:
			return f"ошибка: {e}"


class ScreenshotTool(Tool):
	name = "screenshot"
	description = (
		"Сделать скриншот экрана. Возвращает путь к файлу. "
		"Используй чтобы увидеть что на экране, потом mouse/keyboard для взаимодействия."
	)
	params = {}

	async def execute(self, args: Dict[str, Any]) -> str:
		try:
			from PIL import ImageGrab
			import datetime
			name = f"screenshot_{datetime.datetime.now():%Y%m%d_%H%M%S}.png"
			img = await asyncio.to_thread(ImageGrab.grab)
			await asyncio.to_thread(img.save, name)
			return f"сохранён: {name} ({img.width}×{img.height})"
		except ImportError:
			return "ошибка: pip install Pillow"
		except Exception as e:
			return f"ошибка: {e}"


class MouseTool(Tool):
	name = "mouse"
	description = (
		"Управлять мышью: двигать, кликать, скроллить. "
		"action: move | click | right_click | double_click | scroll. "
		"Для scroll: amount — шаги (+ вниз, - вверх)."
	)
	params = {
		"action": {"type": "string", "description": "move | click | right_click | double_click | scroll"},
		"x": {"type": "integer", "description": "X координата"},
		"y": {"type": "integer", "description": "Y координата"},
		"amount": {"type": "integer", "description": "Для scroll: шаги прокрутки"},
	}

	def to_ollama(self) -> dict:
		return {
			"type": "function",
			"function": {
				"name": self.name,
				"description": self.description,
				"parameters": {
					"type": "object",
					"properties": self.params,
					"required": ["action", "x", "y"],
				},
			},
		}

	async def execute(self, args: Dict[str, Any]) -> str:
		action = args.get("action", "click")
		x = int(args.get("x", 0))
		y = int(args.get("y", 0))
		amount = int(args.get("amount", 3))
		try:
			import pyautogui
			pyautogui.FAILSAFE = True
			if action == "move":
				await asyncio.to_thread(pyautogui.moveTo, x, y, duration=0.2)
				return f"курсор → ({x}, {y})"
			elif action == "click":
				await asyncio.to_thread(pyautogui.click, x, y)
				return f"клик ({x}, {y})"
			elif action == "right_click":
				await asyncio.to_thread(pyautogui.rightClick, x, y)
				return f"правый клик ({x}, {y})"
			elif action == "double_click":
				await asyncio.to_thread(pyautogui.doubleClick, x, y)
				return f"двойной клик ({x}, {y})"
			elif action == "scroll":
				await asyncio.to_thread(pyautogui.scroll, amount, x, y)
				return f"скролл {amount} @ ({x}, {y})"
			return f"ошибка: неизвестный action '{action}'"
		except ImportError:
			return "ошибка: pip install pyautogui"
		except Exception as e:
			return f"ошибка: {e}"


class KeyboardTool(Tool):
	name = "keyboard"
	description = (
		"Управлять клавиатурой. "
		"action: type — напечатать текст, press — нажать клавишу/комбинацию. "
		"Для press ключи через + (ctrl+c, alt+f4, win+d, enter, escape)."
	)
	params = {
		"action": {"type": "string", "description": "type | press"},
		"text": {"type": "string", "description": "Текст для type или клавиши для press"},
	}

	async def execute(self, args: Dict[str, Any]) -> str:
		action = args.get("action", "type")
		text = args.get("text", "")
		try:
			import pyautogui
			if action == "type":
				await asyncio.to_thread(pyautogui.write, text, interval=0.03)
				return f"напечатано: {text[:60]}"
			elif action == "press":
				keys = [k.strip() for k in text.split("+")]
				if len(keys) == 1:
					await asyncio.to_thread(pyautogui.press, keys[0])
				else:
					await asyncio.to_thread(pyautogui.hotkey, *keys)
				return f"нажато: {text}"
			return f"ошибка: action должен быть type или press"
		except ImportError:
			return "ошибка: pip install pyautogui"
		except Exception as e:
			return f"ошибка: {e}"


class BrowserOpenTool(Tool):
	name = "browser_open"
	description = "Открыть URL в браузере пользователя. Используй для открытия сайтов, видео, страниц."
	params = {"url": {"type": "string", "description": "URL"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		url = args.get("url", "")
		try:
			import webbrowser
			await asyncio.to_thread(webbrowser.open, url)
			return f"открыт: {url}"
		except Exception as e:
			return f"ошибка: {e}"


class BrowserReadTool(Tool):
	name = "browser_read"
	description = (
		"Прочитать содержимое страницы: текст и список ссылок. "
		"Для навигации по сайтам и извлечения данных."
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
				out += "\n\nСсылки:\n" + "\n".join(f"• {l['text']}: {l['url']}" for l in data["links"])
			return out[:3000]
		except ImportError:
			return "ошибка: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"ошибка: {e}"


class YouTubeSearchTool(Tool):
	name = "youtube_search"
	description = (
		"Искать видео на YouTube. Возвращает список видео с прямыми ссылками. "
		"Используй вместо google_search для поиска видео."
	)
	params = {"query": {"type": "string", "description": "Поисковый запрос"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		import urllib.parse
		query = args.get("query", "")
		try:
			from playwright.async_api import async_playwright
			async with async_playwright() as pw:
				browser = await pw.chromium.launch(headless=True)
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
				await browser.close()
			if not results:
				return "нет результатов"
			return "\n".join(
				f"{i+1}. {r['title']}{(' · ' + r['meta']) if r['meta'] else ''}\n   {r['url']}"
				for i, r in enumerate(results)
			)
		except ImportError:
			return "ошибка: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"ошибка: {e}"


class GoogleSearchTool(Tool):
	name = "google_search"
	description = "Поиск в интернете. Возвращает топ-5 результатов. Не открывает браузер пользователю."
	params = {"query": {"type": "string", "description": "Поисковый запрос"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		query = args.get("query", "")
		try:
			from duckduckgo_search import DDGS
			results = await asyncio.to_thread(
				lambda: list(DDGS().text(query, max_results=5))
			)
			if not results:
				return "нет результатов"
			return "\n".join(
				f"• {r['title']}\n  {r['href']}\n  {r.get('body', '')[:200]}"
				for r in results
			)
		except ImportError:
			return "ошибка: pip install duckduckgo-search"
		except Exception as e:
			return f"ошибка: {e}"


class BrowserScreenshotTool(Tool):
	name = "browser_screenshot"
	description = "Сделать скриншот веб-страницы в фоне, вернуть путь к файлу."
	params = {"url": {"type": "string", "description": "URL"}}

	async def execute(self, args: Dict[str, Any]) -> str:
		url = args.get("url", "")
		try:
			from playwright.async_api import async_playwright
			import datetime
			name = f"browser_{datetime.datetime.now():%Y%m%d_%H%M%S}.png"
			async with async_playwright() as pw:
				browser = await pw.chromium.launch(headless=True)
				page = await browser.new_page()
				await page.goto(url, timeout=20000)
				await page.screenshot(path=name, full_page=True)
				await browser.close()
			return f"сохранён: {name}"
		except ImportError:
			return "ошибка: pip install playwright && playwright install chromium"
		except Exception as e:
			return f"ошибка: {e}"


# ── реестр и ожидающие подтверждения ─────────────────────────────────────────

REGISTRY: Dict[str, Tool] = {t.name: t for t in [
	BashTool(),
	FileReadTool(),
	FileWriteTool(),
	FileDeleteTool(),
	FileListTool(),
	ClipboardTool(),
	ScreenshotTool(),
	MouseTool(),
	KeyboardTool(),
	BrowserOpenTool(),
	BrowserReadTool(),
	YouTubeSearchTool(),
	GoogleSearchTool(),
	BrowserScreenshotTool(),
]}

# {id: {"event": asyncio.Event, "ok": bool}}
PENDING: Dict[str, dict] = {}


def gen_id() -> str:
	return uuid.uuid4().hex[:8]


def parse_args(arguments: Any) -> dict:
	if isinstance(arguments, dict):
		return arguments
	try:
		return json.loads(arguments)
	except Exception:
		return {}
