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
