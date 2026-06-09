import subprocess
import time
from typing import Optional

import ollama


class OllamaClient:
	def __init__(self, model: str = "qwen3:8b") -> None:
		self.model = model
		self._process: Optional[subprocess.Popen] = None

	def is_running(self) -> bool:
		try:
			subprocess.run(["ollama", "list"], capture_output=True, timeout=5, check=True)
			return True
		except (subprocess.TimeoutExpired, subprocess.CalledProcessError, FileNotFoundError):
			return False

	def start(self) -> None:
		import sys
		if sys.platform == "win32":
			import subprocess as sp
			self._process = sp.Popen(
				["ollama", "serve"],
				creationflags=sp.CREATE_NEW_PROCESS_GROUP,
				stdout=sp.DEVNULL,
				stderr=sp.DEVNULL,
			)
		else:
			self._process = subprocess.Popen(
				["ollama", "serve"],
				start_new_session=True,
				stdout=subprocess.DEVNULL,
				stderr=subprocess.DEVNULL,
			)

	def wait_for_ready(self, timeout: int = 30) -> bool:
		deadline = time.time() + timeout
		while time.time() < deadline:
			try:
				ollama.list()
				return True
			except Exception:
				time.sleep(0.5)
		return False

	def ensure_running(self) -> bool:
		if self.is_running():
			return True
		self.start()
		return self.wait_for_ready()

	def shutdown(self) -> None:
		if self._process is not None:
			self._process.terminate()
			try:
				self._process.wait(timeout=5)
			except subprocess.TimeoutExpired:
				self._process.kill()
			self._process = None
