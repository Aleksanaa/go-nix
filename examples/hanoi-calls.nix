# Towers of Hanoi counting only the moves it would make, so that the timing
# measures function calls and arithmetic rather than list building.
#
#   nix-instantiate --eval examples/hanoi-calls.nix
#   gon eval -f examples/hanoi-calls.nix
#
# `disks` is on its own line so that examples/bench.sh can rewrite it.
let
  disks = 18;

  count = n: if n == 0 then 0 else count (n - 1) + 1 + count (n - 1);
in
"disks=${toString disks} moves=${toString (count disks)}"
