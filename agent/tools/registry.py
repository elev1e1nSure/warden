from __future__ import annotations

from typing import Dict

from agent.tools.base import Tool
from agent.tools.browser import (
    BrowserOpenTool,
    BrowserReadTool,
    BrowserScreenshotTool,
    YouTubeSearchTool,
)
from agent.tools.files import (
    EditTool,
    FileDeleteTool,
    FileListTool,
    FileReadTool,
    FileWriteTool,
    GlobTool,
    GrepTool,
)
from agent.tools.input import ClipboardTool, KeyboardTool, MouseTool, ScreenshotTool
from agent.tools.misc import QuestionTool, SkillTool, TodoWriteTool
from agent.tools.patch import ApplyPatchTool
from agent.tools.search import GoogleSearchTool, WebFetchTool
from agent.tools.shell import BashTool, PowerShellTool

REGISTRY: Dict[str, Tool] = {t.name: t for t in [
	PowerShellTool(),
	BashTool(),
	FileReadTool(),
	GlobTool(),
	GrepTool(),
	EditTool(),
	FileWriteTool(),
	FileDeleteTool(),
	FileListTool(),
	TodoWriteTool(),
	SkillTool(),
	ClipboardTool(),
	ScreenshotTool(),
	MouseTool(),
	KeyboardTool(),
	BrowserOpenTool(),
	BrowserReadTool(),
	YouTubeSearchTool(),
	GoogleSearchTool(),
	BrowserScreenshotTool(),
	ApplyPatchTool(),
	WebFetchTool(),
	QuestionTool(),
]}
