# String work: interpolation, concatenation and the string builtins.
#
# Every iteration builds a fresh string, so this measures how much a string
# operation costs rather than how the evaluator is structured.
#
# `n` is on its own line so that examples/bench.sh can rewrite it.
let
  n = 250000;

  # Interpolation and a substring, keeping the accumulator a fixed length so
  # the work per iteration stays constant.
  step = acc: i: builtins.substring 0 24 "item-${toString i}-${acc}";
  final = builtins.foldl' step "seed" (builtins.genList (i: i) n);

  joined = builtins.concatStringsSep "," (builtins.genList (i: toString i) 2000);
  swapped = builtins.replaceStrings [ "," "1" ] [ ";" "one" ] joined;
in
"n=${toString n} final=${final} joined=${toString (builtins.stringLength joined)}"
+ " swapped=${toString (builtins.stringLength swapped)}"
