# A fixpoint with an overlay, which is how Nixpkgs is put together: a package
# set defined in terms of itself, extended by a function that receives both the
# final set and the one it is overriding.
#
# Exercises recursion through laziness, set construction, `//` and the
# attribute-set idioms `lib` is built from.
#
# `n` is on its own line so that examples/bench.sh can rewrite it.
let
  n = 60000;

  fix = f: let x = f x; in x;

  mapAttrs = f: set:
    builtins.listToAttrs (map (name: {
      inherit name;
      value = f name set.${name};
    }) (builtins.attrNames set));

  base = self: builtins.listToAttrs (builtins.genList (i: {
    name = "p${toString i}";
    value = { pname = "p${toString i}"; version = 1 + i; };
  }) n);

  # An overlay that bumps every version and, as a real one would, reaches back
  # through the fixpoint. `buddy` stays a thunk until the checksum forces it.
  overlay = self: super:
    mapAttrs (name: pkg: pkg // {
      version = pkg.version + 1;
      buddy = self.p0.pname;
    }) super;

  pkgs = fix (self: let s = base self; in s // overlay self s);

  names = builtins.attrNames pkgs;
  total = builtins.foldl' (acc: name:
    acc + pkgs.${name}.version + builtins.stringLength pkgs.${name}.buddy) 0 names;
in
"n=${toString n} pkgs=${toString (builtins.length names)} total=${toString total}"
