#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
target="${RUNE_INSTALL_BIN:-}"

if [[ -z "$target" ]]; then
  target="$(command -v rune 2>/dev/null || true)"
fi

if [[ -z "$target" ]]; then
  target="$HOME/.local/bin/rune"
  mkdir -p "$(dirname "$target")"
fi

if [[ -d "$target" ]]; then
  echo "rune install target is a directory: $target" >&2
  exit 1
fi

echo "Installing rune to $target"
version="dev"
if git -C "$repo_root" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  version="$(git -C "$repo_root" describe --tags --always --dirty)"
fi

(
  cd "$repo_root"
  go test ./...
  go build -ldflags "-X main.version=$version" -o "$target" ./cmd/rune
)

"$target" --version
