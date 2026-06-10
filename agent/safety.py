"""Safety policy layer for leashed mode.

The model proposes a tool call; this module classifies the risk.
The TUI renders a human confirmation or blocks the action.
"""

from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Any, List


@dataclass
class SafetyDecision:
	risk: str  # "safe" | "confirm" | "blocked"
	reason: str
	summary: str
	details: List[str] = field(default_factory=list)
	normalized_args: Dict[str, Any] = field(default_factory=dict)


# ---------------------------------------------------------------------------
# Path safety
# ---------------------------------------------------------------------------

def _resolve_workspace() -> Path:
	"""Return the canonical workspace root."""
	return Path(os.getcwd()).resolve()


def _is_path_within_workspace(path: str | Path, workspace: Path | None = None) -> bool:
	"""True if *path* is contained inside *workspace*."""
	try:
		target = Path(path).resolve()
	except (OSError, ValueError):
		return False
	if workspace is None:
		workspace = _resolve_workspace()
	try:
		target.relative_to(workspace)
		return True
	except ValueError:
		return False


def _is_dangerous_path(path: str) -> bool:
	"""Block UNC paths, device paths, and obvious traversal."""
	p = str(path).strip().lower()
	if p.startswith("\\\\"):
		return True
	if p.startswith("\\\\.\\") or p.startswith("\\\\?\\"):
		return True
	normalized = p.replace("\\", "/")
	if "../" in normalized or "/.." in normalized:
		return True
	# Absolute unix-style paths on Windows are suspicious
	if normalized.startswith("/") and not re.match(r"^/[a-z]:", normalized):
		return True
	return False


# ---------------------------------------------------------------------------
# PowerShell classification
# ---------------------------------------------------------------------------

# Aliases that map to dangerous cmdlets
_DELETE_ALIASES = {"rm", "del", "erase", "rmdir", "rd", "ri", "remove-item"}
_KILL_ALIASES = {"kill", "spps", "stop-process"}
_INVOKE_ALIASES = {"iex", "invoke-expression"}

# Dangerous cmdlets / commands
_BLOCKED_CMDLETS = {
	"remove-item", "ri", "rmdir", "del", "erase", "rd",  # delete
	"stop-process", "spps", "kill", "taskkill",  # kill
	"format", "mkfs", "diskpart",  # disk
	"set-service", "sc", "sc.exe",  # service changes
	"new-service", "remove-service",  # service lifecycle
	"reg", "reg.exe", "set-itemproperty", "remove-itemproperty", "new-itemproperty",  # registry
	"invoke-expression", "iex",  # eval
	"invoke-command",  # remote execution
	"clear-content", "clc",  # wipe file content
	"set-executionpolicy",  # policy changes
	"netsh",  # firewall/network
	"bcdedit",  # boot config
	"cipher",  # encryption/wipe
}

_CONFIRM_CMDLETS = {
	"set-content", "add-content", "out-file",
	"copy-item", "cp", "cpi", "move-item", "mv", "mi",
	"rename-item", "rni", "ren",
	"start-process", "saps", "start",
	"winget", "npm", "pnpm", "pip", "go",  # package managers
	"git", "node", "python", "py",  # common executables
}

_SAFE_CMDLETS = {
	"get-childitem", "gci", "ls", "dir",
	"get-content", "gc", "cat", "type",
	"test-path", "resolve-path",
	"get-process", "gps", "ps",
	"get-service", "gsv",
	"get-item", "gi",
	"where-object", "?",
	"foreach-object", "%",
	"select-object", "sort-object", "measure-object",
	"write-output", "write-host", "write-verbose", "write-warning",
	"out-string", "out-null",
	"findstr", "grep", "rg", "fd",
}

# Patterns for encoded/remote execution; includes short aliases -e/-ec/-en
_ENCODED_RE = re.compile(
	r"-[eE]nc(?:oded)?[Cc]ommand\b|/[eE]:\b|-enc\b|-[eE][cCnN]\b",
	re.IGNORECASE,
)
_REMOTE_PIPE_RE = re.compile(
	r"(iwr|irm|invoke-webrequest|invoke-restmethod|curl\.exe|wget\.exe)\s+.*\|\s*iex",
	re.IGNORECASE,
)

# Real chain operators — semicolon, &, and newlines chain commands; pipe is excluded (benign in pipelines)
_CHAIN_RE = re.compile(r"[;&\r\n]")


def _normalize_command(command: str) -> str:
	"""Collapse backtick continuations and normalize whitespace."""
	text = command.replace("`\r\n", " ").replace("`\n", " ").replace("`\r", " ")
	text = re.sub(r"`\s+", " ", text)
	return text.strip()


def _tokens_from_command(command: str) -> List[str]:
	"""Split command into rough tokens for analysis."""
	tokens = re.split(r"[\s|;`&|]+", command)
	return [t.lower().strip("\t\r\n'\"") for t in tokens if t]


def _extract_executable_and_args(command: str) -> tuple[str, List[str]]:
	"""Extract the first executable and remaining tokens."""
	tokens = _tokens_from_command(command)
	if not tokens:
		return "", []
	return tokens[0], tokens[1:]


def _has_any(tokens: List[str], candidates: set[str]) -> bool:
	return any(t in candidates for t in tokens)


def _classify_powershell(command: str) -> tuple[str, str, List[str]]:
	"""Classify a PowerShell command string. Returns (risk, reason, details)."""
	norm = _normalize_command(command)
	tokens = _tokens_from_command(norm)

	# 1. Encoded commands → blocked
	if _ENCODED_RE.search(norm):
		return "blocked", "encoded command execution", ["uses -EncodedCommand or similar"]

	# 2. Remote execution pipe → blocked
	if _REMOTE_PIPE_RE.search(norm):
		return "blocked", "remote script execution via iex", ["downloads remote content and executes it"]

	# 3. Nested shells → recurse into first layer
	nested_match = re.search(
		r"(?:cmd\.exe|cmd)\s+/[cCkK]\s+(?:['\"])?(.+?)(?:['\"])?$|"
		r"(?:pwsh|powershell)\s+(?:-[cC]ommand|-c)\s+['\"]?(.+?)['\"]?$|"
		r"(?:bash|sh)\s+-c\s+['\"]?(.+?)['\"]?$",
		norm, re.IGNORECASE,
	)
	if nested_match:
		inner = next((g for g in nested_match.groups() if g), "")
		if inner:
			return _classify_powershell(inner)

	# 4. Check for cmd.exe style destructive flags
	if re.search(r"\b(rd|rmdir|del|erase|deltree)\b.*/[fFsSqQ]\b", norm, re.IGNORECASE):
		return "blocked", "destructive cmd.exe command", ["uses cmd-style delete with force/recurse flags"]

	# 5. Check for format/mkfs/diskpart
	if re.search(r"\b(format\s+[a-z]:|mkfs|diskpart|cipher\s+/w)\b", norm, re.IGNORECASE):
		return "blocked", "disk destruction command", ["can erase drives or volumes"]

	# 5b. Check for shutdown / restart / poweroff (power-state change, any timer)
	if re.search(r"\bshutdown\b.*\s/[srph]\b", norm, re.IGNORECASE):
		return "blocked", "system power command", ["shuts down, restarts, or powers off the machine"]

	# 6. Check for git destructive commands
	git_destructive = re.search(
		r"git\s+(reset\s+--hard|clean\s+-fd|push\s+--force|push\s+-f|branch\s+-D)",
		norm, re.IGNORECASE,
	)
	if git_destructive:
		return "blocked", "destructive git command", [f"matched: {git_destructive.group(1)}"]

	# 7. Check for registry / system policy changes
	if re.search(
		r"\b(reg\s+(add|delete|edit)|set-itemproperty|remove-itemproperty|"
		r"new-itemproperty|netsh\s+advfirewall)\b",
		norm, re.IGNORECASE,
	):
		return "blocked", "system/registry modification", ["changes system configuration"]

	# 8. Check for force + recurse delete
	if re.search(
		r"\b(remove-item|rm|del|rmdir|rd|ri)\b.*(-recurse|-r)\s+.*(-force|-f)|"
		r"\b(remove-item|rm|del|rmdir|rd|ri)\b.*(-force|-f)\s+.*(-recurse|-r)",
		norm, re.IGNORECASE,
	):
		return "blocked", "recursive forced deletion", ["uses -Recurse and -Force on delete"]

	# 9. Chain operators before exe-specific early returns — prevents git/go safe exit masking chained evil
	if _CHAIN_RE.search(norm):
		return "confirm", "chained command", ["contains command chains (;/&)"]

	# 10. Identify primary cmdlet
	exe, args = _extract_executable_and_args(norm)

	# Delete without recurse/force → confirm
	if exe in _DELETE_ALIASES:
		return "confirm", "file deletion", ["deletes files or directories"]

	# Kill process → confirm
	if exe in _KILL_ALIASES or "taskkill" in tokens:
		return "confirm", "process termination", ["stops a running process"]

	# Invoke-expression → blocked
	if exe in _INVOKE_ALIASES:
		return "blocked", "code evaluation", ["Invoke-Expression can execute arbitrary code"]

	# Blocked cmdlets
	if exe in _BLOCKED_CMDLETS or _has_any(tokens, _BLOCKED_CMDLETS):
		return "blocked", "restricted system command", [f"command '{exe}' is blocked in leashed mode"]

	# Package managers / installations
	if exe in {"winget", "npm", "pnpm", "pip", "uv", "gem", "cargo"}:
		return "confirm", "package installation or modification", [f"uses {exe}"]

	# Git: safe for status/diff/log/show, confirm for others
	if exe == "git":
		if len(tokens) >= 2:
			sub = tokens[1]
			if sub in {"status", "diff", "log", "show", "branch", "tag", "config", "remote", "stash", "ls-files"}:
				return "safe", "read-only git command", [f"git {sub}"]
		return "confirm", "git command", ["git may change repository state"]

	# Go: safe for test/fmt/vet/env, confirm for others
	if exe == "go":
		if len(tokens) >= 2:
			sub = tokens[1]
			if sub in {"test", "fmt", "vet", "env", "version", "mod", "list", "doc"}:
				return "safe", "read-only go command", [f"go {sub}"]
		return "confirm", "go command", ["go may change project state"]

	# Python: safe for reads, confirm for execution
	if exe in {"python", "py"}:
		if "-m" in tokens and "py_compile" in tokens:
			return "safe", "read-only python check", ["py_compile"]
		return "confirm", "python execution", ["python may execute arbitrary code"]

	# File writes / copies / moves
	if exe in _CONFIRM_CMDLETS or _has_any(tokens, _CONFIRM_CMDLETS):
		return "confirm", "file or system modification", [f"command '{exe}' changes state"]

	# Safe cmdlets
	if exe in _SAFE_CMDLETS or _has_any(tokens, _SAFE_CMDLETS):
		return "safe", "read-only command", []

	# Unknown executable — require confirmation, never assume safe
	return "confirm", "unknown command", ["no safety policy defined for this command"]


# ---------------------------------------------------------------------------
# Tool-level safety assessment
# ---------------------------------------------------------------------------

def assess_tool_call(tool_name: str, args: dict, cwd: str | None = None) -> SafetyDecision:
	"""Assess a tool call and return a SafetyDecision."""
	if cwd is None:
		cwd = os.getcwd()
	workspace = Path(cwd).resolve()
	normalized = dict(args)

	# --- file_write ---
	if tool_name in ("file_write", "write"):
		path = str(normalized.get("path", ""))
		if _is_dangerous_path(path):
			return SafetyDecision(
				risk="blocked",
				reason="dangerous path",
				summary="File path is outside allowed scope",
				details=["UNC path, device path, or traversal detected"],
				normalized_args=normalized,
			)
		if not _is_path_within_workspace(path, workspace):
			return SafetyDecision(
				risk="confirm",
				reason="writes outside workspace",
				summary="Writing file outside workspace",
				details=[f"path: {path}"],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="confirm",
			reason="modifies files",
			summary="Writing file inside workspace",
			details=[f"path: {path}"],
			normalized_args=normalized,
		)

	# --- file_delete ---
	if tool_name in ("file_delete", "delete"):
		path = str(normalized.get("path", ""))
		if _is_dangerous_path(path):
			return SafetyDecision(
				risk="blocked",
				reason="dangerous path",
				summary="File path is outside allowed scope",
				details=["UNC path, device path, or traversal detected"],
				normalized_args=normalized,
			)
		if not _is_path_within_workspace(path, workspace):
			return SafetyDecision(
				risk="blocked",
				reason="deletes outside workspace",
				summary="Deleting file outside workspace is blocked",
				details=[f"path: {path}"],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="confirm",
			reason="destructive file operation",
			summary="Deleting file inside workspace",
			details=[f"path: {path}"],
			normalized_args=normalized,
		)

	# --- file_read ---
	if tool_name in ("file_read", "read"):
		path = str(normalized.get("path", ""))
		if _is_dangerous_path(path):
			return SafetyDecision(
				risk="blocked",
				reason="dangerous path",
				summary="File path is outside allowed scope",
				details=["UNC path, device path, or traversal detected"],
				normalized_args=normalized,
			)
		if not _is_path_within_workspace(path, workspace):
			return SafetyDecision(
				risk="confirm",
				reason="reads outside workspace",
				summary="Reading file outside workspace",
				details=[f"path: {path}"],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="safe",
			reason="read-only",
			summary="Reading file",
			details=[f"path: {path}"],
			normalized_args=normalized,
		)

	# --- file_list ---
	if tool_name in ("file_list", "list"):
		path = str(normalized.get("path", "."))
		if _is_dangerous_path(path):
			return SafetyDecision(
				risk="blocked",
				reason="dangerous path",
				summary="Path is outside allowed scope",
				details=["UNC path, device path, or traversal detected"],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="safe",
			reason="read-only",
			summary="Listing directory",
			details=[f"path: {path}"],
			normalized_args=normalized,
		)

	# --- bash / powershell ---
	if tool_name in ("bash", "powershell", "shell"):
		command = str(normalized.get("command", ""))
		risk, reason, details = _classify_powershell(command)
		summary = reason.capitalize()
		if risk == "safe":
			summary = "Read-only shell command"
		return SafetyDecision(
			risk=risk,
			reason=reason,
			summary=summary,
			details=details,
			normalized_args=normalized,
		)

	# --- clipboard ---
	if tool_name == "clipboard":
		action = str(normalized.get("action", "read")).lower()
		if action == "read":
			return SafetyDecision(
				risk="safe",
				reason="read-only",
				summary="Reading clipboard",
				details=[],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="confirm",
			reason="modifies clipboard",
			summary="Writing to clipboard",
			details=[],
			normalized_args=normalized,
		)

	# --- screenshot ---
	if tool_name == "screenshot":
		return SafetyDecision(
			risk="safe",
			reason="read-only",
			summary="Taking screenshot",
			details=[],
			normalized_args=normalized,
		)

	# --- mouse ---
	if tool_name == "mouse":
		action = str(normalized.get("action", "click")).lower()
		if action == "move":
			return SafetyDecision(
				risk="safe",
				reason="read-only pointer",
				summary="Moving cursor",
				details=[],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="confirm",
			reason="simulates input",
			summary=f"Mouse {action}",
			details=["can interact with UI elements"],
			normalized_args=normalized,
		)

	# --- keyboard ---
	if tool_name == "keyboard":
		action = str(normalized.get("action", "type")).lower()
		text = str(normalized.get("text", "")).lower()
		if action == "press":
			dangerous_keys = {"delete", "backspace", "alt+f4", "ctrl+w", "ctrl+shift+w"}
			if any(dk in text for dk in dangerous_keys):
				return SafetyDecision(
					risk="confirm",
					reason="destructive key combination",
					summary=f"Pressing {text}",
					details=["can close windows or delete content"],
					normalized_args=normalized,
				)
		return SafetyDecision(
			risk="confirm",
			reason="simulates input",
			summary=f"Keyboard {action}",
			details=["types or presses keys"],
			normalized_args=normalized,
		)

	# --- browser_open ---
	if tool_name == "browser_open":
		url = str(normalized.get("url", "")).lower()
		if "localhost" in url or "127.0.0.1" in url:
			return SafetyDecision(
				risk="safe",
				reason="local URL",
				summary="Opening localhost URL",
				details=[f"url: {url}"],
				normalized_args=normalized,
			)
		return SafetyDecision(
			risk="confirm",
			reason="opens external URL",
			summary="Opening external URL",
			details=[f"url: {url}"],
			normalized_args=normalized,
		)

	# --- browser_read / browser_screenshot / youtube_search / google_search ---
	if tool_name in ("browser_read", "browser_screenshot", "youtube_search", "google_search"):
		return SafetyDecision(
			risk="safe",
			reason="read-only",
			summary=f"Using {tool_name}",
			details=[],
			normalized_args=normalized,
		)

	# --- unknown tool ---
	return SafetyDecision(
		risk="confirm",
		reason="unknown tool",
		summary=f"Unknown tool: {tool_name}",
		details=["no safety policy defined — requires confirmation"],
		normalized_args=normalized,
	)
