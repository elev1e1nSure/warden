from textual.app import App

from agent.ollama_client import OllamaClient


class WardenApp(App):
	CSS_PATH = "theme.css"

	def __init__(self, model: str, auto_ollama: bool) -> None:
		super().__init__()
		self.model = model
		self.auto_ollama = auto_ollama
		self.ollama = OllamaClient(model=model)

	async def on_mount(self) -> None:
		if self.auto_ollama:
			await self.ollama.ensure_running()
