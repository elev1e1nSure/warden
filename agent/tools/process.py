from __future__ import annotations

import asyncio
import os
import re
import subprocess
from typing import Any, Dict

from agent.tools.base import Tool


def _is_windows() -> bool:
	return os.name == "nt"


class ProcessListTool(Tool):
	"""List running processes. Structured output (name, pid, optional session id)."""
	name = "process_list"
	description = (
		"List running processes. Returns a compact table: PID, name, optional session id. "
		"Filter by name (case-insensitive substring). Output is capped at 200 rows."
	)
	params = {
		"filter": {"type": "string", "description": "Substring to filter by process name (optional)"},
	}

	async def execute(self, args: Dict[str, Any]) -> str:
		name_filter = str(args.get("filter", "")).strip().lower()
		try:
			if _is_windows():
				rows = await self._list_windows(name_filter)
			else:
				rows = await self._list_unix(name_filter)
		except subprocess.TimeoutExpired:
			return "error: timeout listing processes"
		except Exception as e:
			return f"error: {e}"

		if not rows:
			return "no processes" if not name_filter else f"no processes matching '{name_filter}'"
		out = "\n".join(f"{pid:>7}  {name}" for pid, name in rows[:200])
		if len(rows) > 200:
			out += f"\n... and {len(rows) - 200} more"
		return out

	async def _list_windows(self, name_filter: str) -> list[tuple[str, str]]:
		# tasklist /FO CSV /NH gives: "name","pid","session","sessionNum","mem"
		proc = await asyncio.to_thread(
			subprocess.run,
			["tasklist", "/FO", "CSV", "/NH"],
			capture_output=True, text=True, timeout=15,
			encoding="utf-8", errors="replace",
		)
		rows = []
		for line in (proc.stdout or "").splitlines():
			parts = [p.strip().strip('"') for p in line.split('","')]
			if len(parts) < 2 or not parts[1].isdigit():
				continue
			pid, name = parts[1], parts[0]
			if name_filter and name_filter not in name.lower():
				continue
			rows.append((pid, name))
		return rows

	async def _list_unix(self, name_filter: str) -> list[tuple[str, str]]:
		# `ps -eo pid=,comm=` is portable and predictable.
		proc = await asyncio.to_thread(
			subprocess.run,
			["ps", "-eo", "pid=,comm="],
			capture_output=True, text=True, timeout=15,
			encoding="utf-8", errors="replace",
		)
		rows = []
		for line in (proc.stdout or "").splitlines():
			m = re.match(r"^\s*(\d+)\s+(.+)$", line)
			if not m:
				continue
			pid, name = m.group(1), m.group(2).strip()
			if name_filter and name_filter not in name.lower():
				continue
			rows.append((pid, name))
		return rows


class ProcessKillTool(Tool):
	"""Terminate a process by PID. Refuses obviously dangerous PIDs."""
	name = "process_kill"
	description = (
		"Terminate a process by PID. On Windows uses taskkill /F; on Unix uses kill -9. "
		"Always force-kills (no graceful shutdown). Refuses PIDs 0, 1, and the current process."
	)
	params = {
		"pid": {"type": "integer", "description": "Process ID to terminate"},
	}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return True

	async def execute(self, args: Dict[str, Any]) -> str:
		pid = args.get("pid")
		try:
			pid = int(pid)
		except (TypeError, ValueError):
			return "error: pid must be an integer"
		if pid <= 1:
			return "error: refusing to kill PID 0 or 1 (init/system)"
		if pid == os.getpid():
			return "error: refusing to kill self"

		try:
			if _is_windows():
				proc = await asyncio.to_thread(
					subprocess.run,
					["taskkill", "/F", "/PID", str(pid)],
					capture_output=True, text=True, timeout=10,
					encoding="utf-8", errors="replace",
				)
			else:
				proc = await asyncio.to_thread(
					subprocess.run,
					["kill", "-9", str(pid)],
					capture_output=True, text=True, timeout=10,
					encoding="utf-8", errors="replace",
				)
		except subprocess.TimeoutExpired:
			return f"error: timeout killing PID {pid}"
		except Exception as e:
			return f"error: {e}"

		if proc.returncode == 0:
			return f"killed PID {pid}"
		err = (proc.stderr or proc.stdout or "").strip() or f"exit {proc.returncode}"
		return f"error: failed to kill PID {pid}: {err[:200]}"
