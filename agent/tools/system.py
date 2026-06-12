from __future__ import annotations

import asyncio
import os
import platform
import shutil
import socket
import subprocess
from typing import Any, Dict

from agent.tools.base import Tool


def _is_windows() -> bool:
	return os.name == "nt"


def _fmt_bytes(n: float) -> str:
	for unit in ("B", "KB", "MB", "GB"):
		if n < 1024:
			return f"{n:.1f}{unit}"
		n /= 1024
	return f"{n:.1f}TB"


def _total_ram() -> int | None:
	"""Total physical memory in bytes, or None if it can't be determined."""
	try:
		if _is_windows():
			import ctypes

			class _MemStatus(ctypes.Structure):
				_fields_ = [
					("dwLength", ctypes.c_ulong),
					("dwMemoryLoad", ctypes.c_ulong),
					("ullTotalPhys", ctypes.c_ulonglong),
					("ullAvailPhys", ctypes.c_ulonglong),
					("ullTotalPageFile", ctypes.c_ulonglong),
					("ullAvailPageFile", ctypes.c_ulonglong),
					("ullTotalVirtual", ctypes.c_ulonglong),
					("ullAvailVirtual", ctypes.c_ulonglong),
					("ullAvailExtendedVirtual", ctypes.c_ulonglong),
				]

			stat = _MemStatus()
			stat.dwLength = ctypes.sizeof(_MemStatus)
			if ctypes.windll.kernel32.GlobalMemoryStatusEx(ctypes.byref(stat)):
				return int(stat.ullTotalPhys)
			return None
		# Linux / others
		with open("/proc/meminfo", encoding="utf-8") as f:
			for line in f:
				if line.startswith("MemTotal:"):
					return int(line.split()[1]) * 1024
	except Exception:
		return None
	return None


def _uptime_seconds() -> float | None:
	try:
		if _is_windows():
			import ctypes

			return ctypes.windll.kernel32.GetTickCount64() / 1000.0
		with open("/proc/uptime", encoding="utf-8") as f:
			return float(f.read().split()[0])
	except Exception:
		return None


def _fmt_uptime(seconds: float) -> str:
	s = int(seconds)
	d, s = divmod(s, 86400)
	h, s = divmod(s, 3600)
	m, _ = divmod(s, 60)
	parts = []
	if d:
		parts.append(f"{d}d")
	if h or d:
		parts.append(f"{h}h")
	parts.append(f"{m}m")
	return " ".join(parts)


class SystemInfoTool(Tool):
	name = "system_info"
	description = (
		"Report machine info: OS, hostname, CPU count, total RAM, disk usage, "
		"uptime, Python version. Read-only."
	)
	params = {}

	async def execute(self, args: Dict[str, Any]) -> str:
		lines = [
			f"os: {platform.platform()}",
			f"hostname: {socket.gethostname()}",
			f"arch: {platform.machine()}",
			f"cpu: {os.cpu_count()} logical cores",
		]
		ram = _total_ram()
		if ram is not None:
			lines.append(f"ram: {_fmt_bytes(ram)}")
		up = _uptime_seconds()
		if up is not None:
			lines.append(f"uptime: {_fmt_uptime(up)}")
		lines.append(f"python: {platform.python_version()}")

		# disks: each fixed/root mount we can stat
		roots: list[str] = []
		if _is_windows():
			import string

			for letter in string.ascii_uppercase:
				drive = f"{letter}:\\"
				if os.path.exists(drive):
					roots.append(drive)
		else:
			roots.append("/")
		for root in roots:
			try:
				usage = shutil.disk_usage(root)
				lines.append(
					f"disk {root}: {_fmt_bytes(usage.used)} / {_fmt_bytes(usage.total)} used"
				)
			except OSError:
				pass
		return "\n".join(lines)


# Detached PowerShell that shows a tray balloon notification then disposes the
# icon. Runs fire-and-forget so the tool returns immediately.
_NOTIFY_SCRIPT = r"""
Add-Type -AssemblyName System.Windows.Forms | Out-Null
Add-Type -AssemblyName System.Drawing | Out-Null
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.Visible = $true
$n.ShowBalloonTip(5000, $env:WARDEN_NOTIFY_TITLE, $env:WARDEN_NOTIFY_MESSAGE, [System.Windows.Forms.ToolTipIcon]::Info)
Start-Sleep -Seconds 6
$n.Dispose()
"""


class NotifyTool(Tool):
	name = "notify"
	description = (
		"Show a desktop notification (tray balloon) to the user. Windows only. "
		"Use to signal task completion or important events. Non-blocking."
	)
	params = {
		"message": {"type": "string", "description": "Notification body text"},
		"title": {"type": "string", "description": "Notification title (default: Warden)"},
	}

	def tool_definition(self) -> dict:
		d = super().tool_definition()
		d["function"]["parameters"]["required"] = ["message"]
		return d

	async def execute(self, args: Dict[str, Any]) -> str:
		if not _is_windows():
			return "error: notify is Windows-only"
		message = str(args.get("message", "")).strip()
		if not message:
			return "error: message is required"
		title = str(args.get("title", "")).strip() or "Warden"
		env = dict(os.environ)
		env["WARDEN_NOTIFY_TITLE"] = title
		env["WARDEN_NOTIFY_MESSAGE"] = message
		try:
			# fire-and-forget so the balloon's lifetime doesn't block the agent
			await asyncio.to_thread(
				subprocess.Popen,
				["powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", _NOTIFY_SCRIPT],
				env=env,
				stdout=subprocess.DEVNULL,
				stderr=subprocess.DEVNULL,
			)
		except Exception as e:
			return f"error: {e}"
		return f"notified: {title} — {message[:60]}"
