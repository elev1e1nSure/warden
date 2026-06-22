import asyncio
import subprocess
import time

import ollama


class OllamaProcessManager:
    def __init__(self, model: str | None = None) -> None:
        self.model = model or ""
        self._process: subprocess.Popen | None = None
        self._we_started = False

    def is_running(self) -> bool:
        try:
            subprocess.run(["ollama", "list"], capture_output=True, timeout=5, check=True)
            return True
        except (subprocess.TimeoutExpired, subprocess.CalledProcessError, FileNotFoundError):
            return False

    def start(self) -> None:
        import sys

        if sys.platform == "win32":
            self._process = subprocess.Popen(
                ["ollama", "serve"],
                creationflags=subprocess.CREATE_NEW_PROCESS_GROUP,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        else:
            self._process = subprocess.Popen(
                ["ollama", "serve"],
                start_new_session=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
        self._we_started = True

    async def wait_for_ready(self, timeout: int = 30) -> bool:
        deadline = time.time() + timeout
        while time.time() < deadline:
            try:
                ollama.list()
                return True
            except Exception:
                await asyncio.sleep(0.5)
        return False

    async def ensure_running(self) -> bool:
        if self.is_running():
            return True
        self.start()
        return await self.wait_for_ready()

    def has_model(self) -> bool:
        try:
            models = ollama.list()
            names = [m.get("model", "") for m in models.get("models", [])]
            return self.model in names
        except Exception:
            return False

    def _pull_sync(self) -> None:
        ollama.pull(self.model)

    async def pull_model(self) -> None:
        loop = asyncio.get_event_loop()
        await loop.run_in_executor(None, self._pull_sync)

    def shutdown(self) -> None:
        if self._we_started and self._process is not None:
            self._process.terminate()
            try:
                self._process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._process.kill()
            self._process = None
