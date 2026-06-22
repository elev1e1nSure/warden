"""LSP tool — exposes Language Server Protocol queries to the LLM.

Mirrors the opencode `lsp` tool: goToDefinition, findReferences, hover,
documentSymbol, workspaceSymbol, goToImplementation, call hierarchy.
Spawns the appropriate language server over stdio (JSON-RPC 3.17) per
invocation, shuts it down before returning. The tool is best-effort: if
the binary is not on PATH, it returns a clean error instead of crashing
the agent.
"""

from __future__ import annotations

import asyncio
import contextlib
import json
import os
import shutil
import subprocess
from pathlib import Path
from typing import Any

from agent.tools.base import Tool

# Map file extension -> (command, args, languageId)
_SERVERS: dict[str, tuple[str, list[str], str]] = {
    ".go": ("gopls", [], "go"),
    ".py": ("pyright-langserver", ["--stdio"], "python"),
}

# Fallback: some installs use the plain `pylsp` binary
_PY_FALLBACK: tuple[str, list[str], str] = ("pylsp", [], "python")

_OPERATIONS = (
    "goToDefinition",
    "findReferences",
    "hover",
    "documentSymbol",
    "workspaceSymbol",
    "goToImplementation",
    "prepareCallHierarchy",
    "incomingCalls",
    "outgoingCalls",
)

_LSP_INIT_TIMEOUT = 10.0
_LSP_QUERY_TIMEOUT = 15.0
_LSP_PROTOCOL_VERSION = "3.17"


def _file_to_lang(path: str) -> tuple[str | None, str | None, str | None, str | None]:
    """Resolve a file path to (server_cmd, server_args, languageId, ext) or Nones."""
    ext = os.path.splitext(path)[1].lower()
    if ext == ".go":
        cmd, args, lang = _SERVERS[".go"]
        if shutil.which(cmd):
            return cmd, args, lang, ext
        return None, None, None, ext
    if ext == ".py":
        cmd, args, lang = _SERVERS[".py"]
        if shutil.which(cmd):
            return cmd, args, lang, ext
        cmd, args, lang = _PY_FALLBACK
        if shutil.which(cmd):
            return cmd, args, lang, ext
        return None, None, None, ext
    return None, None, None, ext


def _path_to_uri(path: str) -> str:
    abspath = str(Path(path).resolve())
    if os.name == "nt" and abspath.startswith("\\"):
        abspath = abspath.lstrip("\\")
    abspath = abspath.replace("\\", "/")
    if not abspath.startswith("/"):
        abspath = "/" + abspath
    return "file://" + abspath


class _LspClient:
    """Minimal stdio JSON-RPC 3.17 client. One server per instance."""

    def __init__(self, cmd: str, args: list[str], cwd: str) -> None:
        self._cmd = cmd
        self._args = args
        self._cwd = cwd
        self._proc: asyncio.subprocess.Process | None = None
        self._id = 0
        self._reader_task: asyncio.Task | None = None
        self._responses: dict[int, asyncio.Future] = {}
        self._notifications: list[dict] = []
        self._stderr_buf: list[str] = []

    async def start(self, root_uri: str) -> None:
        try:
            self._proc = await asyncio.create_subprocess_exec(
                self._cmd,
                *self._args,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self._cwd,
            )
        except FileNotFoundError as e:
            raise RuntimeError(f"LSP server '{self._cmd}' not found on PATH") from e
        self._reader_task = asyncio.create_task(self._read_loop())
        await asyncio.wait_for(
            self._request(
                "initialize",
                {
                    "processId": os.getpid(),
                    "clientInfo": {"name": "warden", "version": "0.1"},
                    "rootUri": root_uri,
                    "capabilities": {
                        "textDocument": {
                            "definition": {"dynamicRegistration": False},
                            "references": {"dynamicRegistration": False},
                            "hover": {
                                "dynamicRegistration": False,
                                "contentFormat": ["plaintext", "markdown"],
                            },
                            "documentSymbol": {
                                "dynamicRegistration": False,
                                "hierarchicalDocumentSymbolSupport": True,
                            },
                            "implementation": {"dynamicRegistration": False},
                            "callHierarchy": {"dynamicRegistration": False},
                            "synchronization": {"dynamicRegistration": False, "didSave": True},
                        },
                        "workspace": {
                            "symbol": {"dynamicRegistration": False},
                        },
                    },
                    "initializationOptions": {},
                    "trace": "off",
                    "workspaceFolders": [{"uri": root_uri, "name": Path(root_uri.replace("file://", "")).name}],
                },
            ),
            timeout=_LSP_INIT_TIMEOUT,
        )
        # initialized is a notification, not a request
        await self._notify("initialized", {})

    async def stop(self) -> None:
        if self._proc is None:
            return
        with contextlib.suppress(Exception):
            await asyncio.wait_for(self._request("shutdown", None), timeout=2.0)
        with contextlib.suppress(Exception):
            self._proc.stdin.close()
        with contextlib.suppress(Exception):
            self._proc.terminate()
            await asyncio.wait_for(self._proc.wait(), timeout=2.0)
        with contextlib.suppress(Exception):
            self._proc.kill()
        if self._reader_task is not None:
            self._reader_task.cancel()
        self._proc = None

    async def definition(self, uri: str, line: int, char: int) -> list[dict]:
        return await self._call(
            "textDocument/definition",
            {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": char},
            },
        )

    async def references(self, uri: str, line: int, char: int, include_decl: bool = True) -> list[dict]:
        return await self._call(
            "textDocument/references",
            {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": char},
                "context": {"includeDeclaration": include_decl},
            },
        )

    async def hover(self, uri: str, line: int, char: int) -> dict | None:
        return await self._call(
            "textDocument/hover",
            {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": char},
            },
        )

    async def document_symbol(self, uri: str) -> list[dict]:
        return await self._call("textDocument/documentSymbol", {"textDocument": {"uri": uri}})

    async def workspace_symbol(self, query: str) -> list[dict]:
        return await self._call("workspace/symbol", {"query": query})

    async def implementation(self, uri: str, line: int, char: int) -> list[dict]:
        return await self._call(
            "textDocument/implementation",
            {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": char},
            },
        )

    async def prepare_call_hierarchy(self, uri: str, line: int, char: int) -> list[dict]:
        return await self._call(
            "textDocument/prepareCallHierarchy",
            {
                "textDocument": {"uri": uri},
                "position": {"line": line, "character": char},
            },
        )

    async def incoming_calls(self, item: dict) -> list[dict]:
        return await self._call("callHierarchy/incomingCalls", {"item": item})

    async def outgoing_calls(self, item: dict) -> list[dict]:
        return await self._call("callHierarchy/outgoingCalls", {"item": item})

    # ── transport ──────────────────────────────────────────────────────────────

    async def _read_loop(self) -> None:
        assert self._proc is not None and self._proc.stdout is not None
        loop = asyncio.get_event_loop()
        reader = self._proc.stdout
        while True:
            try:
                headers = await loop.run_in_executor(None, _read_headers_sync, reader)
                if headers is None:
                    return
                length = int(headers.get("Content-Length", "0"))
                if length <= 0:
                    continue
                raw = await loop.run_in_executor(None, _read_exact_sync, reader, length)
                if raw is None:
                    return
            except Exception:
                return
            try:
                msg = json.loads(raw.decode("utf-8", errors="replace"))
            except Exception:
                continue
            if "id" in msg and ("result" in msg or "error" in msg):
                fut = self._responses.pop(msg["id"], None)
                if fut is not None and not fut.done():
                    fut.set_result(msg)
            else:
                # notification
                self._notifications.append(msg)

    async def _request(self, method: str, params: Any) -> dict:
        self._id += 1
        req_id = self._id
        payload = {"jsonrpc": "2.0", "id": req_id, "method": method, "params": params}
        fut: asyncio.Future = asyncio.get_event_loop().create_future()
        self._responses[req_id] = fut
        body = json.dumps(payload).encode("utf-8")
        header = f"Content-Length: {len(body)}\r\n\r\n".encode("ascii")
        assert self._proc is not None and self._proc.stdin is not None
        try:
            self._proc.stdin.write(header + body)
            await self._proc.stdin.drain()
        except Exception as e:
            self._responses.pop(req_id, None)
            raise RuntimeError(f"LSP write failed: {e}") from e
        try:
            msg = await asyncio.wait_for(fut, timeout=_LSP_QUERY_TIMEOUT)
        except TimeoutError as e:
            self._responses.pop(req_id, None)
            raise RuntimeError(f"LSP '{method}' timed out") from e
        if "error" in msg:
            raise RuntimeError(f"LSP error: {msg['error']}")
        return msg.get("result")

    async def _call(self, method: str, params: Any) -> Any:
        return await self._request(method, params)

    async def _notify(self, method: str, params: Any) -> None:
        payload = {"jsonrpc": "2.0", "method": method, "params": params}
        body = json.dumps(payload).encode("utf-8")
        header = f"Content-Length: {len(body)}\r\n\r\n".encode("ascii")
        assert self._proc is not None and self._proc.stdin is not None
        self._proc.stdin.write(header + body)
        await self._proc.stdin.drain()


def _read_headers_sync(stream) -> dict[str, str] | None:
    headers: dict[str, str] = {}
    while True:
        line = stream.readline()
        if not line:
            return None
        line = line.decode("ascii", errors="replace").rstrip("\r\n")
        if line == "":
            return headers
        if ":" in line:
            k, v = line.split(":", 1)
            headers[k.strip()] = v.strip()


def _read_exact_sync(stream, n: int) -> bytes | None:
    buf = bytearray()
    while len(buf) < n:
        chunk = stream.read(n - len(buf))
        if not chunk:
            return None
        buf.extend(chunk)
    return bytes(buf)


def _uri_to_path(uri: str) -> str:
    if uri.startswith("file://"):
        p = uri[len("file://") :]
        if os.name == "nt" and p.startswith("/"):
            p = p[1:]
        return p
    return uri


def _location_compact(loc: dict) -> dict:
    uri = loc.get("uri") or loc.get("targetUri") or ""
    path = _uri_to_path(uri)
    # LSP ranges use 0-based; LLM wants 1-based
    r = loc.get("range") or loc.get("targetSelectionRange") or {}
    start = r.get("start", {})
    return {
        "path": path,
        "line": (start.get("line", 0) + 1),
        "character": (start.get("character", 0) + 1),
        "endLine": (r.get("end", {}).get("line", 0) + 1),
        "endCharacter": (r.get("end", {}).get("character", 0) + 1),
    }


def _format_result(operation: str, result: Any) -> str:
    if result is None:
        return f"No results found for {operation}"
    if isinstance(result, list):
        if not result:
            return f"No results found for {operation}"
        compact = []
        for item in result:
            if isinstance(item, dict) and "uri" in item and "range" in item:
                compact.append(_location_compact(item))
            elif isinstance(item, dict) and "location" in item:
                # callHierarchy items wrap a location
                inner = _location_compact(item["location"])
                inner["name"] = item.get("name", "")
                inner["kind"] = item.get("kind", "")
                compact.append(inner)
            else:
                compact.append(item)
        return json.dumps(compact, indent=2)
    if isinstance(result, dict):
        contents = result.get("contents")
        if contents is not None:
            if isinstance(contents, str):
                return contents
            if isinstance(contents, list):
                return "\n\n".join((c.get("value") if isinstance(c, dict) else str(c)) for c in contents)
            if isinstance(contents, dict):
                return contents.get("value", json.dumps(contents, indent=2))
        return json.dumps(result, indent=2)
    return str(result)


class LspTool(Tool):
    name = "lsp"
    description = (
        "Query the language server (LSP) for code intelligence. "
        "Operations: goToDefinition, findReferences, hover, documentSymbol, "
        "workspaceSymbol, goToImplementation, prepareCallHierarchy, incomingCalls, outgoingCalls. "
        "Supported: .go (gopls), .py (pyright-langserver / pylsp). "
        "line and character are 1-based. For workspaceSymbol, set filePath to the workspace root and query to the search string."
    )
    params = {
        "operation": {
            "type": "string",
            "enum": list(_OPERATIONS),
            "description": "The LSP operation to perform",
        },
        "filePath": {"type": "string", "description": "Absolute or relative path to the file"},
        "line": {"type": "integer", "description": "Line number (1-based)"},
        "character": {"type": "integer", "description": "Character offset (1-based)"},
        "query": {
            "type": "string",
            "description": "Search query for workspaceSymbol (empty string requests all symbols)",
        },
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return False

    async def execute(self, args: dict[str, Any]) -> str:
        operation = args.get("operation", "")
        if operation not in _OPERATIONS:
            return f"error: unknown operation '{operation}'"

        file_path = args.get("filePath", "")
        if not file_path:
            return "error: filePath is required"
        if not os.path.isabs(file_path):
            file_path = os.path.abspath(file_path)

        cmd, server_args, lang, ext = _file_to_lang(file_path)
        if cmd is None:
            return f"error: no LSP server configured for {ext or 'this file type'} (need gopls / pyright-langserver on PATH)"

        if not os.path.exists(file_path):
            return f"error: file not found: {file_path}"

        workspace_root = os.getcwd()
        root_uri = _path_to_uri(workspace_root)
        file_uri = _path_to_uri(file_path)

        client = _LspClient(cmd, server_args, workspace_root)
        try:
            await client.start(root_uri)
        except Exception as e:
            return f"error: failed to start {cmd}: {e}"

        try:
            line = max(0, int(args.get("line", 1)) - 1)
            char = max(0, int(args.get("character", 1)) - 1)
            query = args.get("query", "") or ""

            if operation == "goToDefinition":
                result = await client.definition(file_uri, line, char)
            elif operation == "findReferences":
                result = await client.references(file_uri, line, char)
            elif operation == "hover":
                result = await client.hover(file_uri, line, char)
            elif operation == "documentSymbol":
                result = await client.document_symbol(file_uri)
            elif operation == "workspaceSymbol":
                result = await client.workspace_symbol(query)
            elif operation == "goToImplementation":
                result = await client.implementation(file_uri, line, char)
            elif operation == "prepareCallHierarchy":
                result = await client.prepare_call_hierarchy(file_uri, line, char)
            elif operation == "incomingCalls":
                items = await client.prepare_call_hierarchy(file_uri, line, char)
                if not items:
                    result = []
                else:
                    result = []
                    for it in items:
                        result.extend(await client.incoming_calls(it))
            elif operation == "outgoingCalls":
                items = await client.prepare_call_hierarchy(file_uri, line, char)
                if not items:
                    result = []
                else:
                    result = []
                    for it in items:
                        result.extend(await client.outgoing_calls(it))
            else:
                result = None
        except Exception as e:
            return f"error: LSP query failed: {e}"
        finally:
            await client.stop()

        return _format_result(operation, result)
