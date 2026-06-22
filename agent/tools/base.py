from __future__ import annotations

import json
import os
import re
from abc import ABC, abstractmethod
from typing import Any, Dict


_ANSI = re.compile(r'\x1b\[[0-9;]*[mGKHFJABCDsu]|\x1b\][^\x07]*\x07|\x1b=|\x1b>')


def _diff_stats(old: str, new: str) -> str:
	import difflib
	added = removed = 0
	for line in difflib.unified_diff(old.splitlines(), new.splitlines(), lineterm=""):
		if line.startswith("+") and not line.startswith("+++"):
			added += 1
		elif line.startswith("-") and not line.startswith("---"):
			removed += 1
	if added == 0 and removed == 0:
		return ""
	return f"+{added} -{removed}"


def _diff_full(old: str, new: str, path: str) -> str:
	import difflib
	lines = list(difflib.unified_diff(
		old.splitlines(),
		new.splitlines(),
		fromfile=f"a/{path}",
		tofile=f"b/{path}",
		lineterm="",
	))
	return "\n".join(lines)


class ToolResult:
	"""Wraps a tool result string with an optional unified diff."""
	def __init__(self, result: str, diff: str | None = None):
		self.result = result
		self.diff = diff

	def __str__(self) -> str:
		return self.result

	def __contains__(self, item: str) -> bool:
		return item in self.result

	def lower(self) -> str:
		return self.result.lower()


def _clean(text: str) -> str:
	"""Strip ANSI codes and collapse \r-overwrites"""
	text = _ANSI.sub('', text)
	lines = []
	for line in text.split('\n'):
		parts = line.split('\r')
		cleaned = parts[-1].rstrip()
		if cleaned:
			lines.append(cleaned)
	return '\n'.join(lines)


def _in_cwd(path: str) -> bool:
	try:
		cwd = os.getcwd()
		if not cwd.endswith(os.sep):
			cwd += os.sep
		return os.path.abspath(path).startswith(cwd)
	except Exception:
		return False


class Tool(ABC):
	name: str
	description: str
	params: Dict[str, Any]

	@abstractmethod
	async def execute(self, args: Dict[str, Any]) -> str: ...

	def is_dangerous(self, args: Dict[str, Any]) -> bool:
		return False

	def tool_definition(self) -> dict:
		return {
			"type": "function",
			"function": {
				"name": self.name,
				"description": self.description,
				"parameters": {
					"type": "object",
					"properties": self.params,
					"required": list(self.params.keys()),
				},
			},
		}


def parse_args(arguments: Any) -> dict:
	if isinstance(arguments, dict):
		return arguments
	try:
		result = json.loads(arguments)
		if isinstance(result, dict):
			return result
		return {}
	except Exception:
		return {}
