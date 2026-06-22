"""Tests for LspTool — covers server resolution, formatting, transport framing.
Does NOT spawn a real language server; the LSP stdio protocol is exercised
against a fake binary that pipes back canned JSON-RPC responses.
"""

from __future__ import annotations

import os
import shutil
import sys
from pathlib import Path

import pytest

# ── Server resolution ────────────────────────────────────────────────────────


def test_file_to_lang_no_server(tmp_path, monkeypatch):
    """If neither gopls nor pyright is on PATH, return Nones."""
    from agent.tools.lsp import _file_to_lang

    monkeypatch.setattr(shutil, "which", lambda cmd: None)
    cmd, args, lang, ext = _file_to_lang("foo.go")
    assert cmd is None
    assert ext == ".go"
    cmd, args, lang, ext = _file_to_lang("foo.py")
    assert cmd is None
    assert ext == ".py"


def test_file_to_lang_go():
    from agent.tools.lsp import _file_to_lang

    if not shutil.which("gopls"):
        pytest.skip("gopls not on PATH")
    cmd, args, lang, ext = _file_to_lang("foo.go")
    assert cmd == "gopls"
    assert lang == "go"


def test_file_to_lang_unknown_ext():
    from agent.tools.lsp import _file_to_lang

    cmd, args, lang, ext = _file_to_lang("foo.rs")
    assert cmd is None
    assert ext == ".rs"


# ── URI / path helpers ───────────────────────────────────────────────────────


@pytest.mark.skipif(sys.platform == "win32", reason="unix-only path test")
def test_path_to_uri_unix():
    from agent.tools.lsp import _path_to_uri

    assert _path_to_uri("/tmp/foo.py") == "file:///tmp/foo.py"


@pytest.mark.skipif(sys.platform != "win32", reason="windows-only path test")
def test_path_to_uri_windows_strips_leading_slash(monkeypatch):
    from agent.tools.lsp import _path_to_uri

    monkeypatch.setattr(os, "name", "nt")
    assert _path_to_uri("C:/foo/bar.py") == "file:///C:/foo/bar.py"


@pytest.mark.skipif(sys.platform == "win32", reason="unix-only path test")
def test_uri_to_path_unix():
    from agent.tools.lsp import _uri_to_path

    assert _uri_to_path("file:///tmp/foo.py") == "/tmp/foo.py"


@pytest.mark.skipif(sys.platform != "win32", reason="windows-only path test")
def test_uri_to_path_windows():
    from agent.tools.lsp import _uri_to_path

    assert _uri_to_path("file:///C:/foo/bar.py") == "C:/foo/bar.py"


# ── Result formatting ────────────────────────────────────────────────────────


def test_format_result_empty_list():
    from agent.tools.lsp import _format_result

    assert "No results" in _format_result("goToDefinition", [])


def test_format_result_none():
    from agent.tools.lsp import _format_result

    assert "No results" in _format_result("hover", None)


def test_format_result_locations():
    from agent.tools.lsp import _format_result

    locs = [
        {
            "uri": "file:///tmp/foo.py",
            "range": {
                "start": {"line": 4, "character": 2},
                "end": {"line": 4, "character": 7},
            },
        }
    ]
    out = _format_result("goToDefinition", locs)
    # 0-based -> 1-based
    assert '"line": 5' in out
    assert '"character": 3' in out


def test_format_result_hover_string():
    from agent.tools.lsp import _format_result

    out = _format_result("hover", {"contents": "int x = 1"})
    assert "int x = 1" in out


def test_format_result_hover_markdown():
    from agent.tools.lsp import _format_result

    out = _format_result("hover", {"contents": {"value": "**bold**", "language": "python"}})
    assert "**bold**" in out


def test_format_result_hover_list():
    from agent.tools.lsp import _format_result

    out = _format_result("hover", {"contents": [{"value": "first"}, "second"]})
    assert "first" in out
    assert "second" in out


# ── Transport: canned fake LSP server (subprocess pipes) ─────────────────────


def _make_fake_lsp_script(tmp_path: Path, responses: list) -> Path:
    """Write a small Python script that emulates an LSP server over stdio.

    `responses` is a list of dicts; for each request id we send back, the server
    echoes a `result` based on what was requested. We only need to handle
    `initialize` (returns empty caps) and one more request.
    """
    script = tmp_path / "fake_lsp.py"
    script.write_text(
        "import json, sys\n"
        "def send(msg):\n"
        "    body = json.dumps(msg).encode()\n"
        "    sys.stdout.buffer.write(f'Content-Length: {len(body)}\\r\\n\\r\\n'.encode() + body)\n"
        "    sys.stdout.buffer.flush()\n"
        "def read():\n"
        "    line = sys.stdin.readline()\n"
        "    while line and not line.strip():\n"
        "        line = sys.stdin.readline()\n"
        "    if not line: return None\n"
        "    n = int(line.split(':')[1].strip())\n"
        "    sys.stdin.readline()  # blank line\n"
        "    return json.loads(sys.stdin.read(n))\n"
        "while True:\n"
        "    msg = read()\n"
        "    if msg is None: break\n"
        "    method = msg.get('method', '')\n"
        "    if method == 'initialize':\n"
        "        send({'jsonrpc': '2.0', 'id': msg['id'], 'result': {'capabilities': {}}})\n"
        "    elif method == 'shutdown':\n"
        "        send({'jsonrpc': '2.0', 'id': msg['id'], 'result': None})\n"
        "        continue\n"
        "    elif method == 'exit':\n"
        "        break\n"
        "    elif 'id' in msg:\n"
        "        # canned result, fetched by request id\n"
        "        idx = msg['id'] - 1\n"
        "        if 0 <= idx < len(REPLACEMENT):\n"
        "            send({'jsonrpc': '2.0', 'id': msg['id'], 'result': REPLACEMENT[idx]})\n"
        "        else:\n"
        "            send({'jsonrpc': '2.0', 'id': msg['id'], 'result': None})\n"
    )
    return script


@pytest.mark.skipif(sys.platform == "win32", reason="LSP roundtrip flaky on Windows runners")
@pytest.mark.asyncio
async def test_lsp_client_definition_roundtrip(tmp_path, monkeypatch):
    """Spawn a fake LSP server, run a definition query, verify transport + framing."""
    from agent.tools.lsp import _LspClient

    script = _make_fake_lsp_script(tmp_path, None)
    # pre-create the replacement as a list literal
    # we rewrite the script to inline the replacement
    script.write_text(
        "import json, sys\n"
        "REPLACEMENT = [{\n"
        "  'uri': 'file:///tmp/foo.py',\n"
        "  'range': {'start': {'line': 9, 'character': 0}, 'end': {'line': 9, 'character': 4}}\n"
        "}]\n" + "def send(msg):\n"
        "    body = json.dumps(msg).encode()\n"
        "    sys.stdout.buffer.write(f'Content-Length: {len(body)}\\r\\n\\r\\n'.encode() + body)\n"
        "    sys.stdout.buffer.flush()\n"
        "def read():\n"
        "    line = sys.stdin.readline()\n"
        "    while line and not line.strip():\n"
        "        line = sys.stdin.readline()\n"
        "    if not line: return None\n"
        "    n = int(line.split(':')[1].strip())\n"
        "    sys.stdin.readline()\n"
        "    return json.loads(sys.stdin.read(n))\n"
        "while True:\n"
        "    msg = read()\n"
        "    if msg is None: break\n"
        "    method = msg.get('method', '')\n"
        "    if method == 'initialize':\n"
        "        send({'jsonrpc': '2.0', 'id': msg['id'], 'result': {'capabilities': {}}})\n"
        "    elif method == 'shutdown':\n"
        "        send({'jsonrpc': '2.0', 'id': msg['id'], 'result': None})\n"
        "    elif method == 'exit':\n"
        "        break\n"
        "    elif 'id' in msg:\n"
        "        idx = msg['id'] - 1\n"
        "        if 0 <= idx < len(REPLACEMENT):\n"
        "            send({'jsonrpc': '2.0', 'id': msg['id'], 'result': REPLACEMENT[idx]})\n"
        "        else:\n"
        "            send({'jsonrpc': '2.0', 'id': msg['id'], 'result': None})\n"
    )

    workspace = tmp_path / "ws"
    workspace.mkdir()
    (workspace / "foo.py").write_text("def hello():\n    pass\n")

    client = _LspClient(sys.executable, [str(script)], str(workspace))
    await client.start("file://" + str(workspace))
    result = await client.definition("file://" + str(workspace).replace("\\", "/") + "/foo.py", 0, 0)
    await client.stop()

    assert isinstance(result, list)
    assert result and result[0]["uri"].endswith("foo.py")
    assert result[0]["range"]["start"]["line"] == 9


# ── Tool surface ─────────────────────────────────────────────────────────────


def test_lsp_tool_params():
    from agent.tools.lsp import _OPERATIONS, LspTool

    t = LspTool()
    defn = t.tool_definition()
    props = defn["function"]["parameters"]["properties"]
    assert set(props.keys()) >= {"operation", "filePath", "line", "character", "query"}
    assert props["operation"]["enum"] == list(_OPERATIONS)


def test_lsp_tool_unknown_operation(tmp_path, monkeypatch):
    import asyncio

    from agent.tools.lsp import LspTool

    monkeypatch.setattr(shutil, "which", lambda c: "/usr/bin/fake" if c == "gopls" else None)
    result = asyncio.run(
        LspTool().execute(
            {
                "operation": "bogusOp",
                "filePath": str(tmp_path / "foo.go"),
                "line": 1,
                "character": 1,
            }
        )
    )
    assert "unknown operation" in result


def test_lsp_tool_no_server_for_ext(tmp_path, monkeypatch):
    import asyncio

    from agent.tools.lsp import LspTool

    monkeypatch.setattr(shutil, "which", lambda c: None)
    rs = tmp_path / "x.go"
    rs.write_text("package x")
    result = asyncio.run(
        LspTool().execute(
            {
                "operation": "goToDefinition",
                "filePath": str(rs),
                "line": 1,
                "character": 1,
            }
        )
    )
    assert "no LSP server" in result


def test_lsp_tool_missing_file(tmp_path, monkeypatch):
    import asyncio

    from agent.tools.lsp import LspTool

    monkeypatch.setattr(shutil, "which", lambda c: "/usr/bin/fake" if c == "gopls" else None)
    result = asyncio.run(
        LspTool().execute(
            {
                "operation": "goToDefinition",
                "filePath": str(tmp_path / "ghost.go"),
                "line": 1,
                "character": 1,
            }
        )
    )
    assert "not found" in result


def test_lsp_tool_no_filepath():
    import asyncio

    from agent.tools.lsp import LspTool

    result = asyncio.run(
        LspTool().execute(
            {
                "operation": "goToDefinition",
                "line": 1,
                "character": 1,
            }
        )
    )
    assert "filePath" in result
