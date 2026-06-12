from __future__ import annotations

import re
from dataclasses import dataclass
from typing import List


@dataclass
class MemoryFact:
	category: str
	key: str
	value: str
	confidence: float


class MemoryExtractor:
	"""Heuristic fact extraction from chat messages."""

	# Bilingual trigger words mapped to (category, key_template)
	_TRIGGERS = [
		# tech_stack
		(re.compile(r'(?:tech\s*stack|stack)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "tech_stack", "stack"),
		(re.compile(r'(?:using|use|на\s*\w+\s*написано\s*на|использу[юем]|стек)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "tech_stack", "stack"),
		# preference
		(re.compile(r'(?:prefer|like|want|love|предпочитаю|люблю|хочу)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "preference", "preference"),
		(re.compile(r'(?:style|стиль)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "preference", "style"),
		# project
		(re.compile(r'(?:project|проект)["\'\s:]*(?:name|название)?["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "project", "name"),
		(re.compile(r'(?:работаю\s+над|working\s+on)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "project", "name"),
		# user
		(re.compile(r'(?:my\s+name\s+is|меня\s+зовут)["\'\s:]*([^,.;]{1,60})', re.IGNORECASE), "user", "name"),
		(re.compile(r'(?:я\s+|I["\'\s]m\s+)([A-Z][a-z]{1,20})(?:\s|$|[,.])', re.IGNORECASE), "user", "name"),
		(re.compile(r'(?:preferred\s+language|язык)["\'\s:]*([^,.;]{1,30})', re.IGNORECASE), "user", "preferred_language"),
	]

	# Standalone tech keywords that suggest tech_stack when in context
	_TECH_KEYWORDS = {
		"python", "go", "golang", "rust", "javascript", "typescript",
		"java", "kotlin", "swift", "c++", "c#", "ruby", "php",
		"react", "vue", "svelte", "angular", "nextjs", "nuxtjs",
		"django", "flask", "fastapi", "express", "rails",
		"sqlite", "postgres", "mysql", "mongodb", "redis",
		"docker", "kubernetes", "aws", "gcp", "azure",
		"linux", "windows", "macos", "bash", "powershell",
		"ollama", "openai", "openrouter", "anthropic",
		"tailwind", "shadcn", "bootstrap", "css", "html",
	}

	@classmethod
	def extract(cls, text: str) -> List[MemoryFact]:
		if not text or not isinstance(text, str):
			return []

		facts: List[MemoryFact] = []
		seen: set[tuple[str, str]] = set()

		for pattern, category, key_template in cls._TRIGGERS:
			for match in pattern.finditer(text):
				raw = match.group(1).strip("\"' ")
				if not raw:
					continue
				raw = cls._clean_value(raw)
				if not raw:
					continue
				key = cls._derive_key(key_template, raw)
				if (category, key) in seen:
					continue
				seen.add((category, key))
				conf = cls._confidence(match, text)
				facts.append(MemoryFact(category, key, raw, conf))

		# Secondary pass: tech keywords after explicit stack mentions
		facts.extend(cls._extract_tech_keywords(text, seen))
		return facts

	@classmethod
	def _clean_value(cls, value: str) -> str:
		v = value.strip()
		# strip leading filler words / punctuation
		filler = re.compile(
			r"^(?:is|are|was|were|be|being|been|на|в|с|of|for|to|that|this|—|:|\s|\-|\.)+",
			re.IGNORECASE,
		)
		v = filler.sub("", v)
		return v.strip()

	@classmethod
	def _derive_key(cls, template: str, value: str) -> str:
		if template == "stack":
			# For tech stack, derive key from first tech word
			words = re.findall(r"[a-zA-Z+#0-9]+", value)
			for w in words:
				lw = w.lower()
				if lw in cls._TECH_KEYWORDS:
					return lw
			return "stack"
		if template == "name":
			return "name"
		return template

	@classmethod
	def _confidence(cls, match: re.Match, full_text: str) -> float:
		# Base confidence from match quality
		base = 0.7
		# Boost if match is at sentence start (stronger signal)
		start = max(0, match.start() - 1)
		if start == 0 or full_text[start] in ".!?;\n":
			base += 0.15
		# Penalize very long values (likely noise)
		val_len = len(match.group(1).strip())
		if val_len > 40:
			base -= 0.15
		# Boost for short precise values
		if val_len <= 15:
			base += 0.1
		return round(max(0.1, min(1.0, base)), 2)

	@classmethod
	def _extract_tech_keywords(cls, text: str, seen: set[tuple[str, str]]) -> List[MemoryFact]:
		facts: List[MemoryFact] = []
		# Only if text explicitly mentions technology context
		tech_context = bool(re.search(
			r"(?:tech|stack|using|build|backend|frontend|framework|language|библиотек|фреймворк|стек|язык)",
			text,
			re.IGNORECASE,
		))
		if not tech_context:
			return facts

		for kw in cls._TECH_KEYWORDS:
			if ("tech_stack", kw) in seen:
				continue
			pattern = re.compile(rf"\b{re.escape(kw)}\b", re.IGNORECASE)
			if pattern.search(text):
				seen.add(("tech_stack", kw))
				facts.append(MemoryFact("tech_stack", kw, kw, 0.5))
		return facts
