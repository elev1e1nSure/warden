from __future__ import annotations

import asyncio
import shutil
import subprocess
from typing import Any

from agent.tools.base import Tool, _clean


def _shell_executable() -> str:
    """Return pwsh if available, otherwise powershell."""
    if shutil.which("pwsh"):
        return "pwsh"
    return "powershell"


class PowerShellTool(Tool):
    name = "powershell"
    description = "Run a PowerShell command. For files, processes, system."
    params = {"command": {"type": "string", "description": "PowerShell command"}}

    async def execute(self, args: dict[str, Any]) -> str:
        cmd = args.get("command", "")
        shell = _shell_executable()
        # Force UTF-8 output so non-ASCII (Cyrillic etc.) isn't mangled by the
        # console's OEM codepage, then decode as UTF-8.
        wrapped = "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; " + cmd
        try:
            proc = await asyncio.to_thread(
                subprocess.run,
                [shell, "-NonInteractive", "-NoProfile", "-Command", wrapped],
                capture_output=True,
                text=True,
                timeout=30,
                encoding="utf-8",
                errors="replace",
            )
            out = _clean((proc.stdout or "").strip())
            err = _clean((proc.stderr or "").strip())
            if not out and err:
                return f"stderr: {err[:500]}"
            if not out:
                return "(no output)"
            return out[:1000] + (f"\nstderr: {err[:200]}" if err else "")
        except subprocess.TimeoutExpired:
            return "error: timeout 30s"
        except Exception as e:
            return f"error: {e}"


class BashTool(PowerShellTool):
    """Deprecated alias — kept for backward compatibility."""

    name = "bash"
