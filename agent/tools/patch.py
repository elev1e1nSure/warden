from __future__ import annotations

import os
import re
from typing import Any, Dict

from agent.tools.base import Tool, ToolResult, _diff_stats, _diff_full


_PATCH_HEADER = re.compile(r'^--- (?:\S+)')
_PATCH_DELIM = re.compile(r'^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)')


class ApplyPatchTool(Tool):
	name = "apply_patch"
	description = (
		"Apply a unified-format patch to multiple files. "
		"Supports add (--- /dev/null), delete (+++ /dev/null), update, and rename. "
		"Each hunk uses @@ -line,count +line,count @@ format. "
		"Preferred over edit/write when changing multiple files at once."
	)
	params = {
		"patch_text": {"type": "string", "description": "Full unified diff patch text describing all changes"},
	}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return True

	async def execute(self, args: Dict[str, Any]) -> str:
		patch = args.get("patch_text", "")
		if not patch:
			return "error: patch_text is required"
		patch = patch.replace("\r\n", "\n").replace("\r", "\n")

		files = self._parse_patch(patch)
		if not files:
			return "error: no valid hunks found in patch"

		added = sum(1 for l in patch.split("\n") if l.startswith("+") and not l.startswith("+++"))
		removed = sum(1 for l in patch.split("\n") if l.startswith("-") and not l.startswith("---"))
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
					if re.match(r'^--- ', lines[i]):
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
					l = lines[i]
					if re.match(r'^--- ', l) or re.match(r'^@@ ', l):
						break
					hunk_lines.append(l)
					i += 1

				hunks.append({
					"old_start": old_start,
					"old_count": old_count,
					"new_start": new_start,
					"new_count": new_count,
					"lines": hunk_lines,
				})

			path = target.lstrip("/")  # strip leading slash
			# Strip git diff a/ b/ prefixes
			path = re.sub(r'^[ab]/', '', path)
			# Handle Windows paths like /c:/foo
			if re.match(r'^[a-zA-Z]:/', path):
				pass  # keep as-is
			elif path.startswith("/"):
				path = path[1:]

			files.append({
				"path": path,
				"old_path": old_path,
				"new_path": new_path,
				"is_add": is_add,
				"is_delete": is_delete,
				"is_rename": is_rename,
				"hunks": hunks,
			})

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
			old_lines = sum(1 for l in hunk["lines"] if not l or l[0] in " -")
			new_lines = sum(1 for l in hunk["lines"] if not l or l[0] in " +")
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
		for l in hunk["lines"]:
			if len(l) == 0:
				old_lines.append("")
				new_lines.append("")
			elif l[0] == " ":
				old_lines.append(l[1:])
				new_lines.append(l[1:])
			elif l[0] == "-":
				old_lines.append(l[1:])
			elif l[0] == "+":
				new_lines.append(l[1:])

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
		result = lines[:match_start] + new_lines + lines[match_start + len(old_lines):]
		return "\n".join(result)
