"""PowerShell command classification."""

from __future__ import annotations

import re
from typing import List

_DELETE_ALIASES = {"rm", "del", "erase", "rmdir", "rd", "ri", "remove-item"}
_KILL_ALIASES = {"kill", "spps", "stop-process"}
_INVOKE_ALIASES = {"iex", "invoke-expression"}

_BLOCKED_CMDLETS = {
    "remove-item", "ri", "rmdir", "del", "erase", "rd",
    "stop-process", "spps", "kill", "taskkill",
    "format", "mkfs", "diskpart",
    "set-service", "sc", "sc.exe",
    "new-service", "remove-service",
    "reg", "reg.exe", "set-itemproperty", "remove-itemproperty", "new-itemproperty",
    "invoke-expression", "iex",
    "invoke-command",
    "clear-content", "clc",
    "set-executionpolicy",
    "netsh",
    "bcdedit",
    "cipher",
}

_CONFIRM_CMDLETS = {
    "set-content", "add-content", "out-file",
    "copy-item", "cp", "cpi", "move-item", "mv", "mi",
    "rename-item", "rni", "ren",
    "start-process", "saps", "start",
    "winget", "npm", "pnpm", "pip", "go",
    "git", "node", "python", "py",
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

_ENCODED_RE = re.compile(
    r"-[eE]nc(?:oded)?[Cc]ommand\b|/[eE]:\b|-enc\b|-[eE][cCnN]\b",
    re.IGNORECASE,
)
_REMOTE_PIPE_RE = re.compile(
    r"(iwr|irm|invoke-webrequest|invoke-restmethod|curl\.exe|wget\.exe)\s+.*\|\s*iex",
    re.IGNORECASE,
)
_CHAIN_RE = re.compile(r"[;&\r\n]")


def _normalize(command: str) -> str:
    text = command.replace("`\r\n", " ").replace("`\n", " ").replace("`\r", " ")
    return re.sub(r"`\s+", " ", text).strip()


def _tokens(command: str) -> List[str]:
    parts = re.split(r"[\s|;`&|]+", command)
    return [t.lower().strip("\t\r\n'\"") for t in parts if t]


def _has_any(tokens: List[str], candidates: set) -> bool:
    return any(t in candidates for t in tokens)


def classify(command: str) -> tuple[str, str, List[str]]:
    """Classify a PowerShell command. Returns (risk, reason, details)."""
    norm = _normalize(command)
    tokens = _tokens(norm)

    if _ENCODED_RE.search(norm):
        return "blocked", "encoded command execution", ["uses -EncodedCommand or similar"]

    if _REMOTE_PIPE_RE.search(norm):
        return "blocked", "remote script execution via iex", ["downloads remote content and executes it"]

    nested = re.search(
        r"(?:cmd\.exe|cmd)\s+/[cCkK]\s+(?:['\"])?(.+?)(?:['\"])?$|"
        r"(?:pwsh|powershell)\s+(?:-[cC]ommand|-c)\s+['\"]?(.+?)['\"]?$|"
        r"(?:bash|sh)\s+-c\s+['\"]?(.+?)['\"]?$",
        norm, re.IGNORECASE,
    )
    if nested:
        inner = next((g for g in nested.groups() if g), "")
        if inner:
            return classify(inner)

    if re.search(r"\b(rd|rmdir|del|erase|deltree)\b.*/[fFsSqQ]\b", norm, re.IGNORECASE):
        return "blocked", "destructive cmd.exe command", ["uses cmd-style delete with force/recurse flags"]

    if re.search(r"\b(format\s+[a-z]:|mkfs|diskpart|cipher\s+/w)\b", norm, re.IGNORECASE):
        return "blocked", "disk destruction command", ["can erase drives or volumes"]

    if re.search(r"\bshutdown\b.*\s/[srph]\b", norm, re.IGNORECASE):
        return "blocked", "system power command", ["shuts down, restarts, or powers off the machine"]

    if re.search(r"git\s+(reset\s+--hard|clean\s+-fd|push\s+--force|push\s+-f|branch\s+-D)", norm, re.IGNORECASE):
        return "blocked", "destructive git command", ["matched destructive git operation"]

    if re.search(
        r"\b(reg\s+(add|delete|edit)|set-itemproperty|remove-itemproperty|"
        r"new-itemproperty|netsh\s+advfirewall)\b",
        norm, re.IGNORECASE,
    ):
        return "blocked", "system/registry modification", ["changes system configuration"]

    if re.search(
        r"\b(remove-item|rm|del|rmdir|rd|ri)\b.*(-recurse|-r)\s+.*(-force|-f)|"
        r"\b(remove-item|rm|del|rmdir|rd|ri)\b.*(-force|-f)\s+.*(-recurse|-r)",
        norm, re.IGNORECASE,
    ):
        return "blocked", "recursive forced deletion", ["uses -Recurse and -Force on delete"]

    if _CHAIN_RE.search(norm):
        return "confirm", "chained command", ["contains command chains (;/&)"]

    exe = tokens[0] if tokens else ""
    rest = tokens[1:]

    if exe in _DELETE_ALIASES:
        return "confirm", "file deletion", ["deletes files or directories"]
    if exe in _KILL_ALIASES or "taskkill" in tokens:
        return "confirm", "process termination", ["stops a running process"]
    if exe in _INVOKE_ALIASES:
        return "blocked", "code evaluation", ["Invoke-Expression can execute arbitrary code"]
    if exe in _BLOCKED_CMDLETS or _has_any(tokens, _BLOCKED_CMDLETS):
        return "blocked", "restricted system command", [f"command '{exe}' is blocked in leashed mode"]
    if exe in {"winget", "npm", "pnpm", "pip", "uv", "gem", "cargo"}:
        return "confirm", "package installation or modification", [f"uses {exe}"]
    if exe == "git":
        sub = rest[0] if rest else ""
        if sub in {"status", "diff", "log", "show", "branch", "tag", "config", "remote", "stash", "ls-files"}:
            return "safe", "read-only git command", [f"git {sub}"]
        return "confirm", "git command", ["git may change repository state"]
    if exe == "go":
        sub = rest[0] if rest else ""
        if sub in {"test", "fmt", "vet", "env", "version", "mod", "list", "doc"}:
            return "safe", "read-only go command", [f"go {sub}"]
        return "confirm", "go command", ["go may change project state"]
    if exe in {"python", "py"}:
        if "-m" in tokens and "py_compile" in tokens:
            return "safe", "read-only python check", ["py_compile"]
        return "confirm", "python execution", ["python may execute arbitrary code"]
    if exe in _CONFIRM_CMDLETS or _has_any(tokens, _CONFIRM_CMDLETS):
        return "confirm", "file or system modification", [f"command '{exe}' changes state"]
    if exe in _SAFE_CMDLETS or _has_any(tokens, _SAFE_CMDLETS):
        return "safe", "read-only command", []

    return "confirm", "unknown command", ["no safety policy defined for this command"]
