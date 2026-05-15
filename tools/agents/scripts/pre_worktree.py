#!/usr/bin/env python3
"""Create a feature worktree without relying on shell aliases."""

from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path


def git(args: list[str], *, cwd: Path | None = None) -> str:
    return subprocess.check_output(["git", *args], cwd=cwd, text=True, stderr=subprocess.DEVNULL).strip()


def run(args: list[str], *, cwd: Path | None = None) -> None:
    subprocess.check_call(args, cwd=cwd)


def slugify(value: str) -> str:
    value = value.lower()
    value = re.sub(r"[^a-z0-9]+", "-", value).strip("-")
    return value[:64].strip("-") or "feature"


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("name")
    parser.add_argument("--prefix", default="agent")
    parser.add_argument("--base", default="main")
    parser.add_argument("--no-fetch", action="store_true")
    parser.add_argument("--worktree-root", default=str(Path.home() / "programs/wt"))
    args = parser.parse_args()

    try:
        repo = Path(git(["rev-parse", "--show-toplevel"]))
    except Exception:
        print("error: not a Git checkout; cannot create a feature worktree", file=sys.stderr)
        return 1

    slug = slugify(args.name)
    branch = f"{args.prefix.strip('/')}/{slug}"
    worktree = Path(args.worktree_root).expanduser() / slug
    if worktree.exists():
        print(f"error: worktree path already exists: {worktree}", file=sys.stderr)
        return 1

    existing = set(git(["branch", "--format=%(refname:short)"], cwd=repo).splitlines())
    if branch in existing:
        print(f"error: branch already exists: {branch}", file=sys.stderr)
        return 1

    base = args.base
    if not args.no_fetch:
        try:
            run(["git", "fetch", "origin", args.base], cwd=repo)
            remote_base = f"origin/{args.base}"
            remotes = set(git(["branch", "-r", "--format=%(refname:short)"], cwd=repo).splitlines())
            if remote_base in remotes:
                base = remote_base
        except subprocess.CalledProcessError:
            print(f"warning: could not fetch origin {args.base}; using local {base}", file=sys.stderr)

    run(["git", "worktree", "add", str(worktree), "-b", branch, base], cwd=repo)
    print(f"created worktree: {worktree}")
    print(f"branch: {branch}")
    print(f"base: {base}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
