"""Tests for agent/ollama_process.py."""
from __future__ import annotations

import subprocess
import sys
from unittest.mock import AsyncMock, MagicMock, patch, call

import pytest


class TestIsRunning:
	def test_returns_true_when_ollama_responds(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.subprocess.run") as mock_run:
			mock_run.return_value = MagicMock(returncode=0)
			m = OllamaProcessManager()
			assert m.is_running() is True

	def test_returns_false_on_file_not_found(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.subprocess.run", side_effect=FileNotFoundError()):
			m = OllamaProcessManager()
			assert m.is_running() is False

	def test_returns_false_on_called_process_error(self):
		from agent.ollama_process import OllamaProcessManager
		err = subprocess.CalledProcessError(1, "ollama")
		with patch("agent.ollama_process.subprocess.run", side_effect=err):
			m = OllamaProcessManager()
			assert m.is_running() is False

	def test_returns_false_on_timeout(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.subprocess.run", side_effect=subprocess.TimeoutExpired("ollama", 5)):
			m = OllamaProcessManager()
			assert m.is_running() is False


class TestStart:
	def test_start_windows(self, monkeypatch):
		from agent.ollama_process import OllamaProcessManager
		monkeypatch.setattr(sys, "platform", "win32")
		with patch("agent.ollama_process.subprocess.Popen") as mock_popen:
			m = OllamaProcessManager()
			m.start()
		mock_popen.assert_called_once()
		kwargs = mock_popen.call_args[1]
		assert "creationflags" in kwargs
		assert m._we_started is True

	def test_start_posix(self, monkeypatch):
		from agent.ollama_process import OllamaProcessManager
		monkeypatch.setattr(sys, "platform", "linux")
		with patch("agent.ollama_process.subprocess.Popen") as mock_popen:
			m = OllamaProcessManager()
			m.start()
		kwargs = mock_popen.call_args[1]
		assert kwargs.get("start_new_session") is True
		assert m._we_started is True


class TestWaitForReady:
	async def test_returns_true_immediately(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.ollama.list", return_value={"models": []}):
			m = OllamaProcessManager()
			result = await m.wait_for_ready(timeout=5)
		assert result is True

	async def test_returns_false_on_timeout(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.ollama.list", side_effect=Exception("not ready")), \
		     patch("asyncio.sleep", new_callable=AsyncMock):
			m = OllamaProcessManager()
			result = await m.wait_for_ready(timeout=0)
		assert result is False


class TestEnsureRunning:
	async def test_already_running_skips_start(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.subprocess.run") as mock_run, \
		     patch("agent.ollama_process.subprocess.Popen") as mock_popen:
			mock_run.return_value = MagicMock(returncode=0)
			m = OllamaProcessManager()
			with patch("agent.ollama_process.ollama.list", return_value={}):
				result = await m.ensure_running()
		mock_popen.assert_not_called()
		assert result is True

	async def test_not_running_calls_start(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.subprocess.run", side_effect=FileNotFoundError()), \
		     patch("agent.ollama_process.subprocess.Popen") as mock_popen, \
		     patch("agent.ollama_process.ollama.list", return_value={}):
			m = OllamaProcessManager()
			result = await m.ensure_running()
		mock_popen.assert_called_once()
		assert result is True


class TestHasModel:
	def test_model_present(self):
		from agent.ollama_process import OllamaProcessManager
		models_resp = {"models": [{"model": "qwen3:8b"}, {"model": "llama3"}]}
		with patch("agent.ollama_process.ollama.list", return_value=models_resp):
			m = OllamaProcessManager(model="qwen3:8b")
			assert m.has_model() is True

	def test_model_absent(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.ollama.list", return_value={"models": [{"model": "other:7b"}]}):
			m = OllamaProcessManager(model="qwen3:8b")
			assert m.has_model() is False

	def test_exception_returns_false(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.ollama.list", side_effect=Exception("connection error")):
			m = OllamaProcessManager()
			assert m.has_model() is False


class TestPullModel:
	async def test_calls_ollama_pull(self):
		from agent.ollama_process import OllamaProcessManager
		with patch("agent.ollama_process.ollama.pull") as mock_pull:
			m = OllamaProcessManager(model="qwen3:8b")
			await m.pull_model()
		mock_pull.assert_called_once_with("qwen3:8b")


class TestShutdown:
	def test_terminates_process_we_started(self):
		from agent.ollama_process import OllamaProcessManager
		mock_proc = MagicMock()
		m = OllamaProcessManager()
		m._process = mock_proc
		m._we_started = True
		m.shutdown()
		mock_proc.terminate.assert_called_once()
		mock_proc.wait.assert_called_once_with(timeout=5)
		assert m._process is None

	def test_no_op_if_we_did_not_start(self):
		from agent.ollama_process import OllamaProcessManager
		mock_proc = MagicMock()
		m = OllamaProcessManager()
		m._process = mock_proc
		m._we_started = False
		m.shutdown()
		mock_proc.terminate.assert_not_called()

	def test_kills_on_timeout(self):
		from agent.ollama_process import OllamaProcessManager
		mock_proc = MagicMock()
		mock_proc.wait.side_effect = subprocess.TimeoutExpired("ollama", 5)
		m = OllamaProcessManager()
		m._process = mock_proc
		m._we_started = True
		m.shutdown()
		mock_proc.kill.assert_called_once()
