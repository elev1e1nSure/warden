from __future__ import annotations

import asyncio
import json
import ssl
import urllib.error
import urllib.request
from typing import Any

from agent.tools.base import Tool

_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
_MAX_BYTES = 1_000_000


class HttpRequestTool(Tool):
    name = "http_request"
    description = (
        "Make an HTTP request to a REST API. Supports methods, custom headers, "
        "and a request body. Returns the status line and response body (capped). "
        "For reading web pages as text/markdown, prefer webfetch."
    )
    params = {
        "url": {"type": "string", "description": "Request URL (http/https)"},
        "method": {
            "type": "string",
            "description": "GET (default), POST, PUT, PATCH, DELETE, HEAD, OPTIONS",
        },
        "headers": {"type": "object", "description": "Request headers as a JSON object (optional)"},
        "body": {
            "type": "string",
            "description": "Request body; JSON string or raw text (optional)",
        },
        "timeout": {"type": "integer", "description": "Timeout in seconds (max 120, default 30)"},
    }

    def tool_definition(self) -> dict:
        d = super().tool_definition()
        d["function"]["parameters"]["required"] = ["url"]
        return d

    async def execute(self, args: dict[str, Any]) -> str:
        url = str(args.get("url", "")).strip()
        if not url.startswith("http://") and not url.startswith("https://"):
            return "error: URL must start with http:// or https://"
        method = str(args.get("method", "GET")).strip().upper() or "GET"
        if method not in _METHODS:
            return f"error: unsupported method '{method}'"
        timeout = min(int(args.get("timeout", 30)), 120)

        headers = args.get("headers") or {}
        if isinstance(headers, str):
            try:
                headers = json.loads(headers)
            except json.JSONDecodeError:
                return "error: headers must be a JSON object"
        if not isinstance(headers, dict):
            return "error: headers must be an object"
        headers = {str(k): str(v) for k, v in headers.items()}
        headers.setdefault("User-Agent", "warden/1.0")

        body = args.get("body")
        data = body.encode("utf-8") if isinstance(body, str) and body != "" else None

        def _send() -> tuple[int, str, str]:
            req = urllib.request.Request(url, data=data, headers=headers, method=method)
            ctx = ssl.create_default_context()
            try:
                resp_cm = urllib.request.urlopen(req, context=ctx, timeout=timeout)
            except urllib.error.HTTPError as e:
                # HTTPError is also a response: surface status + body
                raw = e.read(_MAX_BYTES)
                return e.code, e.reason or "", raw.decode("utf-8", errors="replace")
            with resp_cm as resp:
                raw = resp.read(_MAX_BYTES)
                return resp.status, resp.reason or "", raw.decode("utf-8", errors="replace")

        try:
            status, reason, text = await asyncio.to_thread(_send)
        except urllib.error.URLError as e:
            return f"error: {e.reason}"
        except TimeoutError:
            return f"error: timeout {timeout}s"
        except Exception as e:
            return f"error: {e}"
        head = f"HTTP {status} {reason}".strip()
        return f"{head}\n{text[:10000]}" if text else head
