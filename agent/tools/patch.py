from __future__ import annotations

import os
import re
from typing import Any

from agent.tools.base import Tool, ToolResult

_PATCH_HEADER = re.compile(r"^--- (?:\S+)")
_PATCH_DELIM = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)")

# Opencode / Anthropic apply_patch format markers
_OC_BEGIN = re.compile(r"^\*\*\* Begin Patch\s*$")
_OC_END = re.compile(r"^\*\*\* End Patch\s*$")
_OC_UPDATE = re.compile(r"^\*\*\* Update File:\s*(.+?)\s*$")
_OC_ADD = re.compile(r"^\*\*\* Add File:\s*(.+?)\s*$")
_OC_DELETE = re.compile(r"^\*\*\* Delete File:\s*(.+?)\s*$")
_OC_MOVE = re.compile(r"^\*\*\* Move to:\s*(.+?)\s*$")
_OC_EOF = re.compile(r"^\*\*\* End of File\s*$")


class ApplyPatchTool(Tool):
    name = "apply_patch"
    description = (
        "Apply a patch to one or more files. "
        "Accepts two formats: (1) opencode/Anthropic `*** Begin Patch ... *** End Patch` "
        "with `*** Update File`, `*** Add File`, `*** Delete File`, `*** Move to`, and "
        "`*** End of File` markers; (2) unified diff with `---` / `+++` / `@@` headers "
        "(add = `--- /dev/null`, delete = `+++ /dev/null`). "
        "Preferred over edit/write when changing multiple files at once."
    )
    params = {
        "patch_text": {"type": "string", "description": "Full patch text describing all changes"},
    }

    def is_dangerous(self, args: dict[str, Any]) -> bool:
        return True

    async def execute(self, args: dict[str, Any]) -> str:
        patch = args.get("patch_text", "")
        if not patch:
            return "error: patch_text is required"
        patch = patch.replace("\r\n", "\n").replace("\r", "\n")

        # Detect format: opencode `*** Begin Patch` wins when present
        if _OC_BEGIN.search(patch):
            return await self._execute_opencode(patch)

        files = self._parse_patch(patch)
        if not files:
            return "error: no valid hunks found in patch"

        added = sum(1 for p in patch.split("\n") if p.startswith("+") and not p.startswith("+++"))
        removed = sum(1 for p in patch.split("\n") if p.startswith("-") and not p.startswith("---"))
        stats = f"+{added} -{removed}" if added or removed else ""

        results = []
        for f in files:
            result = await self._apply_file(f)
            results.append(result)

        out = "\n".join(results)
        label = f"{out}  {stats}" if stats else out
        return ToolResult(label, patch if stats else None)

    def _parse_patch(self, patch: str) -> list:
        files = []
        lines = patch.split("\n")
        i = 0
        while i < len(lines):
            line = lines[i]
            m = re.match(r'^--- (?:"([^"]+)"|(\S+))', line)
            if not m:
                i += 1
                continue
            old_path = m.group(1) or m.group(2) or ""
            i += 1
            if i >= len(lines):
                break
            m2 = re.match(r'^\+\+\+ (?:"([^"]+)"|(\S+))', lines[i])
            if not m2:
                continue
            new_path = m2.group(1) or m2.group(2) or ""
            i += 1

            is_add = old_path == "/dev/null"
            is_delete = new_path == "/dev/null"
            is_rename = not is_add and not is_delete and old_path != new_path

            target = new_path if new_path != "/dev/null" else old_path
            hunks = []

            while i < len(lines):
                h = _PATCH_DELIM.match(lines[i])
                if not h:
                    if re.match(r"^--- ", lines[i]):
                        break
                    i += 1
                    continue
                old_start = int(h.group(1))
                old_count = int(h.group(2) or 1)
                new_start = int(h.group(3))
                new_count = int(h.group(4) or 1)
                i += 1

                hunk_lines = []
                while i < len(lines):
                    ln = lines[i]
                    if re.match(r"^--- ", ln) or re.match(r"^@@ ", ln):
                        break
                    hunk_lines.append(ln)
                    i += 1

                hunks.append(
                    {
                        "old_start": old_start,
                        "old_count": old_count,
                        "new_start": new_start,
                        "new_count": new_count,
                        "lines": hunk_lines,
                    }
                )

            path = target.lstrip("/")  # strip leading slash
            # Strip git diff a/ b/ prefixes
            path = re.sub(r"^[ab]/", "", path)
            # Handle Windows paths like /c:/foo
            if re.match(r"^[a-zA-Z]:/", path):
                pass  # keep as-is
            elif path.startswith("/"):
                path = path[1:]

            files.append(
                {
                    "path": path,
                    "old_path": old_path,
                    "new_path": new_path,
                    "is_add": is_add,
                    "is_delete": is_delete,
                    "is_rename": is_rename,
                    "hunks": hunks,
                }
            )

        return files

    async def _apply_file(self, f: dict) -> str:
        import pathlib

        path = pathlib.Path(f["path"]).resolve()
        abspath = str(path)

        if f["is_delete"]:
            if not path.exists():
                return f"delete: {f['path']} — not found (skipped)"
            if path.is_dir():
                return f"delete: {f['path']} — is a directory (skipped)"
            path.unlink()
            return f"deleted: {f['path']}"

        if f["is_rename"]:
            old_path = pathlib.Path(f["old_path"].lstrip("/")).resolve()
            if not old_path.exists():
                return f"rename: {f['old_path']} → {f['path']} — source not found (skipped)"
            new_content = old_path.read_text(encoding="utf-8")
            old_path.unlink()
        elif f["is_add"]:
            new_content = ""
        else:
            if not path.exists():
                return f"patch: {f['path']} — not found (skipped)"
            new_content = path.read_text(encoding="utf-8")

        delta = 0
        for hunk in f["hunks"]:
            adjusted = dict(hunk)
            adjusted["old_start"] = hunk["old_start"] + delta
            new_content = self._apply_hunk(new_content, adjusted)
            if new_content is None:
                return f"patch: {f['path']} — hunk @@ -{hunk['old_start']},{hunk['old_count']} +{hunk['new_start']},{hunk['new_count']} @@ failed to match"
            # Update delta for subsequent hunks
            old_lines = sum(1 for hl in hunk["lines"] if not hl or hl[0] in " -")
            new_lines = sum(1 for hl in hunk["lines"] if not hl or hl[0] in " +")
            delta += new_lines - old_lines

        d = os.path.dirname(abspath)
        if d:
            os.makedirs(d, exist_ok=True)
        path.write_text(new_content, encoding="utf-8")

        if f["is_add"]:
            return f"added: {f['path']}"
        if f["is_rename"]:
            return f"renamed: {f['old_path']} → {f['path']}"
        return f"patched: {f['path']}"

    def _apply_hunk(self, content: str, hunk: dict) -> str | None:
        lines = content.split("\n")
        old_start = hunk["old_start"] - 1  # 0-indexed
        if old_start < 0:
            old_start = 0

        # Extract old context from hunk
        old_lines = []
        new_lines = []
        for ln in hunk["lines"]:
            if len(ln) == 0:
                old_lines.append("")
                new_lines.append("")
            elif ln[0] == " ":
                old_lines.append(ln[1:])
                new_lines.append(ln[1:])
            elif ln[0] == "-":
                old_lines.append(ln[1:])
            elif ln[0] == "+":
                new_lines.append(ln[1:])

        # Pure addition: no old lines to match — trust old_start hint directly
        if not old_lines:
            insert_at = min(old_start, len(lines))
            result = lines[:insert_at] + new_lines + lines[insert_at:]
            return "\n".join(result)

        # Find matching location starting near old_start, then scan entire file
        search_order = list(range(len(lines) + 1))
        # prioritise positions close to the hinted old_start
        search_order.sort(key=lambda i: abs(i - old_start))
        match_start = None
        for i in search_order:
            if i + len(old_lines) > len(lines):
                continue
            if all(lines[i + j] == ol for j, ol in enumerate(old_lines)):
                match_start = i
                break

        if match_start is None:
            return None

        # Replace
        result = lines[:match_start] + new_lines + lines[match_start + len(old_lines) :]
        return "\n".join(result)

    # ── opencode / Anthropic apply_patch format ─────────────────────────────────
    #
    # Grammar:
    #   *** Begin Patch
    #   *** Update File: <path>
    #    <context line>     (leading space)
    #   -<removed line>
    #   +<added line>
    #   *** End of File
    #   *** Add File: <path>
    #   +<line>...
    #   *** Delete File: <path>
    #   *** Move to: <new path>     (optional, after Update / Add)
    #   *** End Patch

    def _is_opencode_patch(self, patch: str) -> bool:
        return bool(_OC_BEGIN.search(patch)) and bool(_OC_END.search(patch))

    def _parse_opencode(self, patch: str) -> list[dict]:
        lines = patch.split("\n")
        i = 0
        # skip until Begin
        while i < len(lines) and not _OC_BEGIN.match(lines[i]):
            i += 1
        i += 1

        files: list[dict] = []
        current: dict | None = None
        move_to: str | None = None
        at_eof = False
        old_lines: list[str] = []
        new_lines: list[str] = []

        def flush_hunk() -> None:
            nonlocal old_lines, new_lines, move_to, at_eof
            if current is None:
                old_lines, new_lines, move_to = [], [], None
                at_eof = False
                return
            current["hunks"].append(
                {
                    "old_lines": list(old_lines),
                    "new_lines": list(new_lines),
                    "at_eof": at_eof,
                }
            )
            old_lines, new_lines, move_to = [], [], None
            at_eof = False

        def flush_file() -> None:
            nonlocal current
            flush_hunk()
            if current is not None:
                files.append(current)
                current = None

        while i < len(lines):
            line = lines[i]
            if _OC_END.match(line):
                flush_file()
                break
            m_update = _OC_UPDATE.match(line)
            m_add = _OC_ADD.match(line)
            m_delete = _OC_DELETE.match(line)
            m_move = _OC_MOVE.match(line)
            m_eof = _OC_EOF.match(line)
            if m_update or m_add or m_delete:
                # start of a new file op
                flush_file()
                kind = "update" if m_update else ("add" if m_add else "delete")
                path = (m_update or m_add or m_delete).group(1)
                current = {
                    "kind": kind,
                    "path": _normalize_path(path),
                    "raw_path": path,
                    "move_to": None,
                    "hunks": [],
                }
                i += 1
                continue
            if m_move:
                if current is not None:
                    move_to = _normalize_path(m_move.group(1))
                    current["move_to"] = move_to
                i += 1
                continue
            if m_eof:
                at_eof = True
                i += 1
                continue
            if current is None:
                # garbage between blocks — ignore
                i += 1
                continue
            # Body line: must start with ' ', '+', '-'
            if not line:
                # empty line in patch = blank context line in opencode format
                old_lines.append("")
                new_lines.append("")
                i += 1
                continue
            prefix = line[0]
            if prefix == " ":
                old_lines.append(line[1:])
                new_lines.append(line[1:])
            elif prefix == "-":
                old_lines.append(line[1:])
            elif prefix == "+":
                new_lines.append(line[1:])
            else:
                # unknown prefix: treat as no-op, skip
                pass
            i += 1

        flush_file()
        return files

    async def _execute_opencode(self, patch: str) -> str:
        # empty patch sentinel
        normalized = patch.strip()
        if normalized == "*** Begin Patch\n*** End Patch":
            return "error: empty patch"
        files = self._parse_opencode(patch)
        if not files:
            return "error: no valid hunks found in patch"

        results: list[str] = []
        total_added = 0
        total_removed = 0
        for f in files:
            r, a, d = await self._apply_opencode_file(f)
            results.append(r)
            total_added += a
            total_removed += d

        out = "\n".join(results)
        stats = f"+{total_added} -{total_removed}" if total_added or total_removed else ""
        label = f"{out}  {stats}" if stats else out
        return ToolResult(label, patch if stats else None)

    async def _apply_opencode_file(self, f: dict) -> tuple[str, int, int]:
        import pathlib

        path = pathlib.Path(f["path"]).resolve()
        abspath = str(path)

        added = removed = 0

        if f["kind"] == "delete":
            if not path.exists():
                return f"delete: {f['path']} — not found (skipped)", 0, 0
            if path.is_dir():
                return f"delete: {f['path']} — is a directory (skipped)", 0, 0
            path.unlink()
            return f"deleted: {f['path']}", 0, 0

        if f["kind"] == "add":
            content = ""
            for h in f["hunks"]:
                content += "\n".join(h["new_lines"])
                if not h["at_eof"]:
                    content += "\n"
            # always end with a single trailing newline if non-empty
            if content and not content.endswith("\n"):
                content += "\n"
            added = sum(len(h["new_lines"]) for h in f["hunks"])
            d = os.path.dirname(abspath)
            if d:
                os.makedirs(d, exist_ok=True)
            path.write_text(content, encoding="utf-8")
            return f"added: {f['path']}", added, 0

        # update
        if not path.exists():
            return f"patch: {f['path']} — not found (skipped)", 0, 0
        content = path.read_text(encoding="utf-8")
        lines = content.split("\n")

        for h in f["hunks"]:
            removed += len(h["old_lines"])
            added += len(h["new_lines"])
            lines = self._apply_opencode_hunk(lines, h)
            if lines is None:
                return f"patch: {f['path']} — hunk failed to match", 0, 0

        new_content = "\n".join(lines)
        d = os.path.dirname(abspath)
        if d:
            os.makedirs(d, exist_ok=True)
        path.write_text(new_content, encoding="utf-8")

        if f.get("move_to"):
            new_path = pathlib.Path(f["move_to"]).resolve()
            nd = os.path.dirname(str(new_path))
            if nd:
                os.makedirs(nd, exist_ok=True)
            new_path.write_text(new_content, encoding="utf-8")
            path.unlink()
            return f"renamed: {f['path']} → {f['move_to']}", added, removed

        return f"patched: {f['path']}", added, removed

    def _apply_opencode_hunk(self, lines: list[str], hunk: dict) -> list[str] | None:
        old_lines = hunk["old_lines"]
        new_lines = hunk["new_lines"]
        at_eof = hunk["at_eof"]

        # pure addition: trust current position (end of file if at_eof)
        if not old_lines:
            if at_eof:
                return lines + new_lines
            # insertion at end as a safe default
            return lines + new_lines

        # search whole file, prefer proximity to end (more natural for incremental edits)
        match_start: int | None = None
        for i in range(len(lines) - len(old_lines) + 1):
            if all(lines[i + j] == ol for j, ol in enumerate(old_lines)):
                match_start = i
                if at_eof:
                    break
        if match_start is None:
            return None

        # also enforce at_eof: old_lines must be the last len(old_lines) of the file
        if at_eof and match_start + len(old_lines) != len(lines):
            # try to find a match that ends at EOF
            for i in range(len(lines) - len(old_lines), -1, -1):
                if all(lines[i + j] == ol for j, ol in enumerate(old_lines)):
                    match_start = i
                    break
            if match_start + len(old_lines) != len(lines):
                return None

        return lines[:match_start] + new_lines + lines[match_start + len(old_lines) :]


def _normalize_path(p: str) -> str:
    """Normalize a path from opencode format: strip leading slash, drop a/ b/ git prefixes,
    keep Windows drive letters intact."""
    p = p.strip()
    p = p.lstrip("/")
    p = re.sub(r"^[ab]/", "", p)
    if re.match(r"^[a-zA-Z]:[\\/]", p):
        return p
    return p
