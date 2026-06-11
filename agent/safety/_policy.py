"""Tool-level safety assessment."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, Any, List

from agent.safety._filesystem import is_dangerous_path, is_path_within_workspace
from agent.safety._powershell import classify as classify_powershell


@dataclass
class SafetyDecision:
    risk: str  # "safe" | "confirm" | "blocked"
    reason: str
    summary: str
    details: List[str] = field(default_factory=list)
    normalized_args: Dict[str, Any] = field(default_factory=dict)


def _decide(risk, reason, summary, details=None, args=None, tool=None, mode="ask") -> SafetyDecision:
    d = SafetyDecision(risk=risk, reason=reason, summary=summary,
                       details=details or [], normalized_args=args or {})
    return _apply_mode(d, tool or "", mode)


def _apply_mode(decision: SafetyDecision, tool_name: str, mode: str) -> SafetyDecision:
    if mode == "auto" and decision.risk == "confirm" and tool_name not in ("file_delete", "delete"):
        return SafetyDecision(
            risk="safe",
            reason=decision.reason,
            summary=decision.summary,
            details=decision.details,
            normalized_args=decision.normalized_args,
        )
    return decision


def assess_tool_call(tool_name: str, args: dict, cwd: str | None = None, mode: str = "ask") -> SafetyDecision:
    if cwd is None:
        cwd = os.getcwd()
    workspace = Path(cwd).resolve()
    norm = dict(args)

    def _d(risk, reason, summary, details=None):
        return _decide(risk, reason, summary, details, norm, tool_name, mode)

    # file_write
    if tool_name in ("file_write", "write"):
        path = str(norm.get("path", ""))
        if is_dangerous_path(path):
            return _d("blocked", "dangerous path", "File path is outside allowed scope",
                      ["UNC path, device path, or traversal detected"])
        if not is_path_within_workspace(path, workspace):
            return _d("confirm", "writes outside workspace", "Writing file outside workspace", [f"path: {path}"])
        return _d("confirm", "modifies files", "Writing file inside workspace", [f"path: {path}"])

    # file_delete
    if tool_name in ("file_delete", "delete"):
        path = str(norm.get("path", ""))
        if is_dangerous_path(path):
            return _d("blocked", "dangerous path", "File path is outside allowed scope",
                      ["UNC path, device path, or traversal detected"])
        if not is_path_within_workspace(path, workspace):
            return _d("blocked", "deletes outside workspace", "Deleting file outside workspace is blocked",
                      [f"path: {path}"])
        return _d("confirm", "destructive file operation", "Deleting file inside workspace", [f"path: {path}"])

    # file_read
    if tool_name in ("file_read", "read"):
        path = str(norm.get("path", ""))
        if is_dangerous_path(path):
            return _d("blocked", "dangerous path", "File path is outside allowed scope",
                      ["UNC path, device path, or traversal detected"])
        if not is_path_within_workspace(path, workspace):
            return _d("confirm", "reads outside workspace", "Reading file outside workspace", [f"path: {path}"])
        return _d("safe", "read-only", "Reading file", [f"path: {path}"])

    # file_list
    if tool_name in ("file_list", "list"):
        path = str(norm.get("path", "."))
        if is_dangerous_path(path):
            return _d("blocked", "dangerous path", "Path is outside allowed scope",
                      ["UNC path, device path, or traversal detected"])
        if not is_path_within_workspace(path, workspace):
            return _d("confirm", "lists outside workspace", "Listing directory outside workspace", [f"path: {path}"])
        return _d("safe", "read-only", "Listing directory", [f"path: {path}"])

    # todowrite / skill
    if tool_name == "todowrite":
        return _d("safe", "updates session todo state", "Updating todo list")
    if tool_name == "skill":
        return _d("safe", "reads local skill files", "Loading skill", [f"name: {norm.get('name', '')}"])

    # bash / powershell
    if tool_name in ("bash", "powershell"):
        command = str(norm.get("command", ""))
        risk, reason, details = classify_powershell(command)
        summary = "Read-only shell command" if risk == "safe" else reason.capitalize()
        return _d(risk, reason, summary, details)

    # clipboard
    if tool_name == "clipboard":
        if str(norm.get("action", "read")).lower() == "read":
            return _d("safe", "read-only", "Reading clipboard")
        return _d("confirm", "modifies clipboard", "Writing to clipboard")

    # screenshot
    if tool_name == "screenshot":
        return _d("safe", "read-only", "Taking screenshot")

    # mouse
    if tool_name == "mouse":
        action = str(norm.get("action", "click")).lower()
        if action == "move":
            return _d("safe", "read-only pointer", "Moving cursor")
        return _d("confirm", "simulates input", f"Mouse {action}", ["can interact with UI elements"])

    # keyboard
    if tool_name == "keyboard":
        action = str(norm.get("action", "type")).lower()
        text = str(norm.get("text", "")).lower()
        if action == "press":
            dangerous = {"delete", "backspace", "alt+f4", "ctrl+w", "ctrl+shift+w"}
            if any(dk in text for dk in dangerous):
                return _d("confirm", "destructive key combination", f"Pressing {text}",
                          ["can close windows or delete content"])
        return _d("confirm", "simulates input", f"Keyboard {action}", ["types or presses keys"])

    # browser_open
    if tool_name == "browser_open":
        url = str(norm.get("url", "")).lower()
        if "localhost" in url or "127.0.0.1" in url:
            return _d("safe", "local URL", "Opening localhost URL", [f"url: {url}"])
        return _d("confirm", "opens external URL", "Opening external URL", [f"url: {url}"])

    # read-only browser / search tools
    if tool_name in ("browser_read", "browser_screenshot", "youtube_search", "google_search"):
        return _d("safe", "read-only", f"Using {tool_name}")

    # apply_patch
    if tool_name == "apply_patch":
        return _d("confirm", "modifies files via patch", "Applying patch to files",
                  ["can create, modify, delete, or rename files"])

    # webfetch
    if tool_name == "webfetch":
        url = str(norm.get("url", "")).lower()
        if "localhost" in url or "127.0.0.1" in url or "::1" in url:
            return _d("safe", "read-only local", "Fetching local URL", [f"url: {url}"])
        return _d("safe", "read-only", f"Fetching {url}")

    # question
    if tool_name == "question":
        return _d("safe", "interactive", "Asking user")

    return _d("confirm", "unknown tool", f"Unknown tool: {tool_name}",
              ["no safety policy defined — requires confirmation"])
