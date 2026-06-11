"""Skill discovery and parsing for Warden.

A skill is a Markdown instruction set in `<dir>/SKILL.md` that the agent
can load on demand. Optional YAML frontmatter (name, description) is
parsed; the body is the actual instructions.

Discovery paths (low to high priority — later wins on duplicate name):
- ~/.codex/skills/   (legacy)
- ~/.agents/skills/
- ~/.claude/skills/   (compat)
- ~/.warden/skills/
- <cwd>/skills/      (legacy bare project)
- <cwd>/.claude/skills/   (compat)
- <cwd>/.warden/skills/   (canonical project)
"""

from __future__ import annotations

import logging
import re
from dataclasses import dataclass
from pathlib import Path

log = logging.getLogger(__name__)

MAX_SKILL_BYTES = 64 * 1024  # hard cap; beyond this, the body is truncated

# directory name must match this
_NAME_RE = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")

# frontmatter at top of file: ---\nkey: value\n...\n---\nbody
_FRONTMATTER_RE = re.compile(r"\A---\s*\n(.*?)\n---\s*\n(.*)", re.DOTALL)


@dataclass
class Skill:
	name: str
	description: str
	content: str  # body, frontmatter stripped
	location: str  # absolute path to SKILL.md
	directory: str  # absolute path to skill dir


def _skill_roots() -> list[Path]:
	home = Path.home()
	return [
		home / ".codex" / "skills",
		home / ".agents" / "skills",
		home / ".claude" / "skills",
		home / ".warden" / "skills",
		Path.cwd() / "skills",
		Path.cwd() / ".claude" / "skills",
		Path.cwd() / ".warden" / "skills",
	]


def _parse_frontmatter(text: str) -> tuple[dict[str, str], str]:
	"""Extract simple `key: value` frontmatter. Returns (meta, body)."""
	m = _FRONTMATTER_RE.match(text)
	if not m:
		return {}, text
	meta: dict[str, str] = {}
	for line in m.group(1).splitlines():
		line = line.rstrip()
		if not line or line.startswith("#"):
			continue
		if ":" not in line:
			continue
		key, _, value = line.partition(":")
		key = key.strip().lower()
		value = value.strip().strip('"').strip("'")
		if key and value:
			meta[key] = value
	return meta, m.group(2).lstrip("\n")


def _validate_name(name: str) -> bool:
	return bool(name) and len(name) <= 64 and bool(_NAME_RE.match(name))


def _parse_skill_file(skill_md: Path) -> Skill | None:
	"""Parse a single SKILL.md file. Returns None if invalid."""
	try:
		raw = skill_md.read_text(encoding="utf-8", errors="replace")
	except OSError as e:
		log.warning("skill: cannot read %s: %s", skill_md, e)
		return None

	if len(raw.encode("utf-8")) > MAX_SKILL_BYTES:
		raw = raw.encode("utf-8")[:MAX_SKILL_BYTES].decode("utf-8", errors="ignore")
		log.warning("skill: %s truncated to %d bytes", skill_md, MAX_SKILL_BYTES)

	meta, body = _parse_frontmatter(raw)

	dir_name = skill_md.parent.name
	fm_name = meta.get("name", "").strip()
	# frontmatter name wins if present and valid; otherwise directory name
	name = fm_name if _validate_name(fm_name) else dir_name
	if not _validate_name(name):
		log.warning("skill: %s has invalid name %r — skipped", skill_md, name)
		return None

	if name != dir_name and fm_name:
		log.warning(
			"skill: %s — frontmatter name %r does not match dir %r, using frontmatter",
			skill_md, fm_name, dir_name,
		)

	description = meta.get("description", "").strip()
	if not description:
		log.warning("skill: %s — missing description in frontmatter", skill_md)

	return Skill(
		name=name,
		description=description,
		content=body.rstrip() + "\n",
		location=str(skill_md.resolve()),
		directory=str(skill_md.parent.resolve()),
	)


def discover_skills() -> list[Skill]:
	"""Scan all roots and return skills, deduped by name (highest priority wins)."""
	by_name: dict[str, Skill] = {}
	for root in _skill_roots():
		if not root.is_dir():
			continue
		try:
			children = sorted(root.iterdir(), key=lambda p: p.name.lower())
		except OSError as e:
			log.warning("skill: cannot list %s: %s", root, e)
			continue
		for child in children:
			if not child.is_dir():
				continue
			skill_md = child / "SKILL.md"
			if not skill_md.is_file():
				continue
			parsed = _parse_skill_file(skill_md)
			if parsed is None:
				continue
			# later roots override earlier ones for the same name
			by_name[parsed.name] = parsed
	return sorted(by_name.values(), key=lambda s: s.name)


def find_skill(name: str) -> Skill | None:
	"""Find a single skill by name."""
	for skill in discover_skills():
		if skill.name == name:
			return skill
	return None


def list_siblings(skill: Skill, limit: int = 10) -> list[str]:
	"""List sibling files in the skill directory (up to limit)."""
	d = Path(skill.directory)
	out: list[str] = []
	try:
		entries = sorted(d.iterdir(), key=lambda p: p.name.lower())
	except OSError:
		return out
	for entry in entries:
		if entry.name == "SKILL.md":
			continue
		out.append(f"{entry.name}/" if entry.is_dir() else entry.name)
		if len(out) >= limit:
			break
	return out


def format_catalog(skills: list[Skill] | None = None) -> str:
	"""Render the available skills as an <available_skills> XML block."""
	items = skills if skills is not None else discover_skills()
	if not items:
		return ""
	lines = ["<available_skills>"]
	for s in items:
		lines.append("  <skill>")
		lines.append(f"    <name>{s.name}</name>")
		lines.append(f"    <description>{s.description}</description>")
		lines.append(f"    <location>{s.location}</location>")
		lines.append("  </skill>")
	lines.append("</available_skills>")
	lines.append("Use the skill tool to load a skill when a task matches its description.")
	return "\n".join(lines)


def wrap_skill_content(skill: Skill) -> str:
	"""Wrap a skill's body in the <skill_content> payload format."""
	files = list_siblings(skill)
	files_block = "\n".join(files) if files else "(no extra files)"
	return (
		f'<skill_content name="{skill.name}">\n'
		f"# Skill: {skill.name}\n"
		f"\n"
		f"{skill.content.rstrip()}\n"
		f"\n"
		f"Base directory for this skill: {skill.directory}\n"
		f"<skill_files>\n"
		f"{files_block}\n"
		f"</skill_files>\n"
		f"</skill_content>"
	)
