from agent.memory.extractor import MemoryExtractor


class TestTechStack:
    def test_stack_explicit(self) -> None:
        text = "My tech stack is Python, Go and SQLite"
        facts = MemoryExtractor.extract(text)
        cats = {f.category for f in facts}
        assert "tech_stack" in cats
        assert any(f.value.lower().startswith("python") for f in facts if f.category == "tech_stack")

    def test_stack_russian(self) -> None:
        text = "Стек: Python, React, PostgreSQL"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "tech_stack" for f in facts)

    def test_keyword_fallback(self) -> None:
        text = "I am building a backend using fastapi and redis"
        facts = MemoryExtractor.extract(text)
        keys = {f.key for f in facts if f.category == "tech_stack"}
        assert "fastapi" in keys
        assert "redis" in keys


class TestPreference:
    def test_prefer_english(self) -> None:
        text = "I prefer dark mode"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "preference" and "dark" in f.value.lower() for f in facts)

    def test_prefer_russian(self) -> None:
        text = "Предпочитаю русский язык"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "preference" for f in facts)

    def test_style(self) -> None:
        text = "My style: terseness over verbosity"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "preference" and f.key == "style" for f in facts)


class TestProject:
    def test_project_name(self) -> None:
        text = "Project name: warden"
        facts = MemoryExtractor.extract(text)
        proj = [f for f in facts if f.category == "project"]
        assert len(proj) == 1
        assert "warden" in proj[0].value.lower()

    def test_working_on(self) -> None:
        text = "I am working on a new CLI tool"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "project" for f in facts)


class TestUser:
    def test_my_name_is(self) -> None:
        text = "My name is Alice"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "user" and f.key == "name" and "alice" in f.value.lower() for f in facts)

    def test_russian_name(self) -> None:
        text = "Меня зовут Боб"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "user" and "боб" in f.value.lower() for f in facts)

    def test_language(self) -> None:
        text = "Preferred language: Russian"
        facts = MemoryExtractor.extract(text)
        assert any(f.category == "user" and f.key == "preferred_language" for f in facts)


class TestEdgeCases:
    def test_empty_string(self) -> None:
        assert MemoryExtractor.extract("") == []

    def test_no_facts(self) -> None:
        text = "Hello, how are you today?"
        assert MemoryExtractor.extract(text) == []

    def test_confidence_bounds(self) -> None:
        text = "My name is X"
        facts = MemoryExtractor.extract(text)
        for f in facts:
            assert 0.0 < f.confidence <= 1.0

    def test_duplicate_suppression(self) -> None:
        text = "I prefer dark mode. I prefer dark mode."
        facts = MemoryExtractor.extract(text)
        # Should not duplicate same (category, key)
        keys = [(f.category, f.key) for f in facts]
        assert len(keys) == len(set(keys))
