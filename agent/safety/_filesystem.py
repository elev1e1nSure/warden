"""Path safety helpers."""

from __future__ import annotations

import os
import re
import sys
from pathlib import Path


def resolve_workspace() -> Path:
    return Path(os.getcwd()).resolve()


def is_path_within_workspace(path: str | Path, workspace: Path | None = None) -> bool:
    try:
        target = Path(path).resolve()
    except (OSError, ValueError):
        return False
    if workspace is None:
        workspace = resolve_workspace()
    try:
        target.relative_to(workspace)
        return True
    except ValueError:
        return False


def is_dangerous_path(path: str) -> bool:
    """Block UNC paths, device paths, and obvious traversal."""
    p = str(path).strip().lower()
    if p.startswith("\\\\"):
        return True
    if p.startswith("\\\\.\\") or p.startswith("\\\\?\\"):
        return True
    normalized = p.replace("\\", "/")
    if "../" in normalized or "/.." in normalized:
        return True
    # On Windows, a bare "/" prefix is suspicious unless it's a drive-mapped path like /c:/...
    if sys.platform == "win32" and normalized.startswith("/") and not re.match(r"^/[a-z]:", normalized):
        return True
    return False
