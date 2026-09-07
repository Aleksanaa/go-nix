#!/usr/bin/env bash
# Compare gon against nix on the same expression.
#
#   examples/bench.sh                          # hanoi.nix, 10 to 18 disks
#   examples/bench.sh hanoi-calls.nix 12 20
#   GON=./result/bin/gon examples/bench.sh     # time a binary you already have
#
# Both evaluators are asked for the same value and their answers are compared,
# so a timing is only reported once the two agree.
set -euo pipefail

here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(dirname "$here")

file=${1:-hanoi.nix}
from=${2:-10}
to=${3:-18}
repeats=${REPEATS:-3}

nix=${NIX:-$(command -v nix-instantiate || true)}
if [ -z "$nix" ]; then
  echo "nix-instantiate not found" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

gon=${GON:-$work/gon}
if [ ! -x "$gon" ]; then
  echo "building gon..." >&2
  go build -C "$root" -o "$gon" ./cmd/gon
fi

# best runs a command `repeats` times and echoes the shortest wall time in ms.
best() {
  local ms start end best=""
  for ((i = 0; i < repeats; i++)); do
    start=${EPOCHREALTIME/./}
    "$@" >/dev/null 2>&1
    end=${EPOCHREALTIME/./}
    ms=$(((end - start) / 1000))
    if [ -z "$best" ] || [ "$ms" -lt "$best" ]; then best=$ms; fi
  done
  echo "$best"
}

printf '%s, best of %d runs, times in ms\n\n' "$file" "$repeats"
printf '%6s  %10s  %10s  %8s\n' disks gon nix ratio
printf '%6s  %10s  %10s  %8s\n' ------ ---------- ---------- --------

for ((n = from; n <= to; n++)); do
  src=$work/$n.nix
  sed "s/^  disks = .*/  disks = $n;/" "$here/$file" > "$src"

  a=$("$gon" eval -f "$src")
  b=$("$nix" --eval "$src")
  if [ "$a" != "$b" ]; then
    printf '%6d  MISMATCH\n  gon: %s\n  nix: %s\n' "$n" "$a" "$b"
    exit 1
  fi

  g=$(best "$gon" eval -f "$src")
  x=$(best "$nix" --eval "$src")
  printf '%6d  %10d  %10d  %7sx\n' "$n" "$g" "$x" \
    "$(awk -v g="$g" -v x="$x" 'BEGIN { printf "%.2f", x / (g ? g : 1) }')"
done

echo
echo "ratio > 1 means gon was faster. Both timings include process startup:"
printf 'roughly %sms for gon and %sms for nix.\n' \
  "$(best "$gon" eval 1)" "$(best "$nix" --eval -E 1)"
