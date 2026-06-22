from __future__ import annotations

from typing import Any

from agent.tools.base import Tool


class SkillTool(Tool):
    name = "skill"
    description = "Load a local skill file and a small sample of nearby files."
    params = {"name": {"type": "string", "description": "Skill name"}}

    async def execute(self, args: dict[str, Any]) -> str:
        from agent.skills import find_skill, wrap_skill_content

        name = str(args.get("name", "")).strip()
        if not name:
            return "error: name is required"
        skill = find_skill(name)
        if skill is None:
            return f"error: skill not found: {name}"
        return wrap_skill_content(skill)


# ── todowrite ───────────────────────────────────────────────────────────────

_TODO_STORE: dict[str, list] = {}  # session_id → todos


class TodoWriteTool(Tool):
    name = "todowrite"
    description = (
        "Create and maintain a structured task list. "
        "Tracks progress and organizes multi-step work. "
        "States: pending, in_progress, completed, cancelled. "
        "Priorities: high, medium, low."
    )
    params = {
        "todos": {
            "type": "array",
            "description": "List of task items",
            "items": {
                "type": "object",
                "properties": {
                    "content": {"type": "string", "description": "Brief description of the task"},
                    "status": {
                        "type": "string",
                        "description": "pending | in_progress | completed | cancelled",
                    },
                    "priority": {"type": "string", "description": "high | medium | low"},
                },
                "required": ["content", "status", "priority"],
            },
        },
    }

    def __init__(self):
        super().__init__()
        self._session_id = "default"

    def set_session(self, session_id: str):
        self._session_id = session_id

    async def execute(self, args: dict[str, Any]) -> str:
        items = args.get("todos", [])
        if not items:
            return "error: todos list is empty"
        sid = self._session_id
        _TODO_STORE[sid] = items
        active = sum(1 for t in items if t.get("status") != "completed")
        return f"{active} todos — {len(items)} total:\n" + "\n".join(
            f"  [{t.get('status', '?')}] {t.get('priority', '?')}: {t.get('content', '')}" for t in items
        )


class QuestionTool(Tool):
    name = "question"
    description = (
        "Ask the user questions during a task. "
        "Use when you need clarification, preferences, or decisions. "
        "Supports multiple-choice and free-text questions. "
        "Each question can have options for the user to pick from."
    )
    params = {
        "questions": {
            "type": "array",
            "description": "Questions to ask the user",
            "items": {
                "type": "object",
                "properties": {
                    "question": {"type": "string", "description": "The question text"},
                    "header": {"type": "string", "description": "Short label (max 30 chars)"},
                    "options": {
                        "type": "array",
                        "description": "Available choices (omit for free-text answer)",
                        "items": {
                            "type": "object",
                            "properties": {
                                "label": {"type": "string", "description": "Display text"},
                                "description": {
                                    "type": "string",
                                    "description": "Explanation of choice",
                                },
                            },
                            "required": ["label"],
                        },
                    },
                    "multiple": {
                        "type": "boolean",
                        "description": "Allow selecting multiple choices",
                    },
                },
                "required": ["question", "header"],
            },
        },
    }

    async def execute(self, args: dict[str, Any]) -> str:
        raise RuntimeError("question tool must be handled by chat loop, not executed directly")
