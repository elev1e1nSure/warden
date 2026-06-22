from __future__ import annotations

import asyncio
import datetime
import os
import subprocess
import time
from pathlib import Path
from typing import Any

from agent.tools.base import Tool
from agent.tools.input import (
    _cleanup_old_screenshots,
    _get_screenshot_dir,
    _map_to_model,
    _map_to_screen,
)


def _is_windows() -> bool:
    return os.name == "nt"


class ImageLocateTool(Tool):
    name = "image_locate"
    description = (
        "Find a template image on the screen and return the center coordinates "
        "in screenshot space (ready to pass to the mouse tool). "
        "Use to click UI elements reliably instead of guessing pixels. "
        "Returns 'not found' if the template is not visible."
    )
    params = {
        "image": {"type": "string", "description": "Path to the template image (PNG/JPG)"},
        "confidence": {
            "type": "number",
            "description": "Match tolerance 0..1 (default 0.9, needs opencv; falls back to exact match)",
        },
    }

    def tool_definition(self) -> dict:
        d = super().tool_definition()
        d["function"]["parameters"]["required"] = ["image"]
        return d

    async def execute(self, args: dict[str, Any]) -> str:
        path = str(args.get("image", "")).strip()
        if not path:
            return "error: image path is required"
        if not Path(path).exists():
            return f"error: image not found: {path}"
        confidence = args.get("confidence")
        if confidence is not None:
            try:
                confidence = float(confidence)
            except (ValueError, TypeError):
                return "error: confidence must be a number between 0 and 1"
            if not (0 <= confidence <= 1):
                return "error: confidence must be between 0 and 1"
        try:
            import pyautogui

            def _locate():
                try:
                    if confidence is not None:
                        return pyautogui.locateOnScreen(path, confidence=confidence)
                    return pyautogui.locateOnScreen(path)
                except TypeError:
                    # older pyautogui without confidence / opencv missing
                    return pyautogui.locateOnScreen(path)

            box = await asyncio.to_thread(_locate)
        except ImportError:
            return "error: pip install pyautogui (opencv-python for confidence)"
        except Exception as e:
            # pyautogui raises ImageNotFoundException (subclass of Exception) when missing
            if "ImageNotFound" in type(e).__name__ or "could not be located" in str(e).lower():
                return "not found"
            return f"error: {e}"

        if box is None:
            return "not found"
        rx, ry = int(box.left + box.width / 2), int(box.top + box.height / 2)
        mx, my = _map_to_model(rx, ry)
        return f"found at ({mx}, {my}) — size {int(box.width)}x{int(box.height)}, screen ({rx}, {ry})"


# ── OCR via Windows.Media.Ocr (no extra pip deps; uses the OS engine) ────────

_OCR_SCRIPT = r"""
$Path = $env:WARDEN_OCR_PATH
Add-Type -AssemblyName System.Runtime.WindowsRuntime | Out-Null
$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
	$_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
	$_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1'
})[0]
function Await($op, $resultType) {
	$asTask = $asTaskGeneric.MakeGenericMethod($resultType)
	$task = $asTask.Invoke($null, @($op))
	$task.Wait(-1) | Out-Null
	$task.Result
}
[Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
[Windows.Storage.StorageFile, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
$engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()
if ($null -eq $engine) { Write-Error 'OCR engine unavailable (no language pack installed)'; exit 1 }
$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Path)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $result.Text
"""


async def _ocr_image(path: str) -> str:
    """Run Windows OCR on an image file, returning recognized text."""
    env = dict(os.environ)
    env["WARDEN_OCR_PATH"] = path
    proc = await asyncio.to_thread(
        subprocess.run,
        ["powershell", "-NoProfile", "-NonInteractive", "-Command", _OCR_SCRIPT],
        capture_output=True,
        text=True,
        timeout=30,
        encoding="utf-8",
        errors="replace",
        env=env,
    )
    if proc.returncode != 0:
        raise RuntimeError((proc.stderr or "OCR failed").strip().splitlines()[0] if proc.stderr else "OCR failed")
    return (proc.stdout or "").strip()


def _capture_region(region: tuple[int, int, int, int] | None) -> str:
    """Grab the screen (or a region) to a temp PNG and return its path."""
    from PIL import ImageGrab

    screenshot_dir = _get_screenshot_dir()
    _cleanup_old_screenshots(screenshot_dir, max_age_seconds=300)
    name = screenshot_dir / f"ocr_{datetime.datetime.now():%Y%m%d_%H%M%S_%f}.png"
    img = ImageGrab.grab()
    if region is not None:
        x, y, w, h = region
        img = img.crop((x, y, x + w, y + h))
    img.save(name)
    return str(name)


class OcrTool(Tool):
    name = "ocr"
    description = (
        "Recognize text on the screen or in an image file using the Windows OCR "
        "engine (no extra dependencies). Windows only. "
        "Give an image path, or omit it to OCR the current screen. "
        "Optional x/y/w/h restricts OCR to a region (screenshot space)."
    )
    params = {
        "image": {
            "type": "string",
            "description": "Path to an image to OCR (optional; defaults to a screen grab)",
        },
        "x": {"type": "integer", "description": "Region left (screenshot space, optional)"},
        "y": {"type": "integer", "description": "Region top (screenshot space, optional)"},
        "w": {"type": "integer", "description": "Region width (optional)"},
        "h": {"type": "integer", "description": "Region height (optional)"},
    }

    def tool_definition(self) -> dict:
        d = super().tool_definition()
        d["function"]["parameters"]["required"] = []
        return d

    async def execute(self, args: dict[str, Any]) -> str:
        if not _is_windows():
            return "error: ocr is Windows-only"
        path = str(args.get("image", "")).strip()
        try:
            if not path:
                region = None
                if all(k in args for k in ("x", "y", "w", "h")):
                    rx, ry = _map_to_screen(int(args["x"]), int(args["y"]))
                    rw, rh = _map_to_screen(int(args["w"]), int(args["h"]))
                    region = (rx, ry, rw, rh)
                path = await asyncio.to_thread(_capture_region, region)
            elif not Path(path).exists():
                return f"error: image not found: {path}"
            text = await _ocr_image(path)
        except ImportError:
            return "error: pip install Pillow"
        except subprocess.TimeoutExpired:
            return "error: timeout running OCR"
        except Exception as e:
            return f"error: {e}"
        return text or "(no text recognized)"


class WaitForTool(Tool):
    name = "wait_for"
    description = (
        "Wait until a condition holds, polling until it's true or the timeout "
        "elapses. Prefer this over fixed sleeps for reliable automation. "
        "type: window (title appears), text (substring visible on screen, needs OCR), "
        "image (template found on screen)."
    )
    params = {
        "type": {"type": "string", "description": "window | text | image"},
        "target": {
            "type": "string",
            "description": "Window title substring, text to find, or template image path",
        },
        "timeout": {"type": "number", "description": "Max seconds to wait (default 10, max 30)"},
        "interval": {"type": "number", "description": "Seconds between checks (default 0.5)"},
    }

    def tool_definition(self) -> dict:
        d = super().tool_definition()
        d["function"]["parameters"]["required"] = ["type", "target"]
        return d

    async def execute(self, args: dict[str, Any]) -> str:
        kind = str(args.get("type", "")).strip().lower()
        target = str(args.get("target", "")).strip()
        if not target:
            return "error: target is required"
        timeout = min(float(args.get("timeout", 10)), 30.0)
        interval = max(float(args.get("interval", 0.5)), 0.1)
        if kind not in ("window", "text", "image"):
            return "error: type must be window, text, or image"

        start = time.monotonic()
        while True:
            try:
                hit = await self._check(kind, target)
            except Exception as e:
                return f"error: {e}"
            elapsed = time.monotonic() - start
            if hit:
                return f"found after {elapsed:.1f}s"
            if elapsed + interval >= timeout:
                return f"timeout: '{target}' not found after {timeout:.0f}s"
            await asyncio.sleep(interval)

    async def _check(self, kind: str, target: str) -> bool:
        if kind == "window":
            from agent.tools.window import _enumerate_windows

            needle = target.lower()
            return any(needle in str(w.get("title", "")).lower() for w in await _enumerate_windows())
        if kind == "image":
            import pyautogui

            def _locate():
                try:
                    return pyautogui.locateOnScreen(target)
                except Exception as e:
                    if "ImageNotFound" in type(e).__name__ or "could not be located" in str(e).lower():
                        return None
                    raise

            return (await asyncio.to_thread(_locate)) is not None
        # text
        path = await asyncio.to_thread(_capture_region, None)
        text = await _ocr_image(path)
        return target.lower() in text.lower()
