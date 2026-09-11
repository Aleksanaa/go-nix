package daemon

import "io"

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

// AddToStore adds a source path to the store and returns the store path the
// daemon assigned. name is the path's base name and references the store paths
// the source refers to (none for an ordinary path literal); dump writes the
// path's NAR to the writer it is given.
func (c *Conn) AddToStore(name string, references []string, dump func(io.Writer) error) (string, error) {
	c.op(opAddToStore)
	c.putString(name)
	c.putString("fixed:r:sha256") // recursive NAR, sha256
	c.putStrings(references)
	c.putUint64(0) // repair

	fw := &framedWriter{c: c}
	if err := dump(fw); err != nil {
		return "", err
	}
	fw.close()

	if err := c.finish(); err != nil {
		return "", err
	}
	return c.readValidPathInfo()
}

// readValidPathInfo reads the StorePath and UnkeyedValidPathInfo the daemon
// sends in reply, returning the path.
func (c *Conn) readValidPathInfo() (string, error) {
	path := c.getString()
	c.getString()  // deriver: "" or a store path
	c.getString()  // nar hash
	c.getStrings() // references
	c.getUint64()  // registration time
	c.getUint64()  // nar size
	if c.version >= 1<<8|16 {
		c.getUint64()  // ultimate
		c.getStrings() // signatures
		c.getString()  // content address
	}
	return path, c.cause()
}

// framedChunk is how much of a NAR is sent per length-prefixed chunk. The
// daemon reassembles them, so the size only trades memory for round trips.
const framedChunk = 32 * 1024

// framedWriter sends a byte stream to the daemon in the length-prefixed chunks
// its FramedSource expects, terminated by a zero length.
type framedWriter struct {
	c   *Conn
	buf []byte
}

func (f *framedWriter) Write(p []byte) (int, error) {
	f.buf = append(f.buf, p...)
	for len(f.buf) >= framedChunk {
		f.emit(framedChunk)
	}
	return len(p), nil
}

func (f *framedWriter) emit(n int) {
	f.c.putUint64(uint64(n))
	if _, err := f.c.w.Write(f.buf[:n]); err != nil && f.c.err == nil {
		f.c.err = err
	}
	f.buf = append(f.buf[:0], f.buf[n:]...)
}

func (f *framedWriter) close() {
	if len(f.buf) > 0 {
		f.emit(len(f.buf))
	}
	f.c.putUint64(0)
}
