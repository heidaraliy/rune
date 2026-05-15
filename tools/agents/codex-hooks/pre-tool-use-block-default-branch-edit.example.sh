#!/usr/bin/env bash
set -euo pipefail

if ! git rev-parse --show-toplevel >/dev/null 2>&1; then
  exit 0
fi

branch="$(git branch --show-current 2>/dev/null || true)"
if [[ "$branch" == "main" || "$branch" == "master" || -z "$branch" ]]; then
  echo "blocked: do not edit tracked files from ${branch:-detached HEAD}" >&2
  exit 1
fi
