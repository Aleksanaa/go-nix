package eval

import "path"

// NixPath is a filesystem path value.
type NixPath struct {
	Root string
	Path string
}

func (p *NixPath) String() string { return path.Join(p.Root, p.Path) }

func (p *NixPath) Print(recurse int) string { return p.String() }

// Join extends a path with a relative component, as `./a + "/b"` does.
func (p *NixPath) Join(s string) *NixPath {
	return &NixPath{Root: p.Root, Path: path.Join(p.Path, s)}
}

func (p *NixPath) Compare(val NixValue) bool {
	p2, ok := val.(*NixPath)
	return ok && p.String() == p2.String()
}
