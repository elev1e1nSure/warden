from __future__ import annotations

import asyncio
import os
import pathlib
import re
import shutil
import subprocess
from pathlib import Path
from typing import Any

from agent.tools.base import Tool, ToolResult, _diff_full, _diff_stats, _in_cwd


class FileReadTool(Tool):
    name = "file_read"
    description = (
        "Read a file with line numbers. "
        "offset: first line to read (1-based). limit: max lines to return. "
        "Omit both to read the whole file."
    )
    params = {
        "path": {"type": "string", "description": "File path"},
        "offset": {"type": "integer", "description": "First line (1-based, optional)"},
        "limit": {"type": "integer", "description": "Max lines to return (optional)"},
    }

    def tool_definition(self) -> dict:
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": {
                    "type": "object",
                    "properties": self.params,
                    "required": ["path"],
                },
            },
        }

    async def execute(self, args: dict[str, Any]) -> str:
        path = args.get("path", "")
        offset = int(args.get("offset") or 1)
        limit = args.get("limit")
        if offset < 1:
            offset = 1
        try:
            try:
                if os.path.isdir(path):
                    return f"error: is a directory, not a file: {path}"
                if os.path.getsize(path) > 50 * 1024 * 1024:
                    return f"error: file too large (>50MB) — use offset/limit or grep: {path}"
            except OSError:
                pass
            with open(path, encoding="utf-8", errors="replace") as f:
                raw_lines = f.readlines()
            start = offset - 1
            end = start + int(limit) if limit else len(raw_lines)
            slice_lines = raw_lines[start:end]
            result = []
            for i, line in enumerate(slice_lines, start + 1):
                line = line.rstrip("\n")
                if len(line) > 2000:
                    line = line[:2000] + "…"
                result.append(f"{i}: {line}")
            content = "\n".join(result)
            if len(content) > 8000:
                return content[:8000] + "\n...(truncated)"
            return content
        except Exception as e:
            return f"error: {e}"


class GlobTool(Tool):
    name = "glob"
    description = (
        "Find files by glob pattern. Returns matching paths sorted by modification time. "
        "Use ** for recursive search, e.g. **/*.py"
    )
    params = {
        "pattern": {"type": "string", "description": "Glob pattern, e.g. **/*.py"},
        "path": {"type": "string", "description": "Base directory (default: current)"},
    }

    def tool_definition(self) -> dict:
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": {
                    "type": "object",
                    "properties": self.params,
                    "required": ["pattern"],
                },
            },
        }

    async def execute(self, args: dict[str, Any]) -> str:
        pattern = args.get("pattern", "")
        base = pathlib.Path(args.get("path") or ".").resolve()

        def _mtime(p: pathlib.Path) -> float:
            try:
                return p.stat().st_mtime
            except OSError:
                return 0.0  # broken symlink / vanished file — sort last, don't crash

        try:
            matches = sorted(base.glob(pattern), key=_mtime, reverse=True)
            if not matches:
                return "(no matches)"
            paths = [str(p.relative_to(base)).replace("\\", "/") for p in matches[:200]]
            result = "\n".join(paths)
            if len(matches) > 200:
                result += f"\n... and {len(matches) - 200} more"
            return result
        except Exception as e:
            return f"error: {e}"


class GrepTool(Tool):
    name = "grep"
    description = (
        "Search file contents by regex. Returns file:line: text for each match. "
        "Uses ripgrep if available, falls back to Python."
    )
    params = {
        "pattern": {"type": "string", "description": "Regex pattern"},
        "path": {"type": "string", "description": "Directory or file to search (default: .)"},
        "glob": {"type": "string", "description": "File filter, e.g. *.py (optional)"},
        "case_insensitive": {"type": "boolean", "description": "Ignore case (default false)"},
    }

    def tool_definition(self) -> dict:
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": {
                    "type": "object",
                    "properties": self.params,
                    "required": ["pattern"],
                },
            },
        }

    async def execute(self, args: dict[str, Any]) -> str:
        pattern = args.get("pattern", "")
        path = args.get("path") or "."
        glob_filter = args.get("glob", "")
        nocase = args.get("case_insensitive", False)

        rg = shutil.which("rg")
        if rg:
            cmd = [rg, "--line-number", "--no-heading", "--color=never", "--max-count=100"]
            if nocase:
                cmd.append("-i")
            if glob_filter:
                cmd += ["--glob", glob_filter]
            cmd += [pattern, path]
            try:
                proc = await asyncio.to_thread(
                    subprocess.run,
                    cmd,
                    capture_output=True,
                    text=True,
                    timeout=15,
                    encoding="utf-8",
                    errors="replace",
                )
                out = proc.stdout.strip()
                if not out:
                    return "(no matches)"
                lines = out.split("\n")
                if len(lines) > 100:
                    return "\n".join(lines[:100]) + f"\n... and {len(lines) - 100} more"
                return out
            except Exception:
                pass

        # Python fallback
        try:
            flags = re.IGNORECASE if nocase else 0
            regex = re.compile(pattern, flags)
            base = pathlib.Path(path)
            files = [base] if base.is_file() else list(base.rglob(glob_filter or "*"))
            results = []
            for f in sorted(files):
                if not f.is_file():
                    continue
                try:
                    for i, line in enumerate(f.read_text(encoding="utf-8", errors="ignore").splitlines(), 1):
                        if regex.search(line):
                            rel = str(f.relative_to(base) if not base.is_file() else f).replace("\\", "/")
                            results.append(f"{rel}:{i}: {line.rstrip()}")
                            if len(results) >= 100:
                                break
                except Exception:
                    pass
                if len(results) >= 100:
                    break
            return "\n".join(results) if results else "(no matches)"
        except Exception as e:
            return f"error: {e}"


class EditTool(Tool):
    name = "edit"
    description = (
        "Replace a specific string in a file. "
        "old_string must match exactly once — add surrounding lines to make it unique. "
        "For new files use file_write instead."
    )
    params = {
        "path": {"type": "string", "description": "File path"},
        "old_string": {
            "type": "string",
            "description": "Exact text to replace (must be unique in the file)",
        },
        "new_string": {"type": "string", "description": "Replacement text"},
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return not _in_cwd(args.get("path", ""))

    async def execute(self, args: dict[str, Any]) -> str:
        path = args.get("path", "")
        old = args.get("old_string", "")
        new = args.get("new_string", "")
        if not old:
            return "error: old_string is empty"
        try:
            with open(path, encoding="utf-8") as f:
                content = f.read()
            count = content.count(old)
            if count == 0:
                # Try with normalised line endings
                norm_content = content.replace("\r\n", "\n")
                norm_old = old.replace("\r\n", "\n")
                if norm_content.count(norm_old) == 1:
                    new_content = norm_content.replace(norm_old, new.replace("\r\n", "\n"), 1)
                    with open(path, "w", encoding="utf-8") as f:
                        f.write(new_content)
                    stats = _diff_stats(old, new)
                    label = f"edited {path}  {stats}" if stats else f"edited {path}"
                    return ToolResult(label, _diff_full(old, new, path))
                return f"error: old_string not found in {path}"
            if count > 1:
                return f"error: old_string matches {count} times — make it more specific"
            new_content = content.replace(old, new, 1)
            with open(path, "w", encoding="utf-8") as f:
                f.write(new_content)
            stats = _diff_stats(old, new)
            label = f"edited {path}  {stats}" if stats else f"edited {path}"
            return ToolResult(label, _diff_full(old, new, path))
        except FileNotFoundError:
            return f"error: file not found: {path}"
        except Exception as e:
            return f"error: {e}"


class FileWriteTool(Tool):
    name = "file_write"
    description = "Write text to a file. Creates directories if needed."
    params = {
        "path": {"type": "string", "description": "File path"},
        "content": {"type": "string", "description": "Content"},
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return not _in_cwd(args.get("path", ""))

    async def execute(self, args: dict[str, Any]) -> str:
        path = args.get("path", "")
        content = args.get("content", "")
        try:
            d = str(Path(path).resolve().parent)
            if d:
                os.makedirs(d, exist_ok=True)
            old_content = ""
            try:
                with open(path, encoding="utf-8") as f:
                    old_content = f.read()
            except (FileNotFoundError, OSError):
                pass
            with open(path, "w", encoding="utf-8") as f:
                f.write(content)
            stats = _diff_stats(old_content, content)
            label = f"wrote {path}  {stats}" if stats else f"wrote {path}"
            diff = _diff_full(old_content, content, path) if stats else None
            return ToolResult(label, diff)
        except Exception as e:
            return f"error: {e}"


class FileDeleteTool(Tool):
    name = "file_delete"
    description = "Delete a file or directory. Only works inside the current directory. Use recursive=true for directories."
    params = {
        "path": {"type": "string", "description": "File or directory path"},
        "recursive": {"type": "boolean", "description": "Delete directory recursively (default: false)"},
    }

    def tool_definition(self) -> dict:
        return {
            "type": "function",
            "function": {
                "name": self.name,
                "description": self.description,
                "parameters": {
                    "type": "object",
                    "properties": self.params,
                    "required": ["path"],
                },
            },
        }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return True

    async def execute(self, args: dict[str, Any]) -> str:
        path = args.get("path", "")
        recursive = args.get("recursive", False)
        try:
            abs_path = str(Path(path).resolve())
            if not _in_cwd(path):
                return "error: cannot delete outside current directory"
            if not os.path.exists(abs_path):
                return f"error: not found: {path}"
            if os.path.isdir(abs_path):
                if not recursive:
                    return "error: is a directory — set recursive=true to delete"
                shutil.rmtree(abs_path)
                return f"deleted directory: {path}"
            os.remove(abs_path)
            return f"deleted: {path}"
        except Exception as e:
            return f"error: {e}"


class FileListTool(Tool):
    name = "file_list"
    description = "List files and directories."
    params = {"path": {"type": "string", "description": "Directory (. for current)"}}

    async def execute(self, args: dict[str, Any]) -> str:
        path = args.get("path", ".")
        try:
            from agent.safety._filesystem import is_path_within_workspace

            target = Path(path).resolve()
            workspace = Path.cwd().resolve()
            if not is_path_within_workspace(target, workspace):
                return "error: path is outside allowed scope"
            with os.scandir(path) as entries_iter:
                entries = sorted(entries_iter, key=lambda e: e.name.lower())
            dirs, files = [], []
            for e in entries:
                # Skip symlinks that escape the workspace
                if e.is_symlink():
                    resolved = Path(e.path).resolve()
                    if not is_path_within_workspace(resolved, workspace):
                        continue
                if e.is_dir():
                    dirs.append(f"[{e.name}]")
                else:
                    size = e.stat().st_size
                    kb = size / 1024
                    files.append(f"{e.name} ({kb:.1f}KB)" if kb >= 1 else f"{e.name} ({size}B)")
            parts = []
            if dirs:
                parts.append("dirs: " + "  ".join(dirs))
            if files:
                parts.append("files: " + "  ".join(files))
            return "\n".join(parts) if parts else "(empty)"
        except Exception as e:
            return f"error: {e}"
