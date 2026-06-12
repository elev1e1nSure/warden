from __future__ import annotations

import asyncio
import json
import os
import subprocess
from typing import Any, Dict

from agent.tools.base import Tool


def _is_windows() -> bool:
	return os.name == "nt"


# PowerShell helper shared by the window tools. Defines a small user32 wrapper
# once, then runs whatever body is appended. Kept as a raw string so callers
# only supply the action-specific tail.
_WIN32_HEADER = r"""
Add-Type @"
using System;
using System.Runtime.InteropServices;
public class WardenWin {
	[DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr h, out RECT r);
	[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);
	[DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr h, int cmd);
	[DllImport("user32.dll")] public static extern bool MoveWindow(IntPtr h, int x, int y, int w, int ht, bool repaint);
	[DllImport("user32.dll")] public static extern IntPtr SendMessage(IntPtr h, uint msg, IntPtr wp, IntPtr lp);
	[StructLayout(LayoutKind.Sequential)] public struct RECT { public int Left, Top, Right, Bottom; }
}
"@ | Out-Null
"""


async def _run_ps(script: str, timeout: float = 15) -> str:
	"""Run a PowerShell script and return stdout (raises on non-zero exit)."""
	proc = await asyncio.to_thread(
		subprocess.run,
		["powershell", "-NoProfile", "-NonInteractive", "-Command", script],
		capture_output=True, text=True, timeout=timeout,
		encoding="utf-8", errors="replace",
	)
	if proc.returncode != 0:
		raise RuntimeError((proc.stderr or "powershell failed").strip())
	return proc.stdout or ""


async def _enumerate_windows() -> list[dict]:
	"""Top-level windows with a visible title: pid, title, hwnd, x/y/w/h."""
	if not _is_windows():
		return []
	script = _WIN32_HEADER + r"""
$items = Get-Process | Where-Object { $_.MainWindowTitle -ne '' } | ForEach-Object {
	$r = New-Object WardenWin+RECT
	[void][WardenWin]::GetWindowRect($_.MainWindowHandle, [ref]$r)
	[PSCustomObject]@{
		pid   = $_.Id
		title = $_.MainWindowTitle
		hwnd  = [int64]$_.MainWindowHandle
		x = $r.Left; y = $r.Top; w = ($r.Right - $r.Left); h = ($r.Bottom - $r.Top)
	}
}
@($items) | ConvertTo-Json -Compress
"""
	out = (await _run_ps(script)).strip()
	if not out:
		return []
	data = json.loads(out)
	if isinstance(data, dict):
		data = [data]
	return data


def _match_window(windows: list[dict], title: str | None, hwnd: int | None) -> dict | None:
	if hwnd is not None:
		for w in windows:
			if int(w.get("hwnd", 0)) == int(hwnd):
				return w
		return None
	if title:
		needle = title.lower()
		for w in windows:
			if needle in str(w.get("title", "")).lower():
				return w
	return None


class WindowListTool(Tool):
	name = "window_list"
	description = (
		"List open top-level windows (title, pid, hwnd, bounds). "
		"Windows only. Use to find a window before focusing or managing it."
	)
	params = {
		"filter": {"type": "string", "description": "Substring to filter by window title (optional)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = []
		return d

	async def execute(self, args: Dict[str, Any]) -> str:
		if not _is_windows():
			return "error: window tools are Windows-only"
		name_filter = str(args.get("filter", "")).strip().lower()
		try:
			windows = await _enumerate_windows()
		except subprocess.TimeoutExpired:
			return "error: timeout listing windows"
		except Exception as e:
			return f"error: {e}"
		if name_filter:
			windows = [w for w in windows if name_filter in str(w.get("title", "")).lower()]
		if not windows:
			return "no windows" if not name_filter else f"no windows matching '{name_filter}'"
		return "\n".join(
			f"{w.get('hwnd')}  pid={w.get('pid')}  "
			f"[{w.get('x')},{w.get('y')} {w.get('w')}x{w.get('h')}]  {w.get('title')}"
			for w in windows[:100]
		)


class WindowFocusTool(Tool):
	name = "window_focus"
	description = (
		"Bring a window to the foreground by title substring or hwnd. "
		"Windows only. Restores the window if minimized."
	)
	params = {
		"title": {"type": "string", "description": "Window title substring (optional if hwnd given)"},
		"hwnd": {"type": "integer", "description": "Window handle from window_list (optional if title given)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = []
		return d

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return True

	async def execute(self, args: Dict[str, Any]) -> str:
		if not _is_windows():
			return "error: window tools are Windows-only"
		title = str(args.get("title", "")).strip() or None
		hwnd = args.get("hwnd")
		if not title and hwnd is None:
			return "error: give a title or hwnd"
		try:
			win = _match_window(await _enumerate_windows(), title, hwnd)
			if win is None:
				return "error: window not found"
			handle = int(win["hwnd"])
			# 9 = SW_RESTORE
			script = _WIN32_HEADER + (
				f"$h = [IntPtr]{handle}\n"
				"[void][WardenWin]::ShowWindow($h, 9)\n"
				"[void][WardenWin]::SetForegroundWindow($h)\n"
			)
			await _run_ps(script)
			return f"focused: {win.get('title')} (hwnd={handle})"
		except Exception as e:
			return f"error: {e}"


class WindowManageTool(Tool):
	name = "window_manage"
	description = (
		"Manage a window by title substring or hwnd. Windows only. "
		"action: minimize | maximize | restore | close | move (needs x/y) | "
		"resize (needs w/h, optional x/y)."
	)
	params = {
		"action": {"type": "string", "description": "minimize | maximize | restore | close | move | resize"},
		"title": {"type": "string", "description": "Window title substring (optional if hwnd given)"},
		"hwnd": {"type": "integer", "description": "Window handle from window_list (optional if title given)"},
		"x": {"type": "integer", "description": "New left position (move/resize)"},
		"y": {"type": "integer", "description": "New top position (move/resize)"},
		"w": {"type": "integer", "description": "New width (resize)"},
		"h": {"type": "integer", "description": "New height (resize)"},
	}

	_SHOW = {"minimize": 6, "maximize": 3, "restore": 9}  # SW_MINIMIZE/MAXIMIZE/RESTORE

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = ["action"]
		return d

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return True

	async def execute(self, args: Dict[str, Any]) -> str:
		if not _is_windows():
			return "error: window tools are Windows-only"
		action = str(args.get("action", "")).strip().lower()
		title = str(args.get("title", "")).strip() or None
		hwnd = args.get("hwnd")
		if not title and hwnd is None:
			return "error: give a title or hwnd"
		try:
			win = _match_window(await _enumerate_windows(), title, hwnd)
			if win is None:
				return "error: window not found"
			handle = int(win["hwnd"])
			if action in self._SHOW:
				body = f"[void][WardenWin]::ShowWindow([IntPtr]{handle}, {self._SHOW[action]})\n"
			elif action == "close":
				# WM_CLOSE = 0x0010
				body = f"[void][WardenWin]::SendMessage([IntPtr]{handle}, 0x0010, [IntPtr]::Zero, [IntPtr]::Zero)\n"
			elif action in ("move", "resize"):
				x = int(args.get("x", win.get("x", 0)))
				y = int(args.get("y", win.get("y", 0)))
				w = int(args.get("w", win.get("w", 0)))
				h = int(args.get("h", win.get("h", 0)))
				if action == "resize" and (w <= 0 or h <= 0):
					return "error: resize needs positive w and h"
				body = f"[void][WardenWin]::MoveWindow([IntPtr]{handle}, {x}, {y}, {w}, {h}, $true)\n"
			else:
				return f"error: unknown action '{action}'"
			await _run_ps(_WIN32_HEADER + body)
			return f"{action}: {win.get('title')} (hwnd={handle})"
		except Exception as e:
			return f"error: {e}"
