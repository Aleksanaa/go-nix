package eval

import (
	_ "embed"
	"strings"
)

// fetchurlNix is the core package nix embeds in its binary and serves from
// <nix/fetchurl.nix>. It is how nixpkgs reaches the fetchurl derivation
// without a channel, and it must evaluate to the same derivation as Nix does.
//
//go:embed corepkgs/fetchurl.nix
var fetchurlNix string

// corepkgs is the <nix/...> files the evaluator provides itself, keyed by the
// path they are reached at. These are not part of NIX_PATH; they are built in,
// as Nix's MemorySourceAccessor corepkgsFS is.
var corepkgs = map[string]string{
	"fetchurl.nix": fetchurlNix,
}

// corepkgPrefix marks a resolved <nix/...> path so the source layer knows it is
// embedded rather than on the file system.
const corepkgPrefix = "corepkgs:"

// corepkgContent returns the embedded contents of a resolved <nix/...> path.
func corepkgContent(p string) (string, bool) {
	name, ok := strings.CutPrefix(p, corepkgPrefix)
	if !ok {
		return "", false
	}
	content, ok := corepkgs[name]
	return content, ok
}
