"""Tests for agent/logger.py."""

from __future__ import annotations

import agent.logger as logger_module
from agent.logger import Colors


class TestColorize:
    def test_wraps_text_with_color(self):
        result = logger_module._colorize("hello", Colors.RED)
        assert Colors.RED in result
        assert "hello" in result
        assert Colors.RESET in result

    def test_bold_adds_bold_code(self):
        result = logger_module._colorize("x", Colors.GREEN, bold=True)
        assert Colors.BOLD in result


class TestInfo:
    def test_prints_to_stdout(self, capsys):
        logger_module.info("test message")
        out, _ = capsys.readouterr()
        assert "test message" in out
        assert "[INFO]" in out

    def test_not_on_stderr(self, capsys):
        logger_module.info("msg")
        _, err = capsys.readouterr()
        assert err == ""


class TestWarn:
    def test_prints_to_stderr(self, capsys):
        logger_module.warn("warning text")
        _, err = capsys.readouterr()
        assert "warning text" in err
        assert "[WARN]" in err


class TestError:
    def test_prints_to_stderr(self, capsys):
        logger_module.error("error text")
        _, err = capsys.readouterr()
        assert "error text" in err
        assert "[ERROR]" in err


class TestSuccess:
    def test_prints_to_stdout(self, capsys):
        logger_module.success("done!")
        out, _ = capsys.readouterr()
        assert "done!" in out
        assert "[OK]" in out


class TestRequest:
    def test_with_2xx_status(self, capsys):
        logger_module.request("GET", "/health", 200)
        out, _ = capsys.readouterr()
        assert "GET" in out
        assert "/health" in out
        assert "200" in out
        assert "→" in out

    def test_with_5xx_status(self, capsys):
        logger_module.request("POST", "/chat", 500)
        out, _ = capsys.readouterr()
        assert "500" in out

    def test_without_status(self, capsys):
        logger_module.request("POST", "/chat")
        out, _ = capsys.readouterr()
        assert "POST" in out
        assert "/chat" in out
        assert "→" not in out


class TestTool:
    def test_without_result(self, capsys):
        logger_module.tool("file_read", '{"path": "x"}')
        out, _ = capsys.readouterr()
        assert "file_read" in out
        assert "TOOL" in out

    def test_with_result(self, capsys):
        logger_module.tool("file_read", '{"path": "x"}', "content here")
        out, _ = capsys.readouterr()
        assert "file_read" in out
        assert "content here" in out
        assert "→" in out


class TestTimestamp:
    def test_format(self):
        ts = logger_module._timestamp()
        parts = ts.split(":")
        assert len(parts) == 3
        assert all(p.isdigit() for p in parts)
