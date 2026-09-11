package source

import (
	"testing"

	"github.com/alecthomas/assert"
)

func TestSplitNixPath(t *testing.T) {
	tests := map[string][][2]string{
		"":          nil,
		"a":         {{"", "a"}},
		"a:b:c":     {{"", "a"}, {"", "b"}, {"", "c"}},
		"p=b":       {{"p", "b"}},
		"a:p=b:c":   {{"", "a"}, {"p", "b"}, {"", "c"}},
		"a:p=b:q=c": {{"", "a"}, {"p", "b"}, {"q", "c"}},
	}
	for q, a := range tests {
		assert.Equal(t, a, splitNixPath(q), q)
	}
}

func TestLookupPath(t *testing.T) {
	no := func(string) bool { return false }
	yes := func(string) bool { return true }
	assert.Equal(t, "", lookupPath("f", [][2]string{{"", "/a"}}, no))
	assert.Equal(t, "/a/f", lookupPath("f", [][2]string{{"", "/a"}}, yes))
	assert.Equal(t, "/b/f", lookupPath("p/f", [][2]string{{"p", "/b"}}, yes))
	assert.Equal(t, "", lookupPath("pa/f", [][2]string{{"p", "/b"}}, yes))
}
