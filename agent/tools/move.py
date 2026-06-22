from __future__ import annotations

import os
import shutil
from typing import Any

from agent.tools.base import Tool, _in_cwd


class FileMoveTool(Tool):
    """Move or rename a file or directory. Refuses to cross workspace boundaries."""

    name = "file_move"
    description = (
        "Move or rename a file or directory. Both source and destination must be inside the current directory. "
        "Overwrites the destination if it already exists (use with care)."
    )
    params = {
        "src": {"type": "string", "description": "Source path"},
        "dest": {"type": "string", "description": "Destination path"},
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return not (_in_cwd(args.get("src", "")) and _in_cwd(args.get("dest", "")))

    async def execute(self, args: dict[str, Any]) -> str:
        src = args.get("src", "")
        dest = args.get("dest", "")
        if not src or not dest:
            return "error: src and dest are required"
        if not _in_cwd(src) or not _in_cwd(dest):
            return "error: path is outside current directory"
        abs_src = os.path.abspath(src)
        abs_dest = os.path.abspath(dest)
        if not os.path.exists(abs_src):
            return f"error: source not found: {src}"
        # Refuse moves that would land inside themselves (mv a a/b is undefined).
        if os.path.commonpath([abs_src, abs_dest]) == abs_src and abs_src != abs_dest:
            return f"error: cannot move {src} into itself"
        try:
            d = os.path.dirname(abs_dest)
            if d:
                os.makedirs(d, exist_ok=True)
            shutil.move(abs_src, abs_dest)
            return f"moved: {src} → {dest}"
        except Exception as e:
            return f"error: {e}"


class FileCopyTool(Tool):
    """Copy a file. Creates intermediate directories. Refuses paths outside the workspace."""

    name = "file_copy"
    description = (
        "Copy a file. Both source and destination must be inside the current directory. "
        "Creates intermediate directories at the destination if needed."
    )
    params = {
        "src": {"type": "string", "description": "Source path"},
        "dest": {"type": "string", "description": "Destination path"},
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return not (_in_cwd(args.get("src", "")) and _in_cwd(args.get("dest", "")))

    async def execute(self, args: dict[str, Any]) -> str:
        src = args.get("src", "")
        dest = args.get("dest", "")
        if not src or not dest:
            return "error: src and dest are required"
        if not _in_cwd(src) or not _in_cwd(dest):
            return "error: path is outside current directory"
        abs_src = os.path.abspath(src)
        abs_dest = os.path.abspath(dest)
        if not os.path.exists(abs_src):
            return f"error: source not found: {src}"
        if os.path.isdir(abs_src):
            return f"error: source is a directory (only file copy is supported): {src}"
        try:
            d = os.path.dirname(abs_dest)
            if d:
                os.makedirs(d, exist_ok=True)
            shutil.copy2(abs_src, abs_dest)
            return f"copied: {src} → {dest}"
        except Exception as e:
            return f"error: {e}"
