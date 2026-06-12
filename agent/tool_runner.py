from __future__ import annotations

import asyncio
import base64
import io
from pathlib import Path
from typing import Any, AsyncIterator

from agent.confirmations import ConfirmationManager, QuestionManager
from agent.logger import tool as log_tool
from agent.safety import assess_tool_call
from agent.tools import REGISTRY, ToolResult, parse_args
from agent.tools.input import CU_MAX_SIDE


_SCREENSHOT_TOOLS = {"screenshot", "browser_screenshot"}
_CU_TOOLS = {"screenshot", "mouse", "keyboard"}

_CU_WARNING_TITLE = "Computer use is experimental"
_CU_WARNING_DETAILS = [
    "The agent will control your mouse, keyboard, and screen directly.",
    "It can click, type, and interact with any open window.",
    "Use only in a trusted environment — stop with Esc or move cursor to the top-left corner.",
    "This confirmation appears once per session.",
]


def _extract_saved_path(result: str) -> str | None:
	if not result.startswith("saved: "):
		return None
	path_part = result.removeprefix("saved: ").split(" (")[0].strip()
	p = Path(path_part)
	return str(p) if p.exists() else None


def _encode_image(path: str, max_side: int = CU_MAX_SIDE) -> str | None:
	try:
		from PIL import Image
		img = Image.open(path)
		w, h = img.size
		if max(w, h) > max_side:
			scale = max_side / max(w, h)
			img = img.resize((int(w * scale), int(h * scale)), Image.LANCZOS)
		buf = io.BytesIO()
		img.save(buf, format="PNG")
		return base64.b64encode(buf.getvalue()).decode()
	except Exception:
		return None


def _resolve_preview(args: dict, fallback: str) -> str:
	if "command" in args:
		return str(args["command"])
	if "path" in args:
		try:
			return str(Path(str(args["path"])).resolve())
		except Exception:
			return str(args["path"])
	return fallback


async def execute_tool_call(
	tc,
	auto_mode: bool,
	history: list,
	confirmation_manager: ConfirmationManager | None,
	question_manager: QuestionManager | None,
	add_tool_result_fn,
	cu_warned: dict | None = None,
) -> AsyncIterator[tuple]:
	"""Execute a single tool call. Yields events and records results in history."""
	try:
		name = tc.function.name
		raw_args = tc.function.arguments
		tool_call_id = tc.id
	except AttributeError:
		func = tc.get("function", {})
		name = func.get("name", "")
		raw_args = func.get("arguments", {})
		tool_call_id = tc.get("id", "")

	tool = REGISTRY.get(name)
	if not tool:
		add_tool_result_fn(name, f"error: tool '{name}' not found")
		return

	args = parse_args(raw_args)
	args_str = ", ".join(f"{k}={v}" for k, v in args.items())

	# ── computer use: one-time session warning (bypasses auto mode) ──
	if name in _CU_TOOLS and cu_warned is not None and not cu_warned["value"]:
		if confirmation_manager is None:
			add_tool_result_fn(name, "cancelled: no confirmation manager")
			yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
			return
		call_id, _ = confirmation_manager.register()
		yield ("confirm", {
			"id": call_id,
			"tool": name,
			"risk": "confirm",
			"title": _CU_WARNING_TITLE,
			"summary": "computer use requires physical control of your machine",
			"details": _CU_WARNING_DETAILS,
			"args": args_str,
			"preview": args_str,
			"default": "cancel",
		})
		ok = await confirmation_manager.wait(call_id)
		if not ok:
			add_tool_result_fn(name, "cancelled by user")
			yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
			return
		cu_warned["value"] = True

	# ── question tool: special interactive flow ──
	if name == "question":
		if question_manager is None:
			add_tool_result_fn(name, "error: no question manager")
			yield ("tool", {"name": name, "args": args_str, "result": "error: no question manager"})
			return
		questions = args.get("questions", [])
		if not questions:
			add_tool_result_fn(name, "error: no questions provided")
			yield ("tool", {"name": name, "args": args_str, "result": "error: no questions"})
			return
		call_id, _ = question_manager.register(questions)
		yield ("question", {"id": call_id, "questions": questions})
		answers = await question_manager.wait(call_id)
		if answers is None:
			answers = [[] for _ in questions]
		formatted = ", ".join(
			f'"{q.get("question", "")}"="{", ".join(a) if a else "Unanswered"}"'
			for q, a in zip(questions, answers)
		)
		result_str = f"User answered: {formatted}"
		yield ("tool", {"name": name, "args": args_str, "result": result_str})
		add_tool_result_fn(name, result_str, tool_call_id)
		return

	# ── regular tool execution with safety ──
	mode = "auto" if auto_mode else "ask"
	decision = assess_tool_call(name, args, mode=mode)
	if decision.risk == "blocked":
		add_tool_result_fn(name, f"blocked: {decision.reason}")
		yield ("tool", {"name": name, "args": args_str, "result": f"blocked: {decision.reason}"})
		return

	if decision.risk == "confirm":
		if confirmation_manager is None:
			add_tool_result_fn(name, "cancelled: no confirmation manager")
			yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
			return
		call_id, _ = confirmation_manager.register()
		confirm_payload = {
			"id": call_id,
			"tool": name,
			"risk": decision.risk,
			"title": decision.summary,
			"summary": decision.reason,
			"details": decision.details,
			"args": args_str,
			"preview": _resolve_preview(args, args_str),
			"default": "cancel",
		}
		yield ("confirm", confirm_payload)
		ok = await confirmation_manager.wait(call_id)
		if not ok:
			add_tool_result_fn(name, "cancelled by user")
			yield ("tool", {"name": name, "args": args_str, "result": "cancelled"})
			return

	yield ("tool_start", {"name": name, "args": args_str})
	try:
		result_val = await asyncio.wait_for(tool.execute(args), timeout=60)
	except asyncio.TimeoutError:
		result_val = "error: timeout 60s"
	except RuntimeError as e:
		if "question tool must be handled" in str(e):
			result_val = "error: question tool needs interactive flow"
		else:
			result_val = f"error: {e}"
	except Exception as e:
		result_val = f"error: {e}"
	diff_str = result_val.diff if isinstance(result_val, ToolResult) else None
	result_str = result_val.result if isinstance(result_val, ToolResult) else result_val
	log_tool(name, args_str, result_str[:200] if result_str else None)
	payload: dict = {"name": name, "args": args_str, "result": result_str}
	if diff_str:
		payload["diff"] = diff_str
	yield ("tool", payload)
	add_tool_result_fn(name, result_str, tool_call_id)
	if name in _SCREENSHOT_TOOLS:
		img_path = _extract_saved_path(result_str)
		if img_path:
			img_b64 = _encode_image(img_path)
			if img_b64:
				history.append({
					"role": "user",
					"content": "[screenshot attached]",
					"images": [img_b64],
				})
