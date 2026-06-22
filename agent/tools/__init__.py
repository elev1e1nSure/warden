from __future__ import annotations

import shutil
import subprocess
from pathlib import Path

from agent.tools.archive import ArchiveTool
from agent.tools.base import (
    Tool,
    ToolResult,
    _clean,
    _diff_full,
    _diff_stats,
    _in_cwd,
    parse_args,
)
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
from agent.tools.http import HttpRequestTool
from agent.tools.input import ClipboardTool, KeyboardTool, MouseTool, ScreenshotTool
from agent.tools.lsp import LspTool
from agent.tools.memory import MemoryTool
from agent.tools.misc import _TODO_STORE, QuestionTool, SkillTool, TodoWriteTool
from agent.tools.move import FileCopyTool, FileMoveTool
from agent.tools.patch import ApplyPatchTool
from agent.tools.process import ProcessKillTool, ProcessListTool
from agent.tools.screen import ImageLocateTool, OcrTool, WaitForTool
from agent.tools.search import GoogleSearchTool, WebFetchTool
from agent.tools.shell import BashTool, PowerShellTool
from agent.tools.system import NotifyTool, SystemInfoTool
from agent.tools.window import WindowFocusTool, WindowListTool, WindowManageTool

REGISTRY = {
    t.name: t
    for t in [
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
