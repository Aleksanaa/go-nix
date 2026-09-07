# The higher-order list builtins: map, filter, sort and a strict fold, each
# calling back into a Nix function once per element.
#
# `n` is on its own line so that examples/bench.sh can rewrite it.
let
  n = 250000;

  xs = builtins.genList (i: i) n;
  doubled = map (x: x * 2) xs;
  evens = builtins.filter (x: builtins.bitAnd x 3 == 0) doubled;
  total = builtins.foldl' (acc: x: acc + x) 0 evens;

  # Sorting calls the comparator O(n log n) times, on a scrambled list.
  scrambled = builtins.genList (i: (i * 7919) - (i * 7919 / 4096) * 4096) 4096;
  sorted = builtins.sort (p: q: p < q) scrambled;
in
"n=${toString n} kept=${toString (builtins.length evens)} total=${toString total}"
+ " min=${toString (builtins.head sorted)}"
+ " max=${toString (builtins.elemAt sorted (builtins.length sorted - 1))}"
