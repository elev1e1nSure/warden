from __future__ import annotations

import os
import tarfile
import zipfile
from typing import Any, Dict

from agent.tools.base import Tool, _in_cwd


_ARCHIVE_EXTS = {".zip", ".tar", ".tgz", ".tbz2", ".tar.gz", ".tar.bz2"}


def _detect_format(path: str) -> str | None:
	low = path.lower()
	if low.endswith(".zip"):
		return "zip"
	if low.endswith(".tar"):
		return "tar"
	if low.endswith(".tgz") or low.endswith(".tar.gz"):
		return "tar.gz"
	if low.endswith(".tbz2") or low.endswith(".tar.bz2"):
		return "tar.bz2"
	return None


def _tar_mode(fmt: str) -> str:
	return {
		"tar": "w:",
		"tar.gz": "w:gz",
		"tar.bz2": "w:bz2",
	}[fmt]


def _open_tar(path: str, fmt: str, mode: str = "r"):
	if mode == "r":
		m = {"tar": "r:", "tar.gz": "r:gz", "tar.bz2": "r:bz2"}[fmt]
	else:
		m = _tar_mode(fmt)
	return tarfile.open(path, m)


class ArchiveTool(Tool):
	"""List, extract, or create zip/tar archives."""
	name = "archive"
	description = (
		"Work with zip and tar archives. "
		"action: 'list' (show contents), 'extract' (unpack to dest), 'create' (pack sources into path). "
		"Format is detected from the file extension. For 'create', the format is taken from the output extension "
		"(.zip, .tar, .tar.gz/.tgz, .tar.bz2/.tbz2). Extraction is blocked outside the workspace; creation requires sources inside it."
	)
	params = {
		"action": {"type": "string", "description": "list | extract | create"},
		"path": {"type": "string", "description": "Archive file path"},
		"dest": {"type": "string", "description": "Destination directory (for extract, default: archive's directory)"},
		"sources": {
			"type": "array",
			"description": "Files/dirs to pack (for create)",
			"items": {"type": "string"},
		},
	}

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		# Both extract and create mutate the filesystem — confirm.
		# But list is read-only: only confirm if user is doing mutation.
		action = str(args.get("action", "")).lower()
		return action in ("extract", "create")

	async def execute(self, args: Dict[str, Any]) -> str:
		action = str(args.get("action", "")).lower()
		path = args.get("path", "")
		if not path:
			return "error: path is required"
		if action not in ("list", "extract", "create"):
			return "error: action must be list, extract, or create"

		fmt = _detect_format(path)
		if fmt is None:
			exts = ", ".join(sorted(_ARCHIVE_EXTS))
			return f"error: cannot detect archive format from extension (supported: {exts})"

		try:
			if action == "list":
				return await self._list(path, fmt)
			if action == "extract":
				return await self._extract(path, fmt, args.get("dest", ""))
			return await self._create(path, fmt, args.get("sources", []))
		except Exception as e:
			return f"error: {e}"

	async def _list(self, path: str, fmt: str) -> str:
		if not os.path.exists(path):
			return f"error: not found: {path}"
		if not os.path.isfile(path):
			return f"error: not a file: {path}"

		if fmt == "zip":
			with zipfile.ZipFile(path) as zf:
				infos = zf.infolist()
			lines = []
			for info in infos[:200]:
				lines.append(f"{info.file_size:>10}  {info.date_time}  {info.filename}")
			if len(infos) > 200:
				lines.append(f"... and {len(infos) - 200} more")
			return "\n".join(lines) if lines else "(empty)"

		with _open_tar(path, fmt, "r") as tf:
			members = tf.getmembers()
		lines = []
		for m in members[:200]:
			lines.append(f"{m.size:>10}  {m.mtime:.0f}  {'d' if m.isdir() else 'f'}  {m.name}")
		if len(members) > 200:
			lines.append(f"... and {len(members) - 200} more")
		return "\n".join(lines) if lines else "(empty)"

	async def _extract(self, path: str, fmt: str, dest: str) -> str:
		# Refuse path traversal in members (zip slip / tar slip).
		# Both extract and create must land inside the workspace.
		dest = dest or os.path.dirname(os.path.abspath(path)) or "."
		if not _in_cwd(dest):
			return "error: dest is outside current directory"

		if fmt == "zip":
			with zipfile.ZipFile(path) as zf:
				# zip slip guard
				for info in zf.infolist():
					target = os.path.realpath(os.path.join(dest, info.filename))
					if not target.startswith(os.path.realpath(dest) + os.sep) and target != os.path.realpath(dest):
						return f"error: zip slip detected in member: {info.filename}"
				zf.extractall(dest)
			return f"extracted: {path} → {dest}"

		with _open_tar(path, fmt, "r") as tf:
			# tar slip guard
			for m in tf.getmembers():
				target = os.path.realpath(os.path.join(dest, m.name))
				if not target.startswith(os.path.realpath(dest) + os.sep) and target != os.path.realpath(dest):
					return f"error: tar slip detected in member: {m.name}"
			tf.extractall(dest)
		return f"extracted: {path} → {dest}"

	async def _create(self, path: str, fmt: str, sources: list) -> str:
		if not sources:
			return "error: sources is required for create"
		# All sources must exist and live inside the workspace.
		missing = [s for s in sources if not os.path.exists(s)]
		if missing:
			return f"error: source(s) not found: {', '.join(missing)}"
		outside = [s for s in sources if not _in_cwd(s)]
		if outside:
			return f"error: source(s) outside current directory: {', '.join(outside)}"
		# Output must be inside the workspace too.
		if not _in_cwd(path):
			return "error: archive path is outside current directory"

		parent = os.path.dirname(os.path.abspath(path))
		if parent:
			os.makedirs(parent, exist_ok=True)

		count = 0
		if fmt == "zip":
			with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as zf:
				for src in sources:
					if os.path.isdir(src):
						for root, _, files in os.walk(src):
							for f in files:
								full = os.path.join(root, f)
								arc = os.path.relpath(full, os.path.dirname(src) or ".").replace("\\", "/")
								zf.write(full, arc)
								count += 1
					else:
						arc = os.path.basename(src)
						zf.write(src, arc)
						count += 1
			return f"created: {path} ({count} entries)"

		with _open_tar(path, fmt, "w") as tf:
			for src in sources:
				tf.add(src, arcname=os.path.basename(src) if not os.path.isdir(src) else None, recursive=True)
				count += 1
		return f"created: {path} ({count} entries)"
