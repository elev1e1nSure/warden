import asyncio
from datetime import datetime

from textual.app import App, ComposeResult
from textual.widgets import Input, RichLog, Footer
from rich.text import Text

from agent.ollama_client import OllamaClient
from agent.chat import ChatSession


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
		self.chat = ChatSession(model=model)
		self._streaming = False

	def compose(self) -> ComposeResult:
		yield RichLog(highlight=False, wrap=True, id="log")
		yield Input(placeholder="promt...", id="input")
		yield Footer()

	async def on_mount(self) -> None:
		inp = self.query_one("#input", Input)
		inp.focus()
		log = self.query_one("#log", RichLog)
		log.write("[bold cyan]warden[/bold cyan] — готов к работе")
		if self.auto_ollama:
			log.write("[dim]проверка ollama...[/dim]")
			try:
				ok = await self.ollama.ensure_running()
			except FileNotFoundError:
				log.write("[red]ошибка: ollama не установлена[/red]")
				return
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

	def _ts(self) -> str:
		return datetime.now().strftime("%H:%M")

	async def _stream_response(self, prompt: str) -> None:
		log = self.query_one("#log", RichLog)
		inp = self.query_one("#input", Input)
		self._streaming = True
		inp.disabled = True
		log.write(f"[bold cyan]warden[/bold cyan] [dim]{self._ts()}[/dim]  ", expand=False)
		full = ""
		try:
			for token in self.chat.stream(prompt):
				full += token
				log.write(Text(token), scroll_end=True)
				await asyncio.sleep(0)
			log.write("")
		except Exception as e:
			log.write(f"\n[red]ошибка: {e}[/red]")
		finally:
			self._streaming = False
			inp.disabled = False
			inp.focus()

	async def on_input_submitted(self, event: Input.Submitted) -> None:
		if not event.value.strip() or self._streaming:
			return
		log = self.query_one("#log", RichLog)
		log.write(f"[bold]you[/bold] [dim]{self._ts()}[/dim]  {event.value}\n")
		prompt = event.value
		event.input.value = ""
		asyncio.create_task(self._stream_response(prompt))

