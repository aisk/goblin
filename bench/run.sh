#!/usr/bin/env bash
# Run every benchmark in every language and print a Markdown timing table.
#
# Usage: ./run.sh [repeats]        (default: 5)
#
# Runtimes are picked up from $PATH; override with GOBLIN / PYTHON / LUA / NODE.
# Each implementation is run `repeats` times and the *minimum* wall-clock time
# is reported, which is the most noise-resistant statistic for CPU-bound work.

set -euo pipefail

cd "$(dirname "$0")"

REPEATS="${1:-5}"
BENCHES=(fib sieve mandelbrot nqueens matmul hanoi)

GOBLIN="${GOBLIN:-goblin}"
PYTHON="${PYTHON:-python3}"
LUA="${LUA:-lua}"
NODE="${NODE:-node}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

have() { command -v "$1" >/dev/null 2>&1; }

# Wall-clock milliseconds of the best out of $REPEATS runs. Output goes to a
# file so stdout printing is part of what we measure, just like a real run.
best_ms() {
    local best=""
    for _ in $(seq "$REPEATS"); do
        local start end ms
        start=$(date +%s%N)
        "$@" >"$WORK/stdout" 2>"$WORK/stderr" || { echo "FAIL"; return; }
        end=$(date +%s%N)
        ms=$(((end - start) / 1000000))
        if [ -z "$best" ] || [ "$ms" -lt "$best" ]; then best=$ms; fi
    done
    echo "$best"
}

echo "## Environment"
echo
echo '```'
uname -srm
grep -m1 'model name' /proc/cpuinfo 2>/dev/null | sed 's/^[[:space:]]*//' || true
# `have x && cmd` would abort the whole script under `set -e` when x is
# missing, so every probe is a full if-block.
if have "$GOBLIN"; then echo "goblin  repo $(git rev-parse --short HEAD 2>/dev/null || echo unknown)"; fi
if have "$PYTHON"; then "$PYTHON" --version; fi
if have "$LUA"; then "$LUA" -v; fi
if have "$NODE"; then echo "node    $("$NODE" --version)"; fi
if have go; then go version; fi
echo '```'
echo

# Build the two compiled targets up front so compilation is not timed as part
# of the run. Goblin transpile+compile time is reported separately below.
declare -A GOBLINBUILD
for b in "${BENCHES[@]}"; do
    if have go; then
        go build -o "$WORK/$b.go.exe" "./$b/$b.go"
    fi
    if have "$GOBLIN"; then
        start=$(date +%s%N)
        "$GOBLIN" build-exe -o "$WORK/$b.goblin.exe" "./$b/$b.goblin" >/dev/null 2>&1
        end=$(date +%s%N)
        GOBLINBUILD[$b]=$(((end - start) / 1000000))
    fi
done

echo "## Results (best of $REPEATS, milliseconds)"
echo
echo "| benchmark | go | node | lua | python | goblin build-exe | goblin run |"
echo "|---|---|---|---|---|---|---|"
for b in "${BENCHES[@]}"; do
    row="| $b "
    if have go; then row+="| $(best_ms "$WORK/$b.go.exe") "; else row+="| - "; fi
    if have "$NODE"; then row+="| $(best_ms "$NODE" "./$b/$b.js") "; else row+="| - "; fi
    if have "$LUA"; then row+="| $(best_ms "$LUA" "./$b/$b.lua") "; else row+="| - "; fi
    if have "$PYTHON"; then row+="| $(best_ms "$PYTHON" "./$b/$b.py") "; else row+="| - "; fi
    if have "$GOBLIN"; then
        row+="| $(best_ms "$WORK/$b.goblin.exe") "
        row+="| $(best_ms "$GOBLIN" run "./$b/$b.goblin") "
    else
        row+="| - | - "
    fi
    echo "$row|"
done
echo

if have "$GOBLIN"; then
    echo "Goblin transpile + go build time (not included above):"
    for b in "${BENCHES[@]}"; do echo "  $b: ${GOBLINBUILD[$b]}ms"; done
fi
