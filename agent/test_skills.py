from __future__ import annotations

from pathlib import Path

import agent.skills as sk


def _setup_root(base: Path) -> None:
	# project .warden/skills
	d1 = base / ".warden" / "skills" / "alpha"
	d1.mkdir(parents=True, exist_ok=True)
	(d1 / "SKILL.md").write_text(
		"---\nname: alpha\ndescription: First skill\n---\n\n# Alpha\n\nbody\n",
		encoding="utf-8",
	)

	# project .claude/skills
	d2 = base / ".claude" / "skills" / "beta"
	d2.mkdir(parents=True, exist_ok=True)
	(d2 / "SKILL.md").write_text(
		"---\nname: beta\ndescription: Claude compat\n---\n\nbody\n",
		encoding="utf-8",
	)

	# global .warden/skills
	d3 = base / "global_warden" / ".warden" / "skills" / "gamma"
	d3.mkdir(parents=True, exist_ok=True)
	(d3 / "SKILL.md").write_text(
		"---\nname: gamma\ndescription: Global skill\n---\n\nbody\n",
		encoding="utf-8",
	)

	# project overrides global for same name
	d4 = base / ".warden" / "skills" / "gamma"
	d4.mkdir(parents=True, exist_ok=True)
	(d4 / "SKILL.md").write_text(
		"---\nname: gamma\ndescription: Project override\n---\n\nbody\n",
		encoding="utf-8",
	)

	# invalid name
	d5 = base / ".warden" / "skills" / "BadName"
	d5.mkdir(parents=True, exist_ok=True)
	(d5 / "SKILL.md").write_text(
		"---\nname: BadName\ndescription: bad\n---\n\nbody\n",
		encoding="utf-8",
	)

	# missing description
	d6 = base / ".warden" / "skills" / "nodesc"
	d6.mkdir(parents=True, exist_ok=True)
	(d6 / "SKILL.md").write_text(
		"---\nname: nodesc\n---\n\nbody\n",
		encoding="utf-8",
	)

	# empty dir
	d7 = base / ".warden" / "skills" / "nodoc"
	d7.mkdir(parents=True, exist_ok=True)


def _patch_roots(monkeypatch, base: Path) -> None:
	monkeypatch.setattr(sk.Path, "cwd", classmethod(lambda cls: base))
	monkeypatch.setattr(
		sk.Path, "home", classmethod(lambda cls: base / "global_warden")
	)


def test_discover_finds_all_paths(monkeypatch, tmp_path) -> None:
	_setup_root(tmp_path)
	_patch_roots(monkeypatch, tmp_path)

	skills = sk.discover_skills()
	names = {s.name for s in skills}

	assert "alpha" in names
	assert "beta" in names
	assert "gamma" in names
	# "BadName" invalid name → skipped
	assert "BadName" not in names
	# "nodesc" still discovered, just no description
	assert "nodesc" in names
	# "nodoc" has no SKILL.md → skipped
	assert "nodoc" not in names


def test_discover_project_overrides_global(monkeypatch, tmp_path) -> None:
	_setup_root(tmp_path)
	_patch_roots(monkeypatch, tmp_path)

	gamma = next(s for s in sk.discover_skills() if s.name == "gamma")
	assert gamma.description == "Project override"


def test_parse_frontmatter_strips() -> None:
	text = "---\nname: x\ndescription: y\n---\n\nbody text\n"
	meta, body = sk._parse_frontmatter(text)
	assert meta == {"name": "x", "description": "y"}
	assert body == "body text\n"


def test_parse_frontmatter_none() -> None:
	text = "no frontmatter here"
	meta, body = sk._parse_frontmatter(text)
	assert meta == {}
	assert body == text


def test_validate_name() -> None:
	assert sk._validate_name("foo") is True
	assert sk._validate_name("foo-bar") is True
	assert sk._validate_name("foo-123") is True
	assert sk._validate_name("Foo") is False
	assert sk._validate_name("-foo") is False
	assert sk._validate_name("foo-") is False
	assert sk._validate_name("a" * 65) is False
	assert sk._validate_name("") is False


def test_format_catalog(monkeypatch, tmp_path) -> None:
	_setup_root(tmp_path)
	_patch_roots(monkeypatch, tmp_path)

	cat = sk.format_catalog()
	assert "<available_skills>" in cat
	assert "</available_skills>" in cat
	assert "name>alpha<" in cat
	assert "name>beta<" in cat
	assert "skill tool" in cat.lower()


def test_format_catalog_empty() -> None:
	# no skills in empty dir
	cat = sk.format_catalog([])
	assert cat == ""


def test_wrap_skill_content(monkeypatch, tmp_path) -> None:
	_setup_root(tmp_path)
	_patch_roots(monkeypatch, tmp_path)

	alpha = sk.find_skill("alpha")
	assert alpha is not None
	wrapped = sk.wrap_skill_content(alpha)
	assert '<skill_content name="alpha">' in wrapped
	assert "# Skill: alpha" in wrapped
	assert "body" in wrapped
	assert "Base directory" in wrapped
	assert "<skill_files>" in wrapped


def test_max_size_truncates(monkeypatch, tmp_path) -> None:
	big = "x" * (sk.MAX_SKILL_BYTES + 1000)
	d = tmp_path / ".warden" / "skills" / "huge"
	d.mkdir(parents=True, exist_ok=True)
	(d / "SKILL.md").write_text(
		f"---\nname: huge\ndescription: large\n---\n\n{big}\n", encoding="utf-8"
	)
	monkeypatch.setattr(sk.Path, "cwd", classmethod(lambda cls: tmp_path))
	monkeypatch.setattr(sk.Path, "home", classmethod(lambda cls: tmp_path))

	huge = sk.find_skill("huge")
	assert huge is not None
	assert len(huge.content.encode("utf-8")) <= sk.MAX_SKILL_BYTES
