import sys
from datetime import datetime


def _configure_stdio() -> None:
    for stream_name in ("stdout", "stderr"):
        stream = getattr(sys, stream_name)
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure:
            reconfigure(encoding="utf-8", errors="replace")


_configure_stdio()


class Colors:
    """ANSI color codes"""

    RESET = "\033[0m"
    BOLD = "\033[1m"
    DIM = "\033[2m"

    # Colors
    CYAN = "\033[36m"
    YELLOW = "\033[33m"
    RED = "\033[31m"
    GREEN = "\033[32m"
    MAGENTA = "\033[35m"
    WHITE = "\033[37m"
    GRAY = "\033[90m"


def _timestamp() -> str:
    """Returns timestamp in HH:MM:SS format"""
    return datetime.now().strftime("%H:%M:%S")


def _colorize(text: str, color: str, bold: bool = False) -> str:
    """Colorizes text"""
    prefix = color
    if bold:
        prefix += Colors.BOLD
    return f"{prefix}{text}{Colors.RESET}"


def info(msg: str) -> None:
    """Information message"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    tag = _colorize("[INFO]", Colors.CYAN, bold=True)
    print(f"{ts} {tag} {msg}")


def warn(msg: str) -> None:
    """Warning"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    tag = _colorize("[WARN]", Colors.YELLOW, bold=True)
    print(f"{ts} {tag} {msg}", file=sys.stderr)


def error(msg: str) -> None:
    """Error"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    tag = _colorize("[ERROR]", Colors.RED, bold=True)
    print(f"{ts} {tag} {msg}", file=sys.stderr)


def success(msg: str) -> None:
    """Success"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    tag = _colorize("[OK]", Colors.GREEN, bold=True)
    print(f"{ts} {tag} {msg}")


def request(method: str, path: str, status: int | None = None) -> None:
    """HTTP request log"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    method_colored = _colorize(method, Colors.MAGENTA, bold=True)
    path_colored = _colorize(path, Colors.WHITE)

    if status:
        status_color = Colors.GREEN if 200 <= status < 300 else Colors.RED
        status_colored = _colorize(str(status), status_color, bold=True)
        print(f"{ts} {method_colored} {path_colored} → {status_colored}")
    else:
        print(f"{ts} {method_colored} {path_colored}")


def tool(name: str, args: str, result: str | None = None) -> None:
    """Tool execution log"""
    ts = _colorize(f"[{_timestamp()}]", Colors.GRAY)
    name_colored = _colorize(name, Colors.YELLOW, bold=True)
    args_colored = _colorize(args, Colors.DIM)

    if result:
        print(f"{ts} TOOL {name_colored} {args_colored}")
        print(f"{ts}      → {result}")
    else:
        print(f"{ts} TOOL {name_colored} {args_colored}")
