// Package source is the evaluator's access to the file system: resolving the
// paths a Nix expression names — path literals, <angle-bracket> lookups — and
// reading files and directories. It is what stands in for Nix's source
// accessor while there is no store, so everything here is read-only.
package source

import (
	"os"
	"path"
	"sort"
	"strings"
)

// NixPath is the NIX_PATH environment variable split into name/value pairs,
// which is how an <angle-bracket> path is looked up.
var NixPath = splitNixPath(os.Getenv("NIX_PATH"))

func splitNixPath(s string) [][2]string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ":")
	entries := make([][2]string, len(parts))
	for i, part := range parts {
		entries[i][1] = part
		if strings.Contains(part, "=") {
			copy(entries[i][:], strings.SplitN(part, "=", 2))
		}
	}
	return entries
}

// Resolve makes a path literal absolute. A relative path is resolved against
// base, the directory of the file that contains the literal; an absolute path
// is cleaned; and an <angle-bracket> path is looked up in NIX_PATH, reported
// as "" when it is not found.
func Resolve(base, s string) string {
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return searchPath(s[1 : len(s)-1])
	}
	if path.IsAbs(s) {
		return path.Clean(s)
	}
	return path.Clean(path.Join(base, s))
}

func searchPath(name string) string {
	for _, pair := range NixPath {
		k, v := pair[0], pair[1]
		var r string
		switch {
		case k == "":
			r = path.Join(v, name)
		case name == k || strings.HasPrefix(name, k+"/"):
			r = v + name[len(k):]
		default:
			continue
		}
		if Exists(r) {
			return r
		}
	}
	return ""
}

// Dir is the directory a file lives in, which is what the relative paths
// inside it are resolved against.
func Dir(p string) string { return path.Dir(p) }

// Abs makes a path absolute against the current working directory, so that a
// file parsed from a relative path still resolves its own paths to absolute
// ones, as Nix does.
func Abs(p string) string {
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path.Clean(p)
	}
	return path.Clean(path.Join(cwd, p))
}

// ReadFile reads a file's contents.
func ReadFile(p string) ([]byte, error) { return os.ReadFile(p) }

// Entry is one directory entry, as builtins.readDir reports it.
type Entry struct {
	Name string
	Type string // "regular", "directory", "symlink", "unknown"
}

// ReadDir lists a directory's entries, sorted by name.
func ReadDir(p string) ([]Entry, error) {
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		result = append(result, Entry{Name: e.Name(), Type: entryType(e)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func entryType(e os.DirEntry) string {
	switch {
	case e.Type().IsRegular():
		return "regular"
	case e.IsDir():
		return "directory"
	case e.Type()&os.ModeSymlink != 0:
		return "symlink"
	}
	return "unknown"
}

// Exists reports whether a path exists.
func Exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// IsDir reports whether a path exists and is a directory, following symlinks
// the way import does when it decides whether to open default.nix.
func IsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
