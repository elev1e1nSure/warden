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
)
from agent.tools.search import GoogleSearchTool, WebFetchTool
from agent.tools.misc import SkillTool, TodoWriteTool, QuestionTool, _TODO_STORE
from agent.tools.registry import REGISTRY
