#!/usr/bin/env python3
"""Assert that implementation work is not happening from the default branch."""

from __future__ import annotations

import argparse
import subprocess
import sys

DEFAULT_BRANCHES = {"main", "master"}

AGENT_CONFIG_PREFIXES = (
    ".codex/skills/",
    "tools/agents/",
    ".github/",
    "AGENTS.md",
    "README.md",
    "CONTRIBUTING.md",
    "SECURITY.md",
)


def git(args: list[str]) -> str:
    return subprocess.check_output(["git", *args], text=True, stderr=subprocess.DEVNULL).strip()


def is_git_checkout() -> bool:
    try:
        git(["rev-parse", "--show-toplevel"])
        return True
    except Exception:
        return False


def changed_paths() -> list[str]:
    output = subprocess.check_output(
        ["git", "status", "--porcelain", "--untracked-files=all"],
        text=True,
    )
    paths: list[str] = []
    for line in output.splitlines():
        path = line[3:]
        if " -> " in path:
            path = path.split(" -> ", 1)[1]
        paths.append(path)
    return paths


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--allow-dirty-agent-config", action="store_true")
    parser.add_argument("--allow-no-git", action="store_true")
    args = parser.parse_args()

    if not is_git_checkout():
        if args.allow_no_git:
            print("not a Git checkout; worktree packaging is unavailable")
            return 0
        print("error: not a Git checkout; cannot assert worktree safety", file=sys.stderr)
        return 1

    branch = git(["branch", "--show-current"])
    if branch in {"", *DEFAULT_BRANCHES}:
        print(f"error: implementation branch is not safe: {branch or '<detached>'}", file=sys.stderr)
        return 1

    dirty = changed_paths()
    if dirty and not args.allow_dirty_agent_config:
        print("error: working tree is dirty", file=sys.stderr)
        for path in dirty:
            print(f"  {path}", file=sys.stderr)
        return 1

    if dirty and args.allow_dirty_agent_config:
        for path in dirty:
            if not path.startswith(AGENT_CONFIG_PREFIXES):
                print(f"error: non-agent-config dirty path: {path}", file=sys.stderr)
                return 1

    print(f"worktree ready on {branch}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
