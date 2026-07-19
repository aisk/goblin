#!/usr/bin/env bash
set -euo pipefail

# Run the complete Goblin programs that are mirrored in the Book. Keep these
# examples small and self-contained; chapter snippets that intentionally omit
# surrounding context belong in the prose, not in this check.

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
run_example() {
    go run "$repo_dir" run "$1" >/dev/null
}

run_example "$repo_dir/docs/examples/collections-callbacks.goblin"
run_example "$repo_dir/docs/examples/concurrency.goblin"
run_example "$repo_dir/docs/examples/local-module/main.goblin"
