"""Shared pytest fixtures for the agent test suite."""
from __future__ import annotations

import pytest


@pytest.fixture
def tmp_workspace(tmp_path, monkeypatch):
	"""Change cwd to a temporary path and return it."""
	monkeypatch.chdir(tmp_path)
	return tmp_path


@pytest.fixture
def mock_registry():
	"""Return a fresh copy of the tool registry."""
	from agent.tools import REGISTRY
	return REGISTRY
