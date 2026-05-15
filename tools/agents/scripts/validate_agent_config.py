#!/usr/bin/env python3
"""Validate Rune agent config files."""

from __future__ import annotations

import os
import re
import stat
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]

REQUIRED_DOCS = [
    "AGENTS.md",
    ".codex/skills/AGENTS.md",
    ".codex/skills/rune-agent/SKILL.md",
    ".codex/skills/rune-build-engineer/SKILL.md",
    ".codex/skills/rune-cli-engineer/SKILL.md",
    ".codex/skills/rune-tui-engineer/SKILL.md",
    ".codex/skills/rune-store-safety-engineer/SKILL.md",
    "tools/agents/README.md",
    "tools/agents/instructions/index.md",
    "tools/agents/instructions/pre-worktree-pr.instructions.md",
    "tools/agents/instructions/accuracy-pipeline.instructions.md",
    "tools/agents/instructions/agent-config.instructions.md",
    "tools/agents/instructions/cli.instructions.md",
    "tools/agents/instructions/tui.instructions.md",
    "tools/agents/instructions/store-safety.instructions.md",
    "tools/agents/instructions/build-validation.instructions.md",
    "tools/agents/instructions/repo-automation.instructions.md",
    "tools/agents/checklists/implementation-accuracy-review.md",
    "tools/agents/checklists/risk-class-validation-matrix.md",
    "tools/agents/evals/agent-config-evals.md",
    "tools/agents/evals/cli-evals.md",
    "tools/agents/evals/store-safety-evals.md",
    "tools/agents/evals/tui-evals.md",
    "tools/agents/guardrails/repo-policy.md",
    "tools/agents/guardrails/note-store-safety.md",
    "tools/agents/templates/pr-body.md",
]

EXECUTABLE_FILES = [
    "tools/agents/scripts/assert_worktree_ready.py",
    "tools/agents/scripts/pre_worktree.py",
    "tools/agents/scripts/validate_agent_config.py",
    "tools/agents/git-hooks/pre-commit",
    "tools/agents/git-hooks/pre-push",
    "tools/agents/codex-hooks/pre-tool-use-block-default-branch-edit.example.sh",
    "tools/agents/codex-hooks/stop-validation-nudge.example.sh",
]


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def frontmatter(text: str) -> dict[str, str]:
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end == -1:
        return {}
    data: dict[str, str] = {}
    for line in text[4:end].splitlines():
        if ":" not in line or line.startswith("  "):
            continue
        key, value = line.split(":", 1)
        data[key.strip()] = value.strip().strip('"')
    return data


def fail(errors: list[str], message: str) -> None:
    errors.append(message)


def validate_required(errors: list[str]) -> None:
    for rel in REQUIRED_DOCS:
        if not (ROOT / rel).exists():
            fail(errors, f"required file is missing: {rel}")


def validate_root_contract(errors: list[str]) -> None:
    path = ROOT / "AGENTS.md"
    if not path.exists():
        return
    text = read(path)
    lines = text.splitlines()
    if len(lines) > 150:
        fail(errors, f"AGENTS.md is too long: {len(lines)} lines")
    skill_names = {
        skill.parent.name
        for skill in (ROOT / ".codex/skills").glob("*/SKILL.md")
    }
    routed = set(re.findall(r"`(rune-[^`]+)`:", text))
    for name in sorted(routed):
        if name not in skill_names:
            fail(errors, f"AGENTS.md routes missing skill: {name}")


def validate_instruction_index(errors: list[str]) -> None:
    index = ROOT / "tools/agents/instructions/index.md"
    if not index.exists():
        return
    names = set(re.findall(r"`([^`]+\.instructions\.md)`", read(index)))
    for name in sorted(names):
        if not (index.parent / name).exists():
            fail(errors, f"instruction index references missing file: {name}")


def validate_references(errors: list[str]) -> None:
    candidates = [
        ROOT / "AGENTS.md",
        ROOT / ".codex/skills/AGENTS.md",
        *ROOT.glob("tools/agents/**/*.md"),
        *ROOT.glob("tools/agents/**/*.sh"),
        *ROOT.glob(".codex/skills/**/*.md"),
    ]
    for path in candidates:
        if not path.exists():
            continue
        text = read(path)
        for match in re.findall(r"`((?:tools/agents|\.codex/skills|\.github)/[^`]+)`", text):
            if "*" in match or match.endswith("/") or " " in match:
                continue
            if not (ROOT / match).exists():
                fail(errors, f"{path.relative_to(ROOT)} references missing {match}")


def validate_skills(errors: list[str]) -> None:
    skill_root = ROOT / ".codex/skills"
    for skill_file in sorted(skill_root.glob("*/SKILL.md")):
        text = read(skill_file)
        meta = frontmatter(text)
        if not meta.get("name"):
            fail(errors, f"{skill_file.relative_to(ROOT)} missing frontmatter name")
        description = meta.get("description", "")
        if not description:
            fail(errors, f"{skill_file.relative_to(ROOT)} missing description")
        elif len(description.split()) < 8:
            fail(errors, f"{skill_file.relative_to(ROOT)} description is too terse")
        if "\\n\\n" in text:
            fail(errors, f"{skill_file.relative_to(ROOT)} contains literal escaped newlines")


def validate_executable_bits(errors: list[str]) -> None:
    for rel in EXECUTABLE_FILES:
        path = ROOT / rel
        if not path.exists():
            fail(errors, f"executable file is missing: {rel}")
            continue
        mode = path.stat().st_mode
        if not mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH):
            fail(errors, f"file should be executable: {rel}")
        if path.read_bytes().startswith(b"#!") and os.name != "nt":
            first_line = path.read_text(encoding="utf-8").splitlines()[0]
            if "python" not in first_line and "bash" not in first_line:
                fail(errors, f"unexpected shebang for {rel}: {first_line}")


def main() -> int:
    errors: list[str] = []
    validate_required(errors)
    validate_root_contract(errors)
    validate_instruction_index(errors)
    validate_references(errors)
    validate_skills(errors)
    validate_executable_bits(errors)
    if errors:
        for error in errors:
            print(f"error: {error}", file=sys.stderr)
        return 1
    print("agent config validation passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
