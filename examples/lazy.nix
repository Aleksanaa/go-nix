# Laziness: build a large structure and force almost none of it.
#
# Both evaluators leave a list element and an attribute value unevaluated until
# something asks for it, so this measures the cost of creating thunks rather
# than of running them.
#
# `n` is on its own line so that examples/bench.sh can rewrite it.
let
  n = 3000000;

  xs = builtins.genList (i: { value = i * i; label = "x${toString i}"; }) n;

  # One element in every hundred is ever looked at.
  picked = builtins.genList (i: (builtins.elemAt xs (i * 100)).value) (n / 100);
  total = builtins.foldl' (acc: v: acc + v) 0 picked;
in
"n=${toString n} forced=${toString (builtins.length picked)} total=${toString total}"
