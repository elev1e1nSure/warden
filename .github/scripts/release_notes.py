#!/usr/bin/env python3
"""Generate Markdown release notes from conventional commits."""
import os
import re
import subprocess

EMOJI_MAP = {
    "feat":     ("✨", "New Features"),
    "fix":      ("🐛", "Bug Fixes"),
    "perf":     ("⚡", "Performance"),
    "refactor": ("♻️", "Refactoring"),
    "test":     ("🧪", "Tests"),
    "docs":     ("📚", "Documentation"),
    "ci":       ("👷", "CI/CD"),
    "build":    ("📦", "Build"),
    "style":    ("🎨", "Style"),
    "chore":    ("🔧", "Chores"),
    "revert":   ("⏪", "Reverts"),
}


def run(cmd: list[str]) -> str:
    return subprocess.run(cmd, capture_output=True, text=True).stdout.strip()


def get_prev_tag(current: str) -> str:
    tags = [t for t in run(["git", "tag", "--sort=-version:refname"]).splitlines()
            if t and t != current]
    return tags[0] if tags else ""


def get_commits(prev_tag: str) -> list[str]:
    range_ = f"{prev_tag}..HEAD" if prev_tag else "HEAD"
    out = run(["git", "log", range_, "--pretty=format:%s", "--no-merges"])
    return [line for line in out.splitlines() if line.strip()]


def parse(msg: str) -> tuple[str, str, bool, str]:
    m = re.match(r"^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.+)$", msg)
    if m:
        type_, scope, bang, desc = m.groups()
        return type_.lower(), scope or "", bool(bang), desc
    return "", "", False, msg


def main() -> None:
    current = os.environ.get("CURRENT_TAG", "")
    prev = get_prev_tag(current)
    commits = get_commits(prev)

    groups: dict[str, list[str]] = {}
    breaking: list[str] = []
    other: list[str] = []

    for msg in commits:
        type_, scope, is_breaking, desc = parse(msg)
        label = f"**{scope}:** {desc}" if scope else desc
        if is_breaking:
            breaking.append(label)
        if type_ in EMOJI_MAP:
            groups.setdefault(type_, []).append(label)
        else:
            other.append(desc)

    lines: list[str] = []

    if breaking:
        lines += [f"### 💥 Breaking Changes", *[f"- {b}" for b in breaking], ""]

    for type_, (emoji, title) in EMOJI_MAP.items():
        if type_ in groups:
            lines += [f"### {emoji} {title}", *[f"- {m}" for m in groups[type_]], ""]

    if other:
        lines += ["### 📌 Other", *[f"- {m}" for m in other], ""]

    if not lines:
        print("No changes.")
        return

    while lines and not lines[-1]:
        lines.pop()

    print("\n".join(lines))


if __name__ == "__main__":
    main()
