from __future__ import annotations

import json
import os
import re
from abc import ABC, abstractmethod
from typing import Any

_ANSI = re.compile(r"\x1b\[[0-9;]*[mGKHFJABCDsu]|\x1b\][^\x07]*\x07|\x1b=|\x1b>")


def _diff_stats(old: str, new: str) -> str:
    import difflib

    added = removed = 0
    for line in difflib.unified_diff(old.splitlines(), new.splitlines(), lineterm=""):
        if line.startswith("+") and not line.startswith("+++"):
            added += 1
        elif line.startswith("-") and not line.startswith("---"):
            removed += 1
    if added == 0 and removed == 0:
        return ""
    return f"+{added} -{removed}"


def _diff_full(old: str, new: str, path: str) -> str:
    import difflib

    lines = list(
        difflib.unified_diff(
            old.splitlines(),
            new.splitlines(),
            fromfile=f"a/{path}",
            tofile=f"b/{path}",
            lineterm="",
        )
    )
    return "\n".join(lines)


class ToolResult:
    """Wraps a tool result string with an optional unified diff."""

    def __init__(self, result: str, diff: str | None = None):
        self.result = result
        self.diff = diff

    def __str__(self) -> str:
        return self.result

    def __contains__(self, item: str) -> bool:
        return item in self.result

    def lower(self) -> str:
        return self.result.lower()


def _clean(text: str) -> str:
    """Strip ANSI codes and collapse \r-overwrites"""
    text = _ANSI.sub("", text)
    lines = []
    for line in text.split("\n"):
        parts = line.split("\r")
        cleaned = parts[-1].rstrip()
        if cleaned:
            lines.append(cleaned)
    return "\n".join(lines)


def _in_cwd(path: str) -> bool:
    try:
        if os.name != "nt" and isinstance(path, str):
            path = path.replace("\\", "/")
        cwd = os.getcwd()
        if not cwd.endswith(os.sep):
            cwd += os.sep
        return os.path.abspath(path).startswith(cwd)
    except Exception:
        return False


class Tool(ABC):
    name: str
    description: str
    params: dict[str, Any]

    @abstractmethod
    async def execute(self, args: dict[str, Any]) -> str: ...

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return False

    def tool_definition(self) -> dict:
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


def parse_args(arguments: Any) -> dict:
    if isinstance(arguments, dict):
        return arguments
    try:
        result = json.loads(arguments)
        if isinstance(result, dict):
            return result
        return {}
    except Exception:
        return {}


def is_ssrf_safe_url(url: str) -> bool:
    import ipaddress
    import urllib.parse
    import socket

    if not url:
        return False

    url_strip = url.strip()
    url_lower = url_strip.lower()

    if url_lower.startswith("file:") or "file://" in url_lower:
        return False

    try:
        parsed = urllib.parse.urlparse(url_strip)
    except Exception:
        return False

    if not parsed.scheme:
        try:
            parsed = urllib.parse.urlparse("http://" + url_strip)
        except Exception:
            return False

    scheme = parsed.scheme.lower()
    if scheme == "file":
        return False

    hostname = parsed.hostname
    if not hostname:
        if scheme in ("http", "https"):
            return False
        return True

    hostname = hostname.lower().strip()

    if hostname in ("localhost", "localhost.", "loopback", "loopback."):
        return False

    blocked_networks = [
        ipaddress.ip_network("127.0.0.0/8"),
        ipaddress.ip_network("10.0.0.0/8"),
        ipaddress.ip_network("172.16.0.0/12"),
        ipaddress.ip_network("192.168.0.0/16"),
        ipaddress.ip_network("169.254.0.0/16"),
        ipaddress.ip_network("::1/128"),
    ]

    def is_blocked_ip(ip_str: str) -> bool:
        try:
            ip = ipaddress.ip_address(ip_str.strip("[]"))
            if ip.is_loopback or ip.is_private or ip.is_link_local:
                return True
            for network in blocked_networks:
                if ip in network:
                    return True
        except ValueError:
            pass
        return False

    if is_blocked_ip(hostname):
        return False

    try:
        for _, _, _, _, sockaddr in socket.getaddrinfo(hostname, None):
            resolved_ip = sockaddr[0]
            if is_blocked_ip(resolved_ip):
                return False
    except Exception:
        pass

    return True

