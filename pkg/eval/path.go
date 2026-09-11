package eval

import (
	"path"
	"strings"

	"github.com/aleksanaa/go-nix/pkg/source"
)

// NixPath is a filesystem path value. Path is the resolved, absolute, cleaned
// path, which is what a path literal becomes once it has been made absolute
// against the directory of the file that contains it.
type NixPath struct {
	Path string
}

func (p *NixPath) String() string { return p.Path }

// Join extends a path with a relative component, as `./a + "/b"` does.
func (p *NixPath) Join(s string) *NixPath {
	return &NixPath{Path: path.Join(p.Path, s)}
}

// resolvePathLiteral makes a path literal absolute: relative paths against the
// directory of the file that contains them, <angle> paths through NIX_PATH,
// absolute paths left as they are.
func resolvePathLiteral(base, s string) string {
	// <nix/...> names a core package embedded in the evaluator, not a path.
	if name, ok := strings.CutPrefix(s, "<nix/"); ok && strings.HasSuffix(name, ">") {
		name = strings.TrimSuffix(name, ">")
		if _, ok := corepkgs[name]; ok {
			return corepkgPrefix + name
		}
	}
	return source.Resolve(base, s)
}
