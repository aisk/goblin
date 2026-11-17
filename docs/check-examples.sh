#!/usr/bin/env bash
set -euo pipefail

# Run the complete Goblin programs that are mirrored in the Book. Keep these
# examples small and self-contained; chapter snippets that intentionally omit
# surrounding context belong in the prose, not in this check.

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
run_example() {
    go run "$repo_dir" run "$1" >/dev/null
}
# run_with_input feeds stdin and passes arguments to an example that checks
# its own results.
run_with_input() {
    local input=$1 file=$2
    shift 2
    printf '%b' "$input" | go run "$repo_dir" run "$file" "$@" >/dev/null
}

run_with_input '' "$repo_dir/docs/examples/cli-tasks.goblin" in.txt out.txt --verbose
run_example "$repo_dir/docs/examples/collections-callbacks.goblin"
run_example "$repo_dir/docs/examples/concurrency.goblin"
run_example "$repo_dir/docs/examples/concurrency-results.goblin"
run_example "$repo_dir/docs/examples/goblin-handle.goblin"
run_with_input 'Ada\n1\r\n2\n3' "$repo_dir/docs/examples/input.goblin"
run_example "$repo_dir/docs/examples/local-module/main.goblin"
run_example "$repo_dir/docs/examples/math-tasks.goblin"
run_with_input 'y\nINFO up\nERROR disk full\n' "$repo_dir/docs/examples/os-tasks.goblin" in.txt out.txt
run_example "$repo_dir/docs/examples/rand-tasks.goblin"
run_example "$repo_dir/docs/examples/reference-smoke.goblin"
run_with_input "ok\nerror: disk full\nfine\nerror again\n" "$repo_dir/docs/examples/regex-tasks.goblin" err
run_example "$repo_dir/docs/examples/time-tasks.goblin"
run_example "$repo_dir/docs/examples/url-tasks.goblin"
run_example "$repo_dir/docs/examples/protocols.goblin"
