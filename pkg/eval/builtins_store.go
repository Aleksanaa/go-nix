package eval

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"sort"
	"strings"

	"github.com/aleksanaa/go-nix/pkg/nixhash"
)

// The store builtins: the small primitives a derivation needs to name its
// outputs and sources, and to hash and manipulate strings and paths.

// bPlaceholder implements builtins.placeholder: the string an output is known
// by before it is built.
func bPlaceholder(w *worker, args ...*Expression) NixValue {
	return String(nixhash.OutputPlaceholder(assertString(w, args[0].Eval(w)).Content))
}

// bToFile implements builtins.toFile: store a string in a file and return the
// file's store path. The path is computed in memory, like a derivation's.
func bToFile(w *worker, args ...*Expression) NixValue {
	name := assertString(w, args[0].Eval(w)).Content
	contents := assertString(w, args[1].Eval(w))

	var refs []string
	if contents.extra != nil {
		for _, c := range contents.extra.Context {
			if c.path != "" {
				refs = append(refs, c.path)
			} else {
				w.throwf(ErrEval,
					"files created by builtins.toFile may not reference derivations, but '%s' references a derivation",
					name)
			}
		}
	}
	sort.Strings(refs)
	refs = slicesCompact(refs)

	storePath := nixhash.TextStorePath(name, contents.Content, refs)
	return StrValue(stringWithContext(storePath, stringContext{path: storePath}))
}

// bHashString implements builtins.hashString: the base-16 digest of a string.
func bHashString(w *worker, args ...*Expression) NixValue {
	algo := assertString(w, args[0].Eval(w)).Content
	s := assertString(w, args[1].Eval(w)).Content
	var sum []byte
	switch algo {
	case "md5":
		h := md5.Sum([]byte(s))
		sum = h[:]
	case "sha1":
		h := sha1.Sum([]byte(s))
		sum = h[:]
	case "sha256":
		h := sha256.Sum256([]byte(s))
		sum = h[:]
	case "sha512":
		h := sha512.Sum512([]byte(s))
		sum = h[:]
	default:
		w.throwf(ErrEval, "unknown hash algorithm '%s'", algo)
	}
	return String(hex.EncodeToString(sum))
}

// bBaseNameOf implements builtins.baseNameOf, using Nix's legacy string form:
// one trailing slash is dropped, then the part after the last remaining slash
// is returned.
func bBaseNameOf(w *worker, args ...*Expression) NixValue {
	str := CoerceToString(w, args[0].Eval(w))
	return StrValue(withContext(legacyBaseNameOf(str.Content), str))
}

func legacyBaseNameOf(s string) string {
	if s == "" {
		return ""
	}
	last := len(s) - 1
	if s[last] == '/' && last > 0 {
		last--
	}
	if pos := strings.LastIndex(s[:last+1], "/"); pos >= 0 {
		return s[pos+1 : last+1]
	}
	return s[:last+1]
}

// bDirOf implements builtins.dirOf: everything before the final slash.
func bDirOf(w *worker, args ...*Expression) NixValue {
	val := args[0].Eval(w)
	if val.Kind() == KindPath {
		p := val.Path()
		if i := strings.LastIndex(p.Path, "/"); i >= 0 {
			return PathValue(&NixPath{Root: p.Root, Path: p.Path[:i]})
		}
		return PathValue(&NixPath{Root: p.Root})
	}
	str := CoerceToString(w, val)
	pos := strings.LastIndex(str.Content, "/")
	var dir string
	switch {
	case pos < 0:
		dir = "."
	case pos == 0:
		dir = "/"
	default:
		dir = str.Content[:pos]
	}
	return StrValue(withContext(dir, str))
}

// bUnsafeDiscardStringContext implements builtins.unsafeDiscardStringContext:
// the string without the derivations it refers to.
func bUnsafeDiscardStringContext(w *worker, args ...*Expression) NixValue {
	str := CoerceToString(w, args[0].Eval(w))
	return String(str.Content)
}

// bHasContext implements builtins.hasContext: whether a string refers to any
// derivation or store path.
func bHasContext(w *worker, args ...*Expression) NixValue {
	str := assertString(w, args[0].Eval(w))
	return Bool(str.extra != nil && len(str.extra.Context) != 0)
}
