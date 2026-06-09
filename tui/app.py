from textual.app import App, ComposeResult
from textual.widgets import Input, RichLog, Footer

from agent.ollama_client import OllamaClient


class WardenApp(App):
	CSS_PATH = "theme.css"
	BINDINGS = [
		("escape", "clear_input", "очистить"),
	]

	def __init__(self, model: str, auto_ollama: bool) -> None:
		super().__init__()
		self.model = model
		self.auto_ollama = auto_ollama
		self.ollama = OllamaClient(model=model)

	def compose(self) -> ComposeResult:
		yield RichLog(highlight=False, wrap=True, id="log")
		yield Input(placeholder="promt...", id="input")
		yield Footer()

	async def on_mount(self) -> None:
		log = self.query_one("#log", RichLog)
		log.write("[bold cyan]warden[/bold cyan] — готов к работе")
		if self.auto_ollama:
			log.write("[dim]проверка ollama...[/dim]")
			ok = await self.ollama.ensure_running()
			if ok:
				log.write(f"[dim]ollama подключена, модель: {self.model}[/dim]")
				if not self.ollama.has_model():
					log.write(f"[yellow]модель {self.model} не найдена, скачиваем...[/yellow]")
					await self.ollama.pull_model()
					log.write(f"[dim]модель {self.model} готова[/dim]")
			else:
				log.write("[red]ошибка: не удалось подключить ollama[/red]")
		else:
			log.write("[dim]auto-ollama отключен[/dim]")

	async def on_unmount(self) -> None:
		self.ollama.shutdown()

	def action_clear_input(self) -> None:
		inp = self.query_one("#input", Input)
		inp.value = ""

	async def on_input_submitted(self, event: Input.Submitted) -> None:
		if not event.value.strip():
			return
		log = self.query_one("#log", RichLog)
		log.write(f"[bold]you >[/bold] {event.value}")
		event.input.value = ""

