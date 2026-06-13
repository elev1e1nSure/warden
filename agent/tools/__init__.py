from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

from agent.tools.base import (
    Tool,
    ToolResult,
    parse_args,
    _clean,
    _in_cwd,
    _diff_stats,
    _diff_full,
)
from agent.tools.shell import PowerShellTool, BashTool, _shell_executable
from agent.tools.files import (
    FileReadTool,
    GlobTool,
    GrepTool,
    EditTool,
    FileWriteTool,
    FileDeleteTool,
    FileListTool,
)
from agent.tools.patch import ApplyPatchTool
from agent.tools.input import (
    ClipboardTool,
    ScreenshotTool,
    MouseTool,
    KeyboardTool,
    _get_screenshot_dir,
    _cleanup_old_screenshots,
)
from agent.tools.browser import (
    BrowserOpenTool,
    BrowserReadTool,
    YouTubeSearchTool,
    BrowserScreenshotTool,
    BrowserClickTool,
    BrowserFillTool,
)
from agent.tools.search import GoogleSearchTool, WebFetchTool
from agent.tools.misc import SkillTool, TodoWriteTool, QuestionTool, _TODO_STORE
from agent.tools.archive import ArchiveTool
from agent.tools.process import ProcessListTool, ProcessKillTool
from agent.tools.move import FileMoveTool, FileCopyTool
from agent.tools.window import WindowListTool, WindowFocusTool, WindowManageTool
from agent.tools.screen import ImageLocateTool, OcrTool, WaitForTool
from agent.tools.system import SystemInfoTool, NotifyTool
from agent.tools.http import HttpRequestTool
from agent.tools.memory import MemoryTool
from agent.tools.lsp import LspTool

REGISTRY = {
	t.name: t for t in [
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
		LspTool(),
	]
}
