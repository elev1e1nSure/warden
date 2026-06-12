from __future__ import annotations

from typing import Dict

from agent.tools.base import Tool
from agent.tools.browser import (
    BrowserClickTool,
    BrowserFillTool,
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
from agent.tools.archive import ArchiveTool
from agent.tools.process import ProcessListTool, ProcessKillTool
from agent.tools.move import FileMoveTool, FileCopyTool
from agent.tools.window import WindowListTool, WindowFocusTool, WindowManageTool
from agent.tools.screen import ImageLocateTool, OcrTool, WaitForTool
from agent.tools.system import SystemInfoTool, NotifyTool
from agent.tools.http import HttpRequestTool
from agent.tools.memory import MemoryTool

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
	FileMoveTool(),
	FileCopyTool(),
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
	BrowserClickTool(),
	BrowserFillTool(),
	ApplyPatchTool(),
	WebFetchTool(),
	HttpRequestTool(),
	QuestionTool(),
	ArchiveTool(),
	ProcessListTool(),
	ProcessKillTool(),
	WindowListTool(),
	WindowFocusTool(),
	WindowManageTool(),
	ImageLocateTool(),
	OcrTool(),
	WaitForTool(),
	SystemInfoTool(),
	NotifyTool(),
	MemoryTool(),
]}
