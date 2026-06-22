"""Tests for the tool_runner screenshot -> vision pipeline."""

from __future__ import annotations

import base64
import io

from agent.tool_runner import CU_MAX_SIDE, _encode_image, _extract_saved_path

# ── _extract_saved_path ───────────────────────────────────────────────────────


def test_extract_saved_path_new_screenshot_format(tmp_path):
    f = tmp_path / "screenshot_x.png"
    f.write_bytes(b"\x89PNG")
    result = f"saved: {f} (screen 1920x1080, shown 1280x720)"
    assert _extract_saved_path(result) == str(f)


def test_extract_saved_path_browser_format(tmp_path):
    f = tmp_path / "browser_x.png"
    f.write_bytes(b"x")
    assert _extract_saved_path(f"saved: {f}") == str(f)


def test_extract_saved_path_missing_file(tmp_path):
    f = tmp_path / "nope.png"
    assert _extract_saved_path(f"saved: {f} (screen 1x1, shown 1x1)") is None


def test_extract_saved_path_non_saved():
    assert _extract_saved_path("error: nope") is None


# ── _encode_image ─────────────────────────────────────────────────────────────


def test_encode_image_downscales_to_cu_max_side(tmp_path):
    from PIL import Image

    p = tmp_path / "big.png"
    Image.new("RGB", (2560, 1440), "white").save(p)
    b64 = _encode_image(str(p))
    assert b64
    out = Image.open(io.BytesIO(base64.b64decode(b64)))
    assert max(out.size) == CU_MAX_SIDE


def test_encode_image_keeps_small_image(tmp_path):
    from PIL import Image

    p = tmp_path / "small.png"
    Image.new("RGB", (640, 480), "white").save(p)
    b64 = _encode_image(str(p))
    out = Image.open(io.BytesIO(base64.b64decode(b64)))
    assert out.size == (640, 480)


def test_encode_image_missing_file_returns_none():
    assert _encode_image("does-not-exist.png") is None


# ── screenshot -> vision loop (the core of computer use) ──────────────────────


async def test_screenshot_attaches_image_to_history(tmp_path, monkeypatch):
    from PIL import Image

    import agent.tool_runner as tr

    shot = tmp_path / "screenshot_x.png"
    Image.new("RGB", (800, 600), "white").save(shot)

    class FakeShot:
        name = "screenshot"

        async def execute(self, args):
            return f"saved: {shot} (screen 800x600, shown 800x600)"

    monkeypatch.setitem(tr.REGISTRY, "screenshot", FakeShot())

    history: list = []

    def add_result(name, result, call_id=""):
        history.append({"role": "tool", "name": name, "content": result})

    tc = {"function": {"name": "screenshot", "arguments": {}}, "id": "call_1"}

    async for _ in tr.execute_tool_call(tc, True, history, None, None, add_result):
        pass

    imgs = [m for m in history if m.get("role") == "user" and m.get("images")]
    assert imgs, "screenshot result should attach an image message to history"
    assert isinstance(imgs[0]["images"][0], str) and imgs[0]["images"][0]


# ── _resolve_preview ──────────────────────────────────────────────────────────


def test_resolve_preview_command():
    from agent.tool_runner import _resolve_preview

    assert _resolve_preview({"command": "ls"}, "fallback") == "ls"


def test_resolve_preview_path():
    from agent.tool_runner import _resolve_preview

    assert "test.txt" in _resolve_preview({"path": "test.txt"}, "fallback")


def test_resolve_preview_fallback():
    from agent.tool_runner import _resolve_preview

    assert _resolve_preview({}, "fallback") == "fallback"


# ── execute_tool_call ─────────────────────────────────────────────────────────


async def test_unknown_tool():
    import agent.tool_runner as tr

    results = []

    def add_result(name, result, call_id=""):
        results.append((name, result))

    tc = {"function": {"name": "unknown_tool", "arguments": {}}, "id": "call_1"}
    async for _ in tr.execute_tool_call(tc, True, [], None, None, add_result):
        pass
    assert any("error" in r for _, r in results)


async def test_blocked_tool(monkeypatch):
    import agent.tool_runner as tr
    from agent.safety import SafetyDecision

    monkeypatch.setattr(
        tr,
        "assess_tool_call",
        lambda name, args, mode: SafetyDecision("blocked", "unsafe", "details"),
    )
    history = []

    def add_result(name, result, call_id=""):
        history.append({"role": "tool", "name": name, "content": result})

    class FakeTool:
        name = "bash"

        async def execute(self, args):
            return "ok"

    monkeypatch.setitem(tr.REGISTRY, "bash", FakeTool())
    tc = {"function": {"name": "bash", "arguments": {"command": "rm -rf /"}}, "id": "call_1"}
    events = []
    async for ev in tr.execute_tool_call(tc, True, history, None, None, add_result):
        events.append(ev)
    assert any("blocked" in str(ev) for ev in events)


async def test_confirm_cancelled(monkeypatch):
    import agent.tool_runner as tr
    from agent.safety import SafetyDecision

    monkeypatch.setattr(
        tr,
        "assess_tool_call",
        lambda name, args, mode: SafetyDecision("confirm", "risky", "details"),
    )

    class FakeCM:
        def register(self):
            return ("cid", None)

        async def wait(self, cid):
            return False

    class FakeTool:
        name = "file_write"

        async def execute(self, args):
            return "ok"

    monkeypatch.setitem(tr.REGISTRY, "file_write", FakeTool())
    history = []

    def add_result(name, result, call_id=""):
        history.append({"role": "tool", "name": name, "content": result})

    tc = {
        "function": {"name": "file_write", "arguments": {"path": "x.txt", "content": "hi"}},
        "id": "call_1",
    }
    events = []
    async for ev in tr.execute_tool_call(tc, True, history, FakeCM(), None, add_result):
        events.append(ev)
    assert any("cancelled" in str(ev) for ev in events)


async def test_tool_timeout(monkeypatch):
    import asyncio

    import agent.tool_runner as tr

    class FakeTool:
        name = "slow"

        async def execute(self, args):
            await asyncio.sleep(100)

    monkeypatch.setitem(tr.REGISTRY, "slow", FakeTool())
    history = []

    def add_result(name, result, call_id=""):
        history.append({"role": "tool", "name": name, "content": result})

    tc = {"function": {"name": "slow", "arguments": {}}, "id": "call_1"}
    events = []
    async for ev in tr.execute_tool_call(tc, True, history, None, None, add_result):
        events.append(ev)
    assert any("timeout" in str(ev) for ev in events)


# ── _truncate ────────────────────────────────────────────────────────────────


def test_truncate_short_text_unchanged():
    from agent.tool_runner import _truncate

    assert _truncate("hello") == "hello"


def test_truncate_empty_unchanged():
    from agent.tool_runner import _truncate

    assert _truncate("") == ""


def test_truncate_by_lines():
    from agent.tool_runner import _TRUNCATE_MAX_LINES, _truncate

    text = "\n".join(str(i) for i in range(_TRUNCATE_MAX_LINES + 100))
    out = _truncate(text)
    assert "truncated" in out
    # marker should report the original count
    assert str(_TRUNCATE_MAX_LINES + 100) in out


def test_truncate_by_bytes():
    from agent.tool_runner import _truncate

    # 1MB of "a" — hits byte cap, not line cap
    text = "a" * (1024 * 1024)
    out = _truncate(text, max_lines=10_000_000, max_bytes=50_000)
    assert "truncated" in out
    assert len(out.encode("utf-8")) < 1024 * 1024  # must be much smaller


def test_truncate_preserves_under_limit():
    from agent.tool_runner import _truncate

    text = "a\nb\nc"
    assert _truncate(text) == text


def test_truncate_under_line_limit_over_byte_limit():
    from agent.tool_runner import _truncate

    # 100 short lines, each with many bytes
    text = "\n".join("x" * 1000 for _ in range(100))
    out = _truncate(text, max_lines=10_000, max_bytes=50_000)
    assert "truncated" in out


def test_tool_result_is_truncated(monkeypatch):
    """End-to-end: a tool returning a huge string gets truncated in history."""
    import agent.tool_runner as tr

    class HugeTool:
        name = "huge"

        async def execute(self, args):
            return "x" * (200 * 1024)

    monkeypatch.setitem(tr.REGISTRY, "huge", HugeTool())
    history = []

    def add_result(name, result, call_id=""):
        history.append({"role": "tool", "name": name, "content": result})

    tc = {"function": {"name": "huge", "arguments": {}}, "id": "call_1"}
    import asyncio

    async def run():
        async for _ in tr.execute_tool_call(tc, True, history, None, None, add_result):
            pass

    asyncio.run(run())
    tool_msgs = [m for m in history if m.get("role") == "tool"]
    assert tool_msgs, "expected tool result in history"
    assert "truncated" in tool_msgs[0]["content"]
    assert len(tool_msgs[0]["content"]) < 200 * 1024
