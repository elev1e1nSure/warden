# Tools Summary

| Name | Description |
|---|---|
| `powershell` | Run PowerShell commands (Windows PowerShell, `pwsh` if available) |
| `bash` | Deprecated alias for `powershell` |
| `file_read` | Read file content with line numbers (offset/limit for partial reads) |
| `file_write` | Write text to file (creates directories) |
| `file_delete` | Delete file (only in current directory) |
| `file_list` | List files and directories |
| `glob` | Find files by glob pattern (e.g. `**/*.py`) |
| `grep` | Search file contents by regex (uses ripgrep if available) |
| `edit` | Replace specific string in file (must match exactly once) |
| `apply_patch` | Apply unified-format patch to multiple files |
| `clipboard` | Read/write clipboard text |
| `screenshot` | Take desktop screenshot |
| `mouse` | Move/click/scroll mouse |
| `keyboard` | Type text or press keys |
| `browser_open` | Open URL in browser |
| `browser_read` | Read webpage text via Playwright |
| `browser_screenshot` | Capture webpage screenshot via Playwright |
| `youtube_search` | Search YouTube videos |
| `google_search` | Web search (DuckDuckGo) |
| `webfetch` | Fetch content from URL (HTML, JSON, plain text) |
| `skill` | Load local skill file and sample files |
| `todowrite` | Create and maintain structured task list |
| `question` | Ask user questions during task (handled by chat loop) |