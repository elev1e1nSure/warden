from typing import Iterator, List, Dict, Any

import ollama


class ChatSession:
	def __init__(self, model: str) -> None:
		self.model = model
		self.history: List[Dict[str, str]] = []

	def add_user(self, text: str) -> None:
		self.history.append({"role": "user", "content": text})

	def add_assistant(self, text: str) -> None:
		self.history.append({"role": "assistant", "content": text})

	def stream(self, text: str) -> Iterator[str]:
		self.add_user(text)
		full = ""
		for chunk in ollama.chat(model=self.model, messages=self.history, stream=True):
			msg = chunk.get("message", {})
			token = msg.get("content") or msg.get("thinking") or ""
			full += token
			yield token
		self.add_assistant(full)
