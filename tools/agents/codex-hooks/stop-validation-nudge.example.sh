#!/usr/bin/env bash
set -euo pipefail

if git rev-parse --show-toplevel >/dev/null 2>&1; then
  if git diff --quiet --exit-code; then
    exit 0
  fi
fi

echo "Before final response, report validation and note whether Git packaging was available." >&2
