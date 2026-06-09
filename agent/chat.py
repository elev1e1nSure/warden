from typing import Iterator, List, Dict, Any
import sys

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
		print(f"[chat] calling ollama with model={self.model}", file=sys.stderr)
		print(f"[chat] messages={self.history}", file=sys.stderr)
		full = ""
		try:
			for chunk in ollama.chat(model=self.model, messages=self.history, stream=True):
				print(f"[chat] chunk={chunk}", file=sys.stderr)
				token = chunk.get("message", {}).get("content", "")
				full += token
				yield token
			self.add_assistant(full)
		except Exception as e:
			print(f"[chat] error: {e}", file=sys.stderr)
			raise
