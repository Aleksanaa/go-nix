package daemon

import (
	"os"
	"testing"
)

func dialTest(t *testing.T) *Conn {
	t.Helper()
	if _, err := os.Stat(SocketPath()); err != nil {
		t.Skipf("no nix daemon socket at %s", SocketPath())
	}
	c, err := Dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestHandshake(t *testing.T) {
	c := dialTest(t)
	t.Logf("protocol %d.%d, daemon %q, trusted=%v",
		c.Version()>>8, c.Version()&0xff, c.DaemonNixVersion(), c.Trusted())
	if c.Version()>>8 != 1 {
		t.Fatalf("unexpected protocol major %d", c.Version()>>8)
	}
}

func TestQueryValidPaths(t *testing.T) {
	c := dialTest(t)
	// Pick an existing store path.
	entries, err := os.ReadDir("/nix/store")
	if err != nil {
		t.Skip("no readable /nix/store")
	}
	var path string
	for _, e := range entries {
		n := e.Name()
		if len(n) > 33 && n[32] == '-' {
			path = "/nix/store/" + n
			break
		}
	}
	if path == "" {
		t.Skip("no store path found")
	}
	valid, err := c.QueryValidPaths([]string{path}, false)
	if err != nil {
		t.Fatalf("QueryValidPaths: %v", err)
	}
	if len(valid) != 1 || valid[0] != path {
		t.Fatalf("QueryValidPaths(%s) = %v, want itself", path, valid)
	}
}
