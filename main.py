import argparse
import asyncio
import sys

from tui.app import WardenApp


def main() -> None:
	parser = argparse.ArgumentParser(description="warden — cli агент с ollama")
	parser.add_argument("--model", default="qwen3:8b", help="модель ollama (default: qwen3:8b)")
	parser.add_argument("--no-auto-ollama", action="store_true", help="не запускать ollama serve автоматически")
	args = parser.parse_args()

	app = WardenApp(model=args.model, auto_ollama=not args.no_auto_ollama)
	app.run()


if __name__ == "__main__":
	main()
