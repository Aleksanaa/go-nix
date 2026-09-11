package daemon

// Worker protocol operation codes, from Nix's WorkerProto::Op.
const (
	opIsValidPath     = 1
	opAddToStore      = 7
	opBuildPaths      = 9
	opQueryPathInfo   = 26
	opQueryValidPaths = 31
	opAddToStoreNar   = 39
)

// IsValidPath reports whether the daemon has the given store path.
func (c *Conn) IsValidPath(path string) (bool, error) {
	c.op(opIsValidPath)
	c.putString(path)
	if err := c.finish(); err != nil {
		return false, err
	}
	valid := c.getUint64() != 0
	if err := c.cause(); err != nil {
		return false, err
	}
	return valid, nil
}

// QueryValidPaths returns the subset of paths the daemon has, substituting
// them first when substitute is set.
func (c *Conn) QueryValidPaths(paths []string, substitute bool) ([]string, error) {
	c.op(opQueryValidPaths)
	c.putStrings(paths)
	if c.version >= 1<<8|27 {
		c.putUint64(boolUint(substitute))
	}
	if err := c.finish(); err != nil {
		return nil, err
	}
	valid := c.getStrings()
	if err := c.cause(); err != nil {
		return nil, err
	}
	return valid, nil
}

func boolUint(b bool) uint64 {
	if b {
		return 1
	}
	return 0
}
