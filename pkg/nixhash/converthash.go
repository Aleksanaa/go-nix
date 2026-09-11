package nixhash

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

// algoSizes is the digest size in bytes of each algorithm builtins.convertHash
// understands.
var algoSizes = map[string]int{
	"md5": 16, "sha1": 20, "sha256": 32, "sha512": 64,
}

// ConvertHash re-encodes a hash between formats, as builtins.convertHash does.
// The input may name its algorithm ("sha256:…" or the SRI "sha256-…");
// otherwise algo supplies it. format is "base16", "nix32" (or its alias
// "base32"), "base64" or "sri".
func ConvertHash(s, algo, format string) (string, error) {
	bytes, inputAlgo, err := decodeHash(s, algo, false)
	if err != nil {
		return "", err
	}
	return encodeHash(bytes, inputAlgo, format)
}

// ParseHash parses a hash in any of Nix's encodings and returns the base-16
// digest together with the algorithm it resolved to. An empty string is the
// zero hash of algo's size, which Nix's newHashAllowEmpty accepts; algo is used
// when the string does not name its own.
func ParseHash(s, algo string) (base16, resolvedAlgo string, err error) {
	bytes, resolvedAlgo, err := decodeHash(s, algo, true)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(bytes), resolvedAlgo, nil
}

func decodeHash(s, algo string, allowEmpty bool) (Hash, string, error) {
	if s == "" && allowEmpty {
		size, ok := algoSizes[algo]
		if !ok {
			return nil, "", fmt.Errorf("hash '%s' does not include a type", s)
		}
		return make(Hash, size), algo, nil
	}
	inputAlgo, rest, isSRI := splitHashPrefix(s)
	if inputAlgo == "" {
		inputAlgo = algo
	}
	if inputAlgo == "" {
		return nil, "", fmt.Errorf("hash '%s' does not include a type", s)
	}
	size, ok := algoSizes[inputAlgo]
	if !ok {
		return nil, "", fmt.Errorf("unknown hash algorithm '%s'", inputAlgo)
	}

	var bytes []byte
	var err error
	switch {
	case isSRI:
		bytes, err = base64.StdEncoding.DecodeString(rest)
	default:
		switch len(rest) {
		case size * 2:
			bytes, err = hex.DecodeString(rest)
		case (size*8 + 4) / 5:
			bytes, err = base32Decode(rest, size)
		default:
			bytes, err = base64.StdEncoding.DecodeString(rest)
		}
	}
	if err != nil {
		return nil, "", fmt.Errorf("invalid hash '%s': %v", s, err)
	}
	if len(bytes) != size {
		return nil, "", fmt.Errorf("hash '%s' has the wrong length", s)
	}
	return Hash(bytes), inputAlgo, nil
}

func encodeHash(bytes []byte, algo, format string) (string, error) {
	switch format {
	case "base16":
		return hex.EncodeToString(bytes), nil
	case "nix32", "base32":
		return string(base32Encode(bytes)), nil
	case "base64":
		return base64.StdEncoding.EncodeToString(bytes), nil
	case "sri":
		return algo + "-" + base64.StdEncoding.EncodeToString(bytes), nil
	}
	return "", fmt.Errorf("unknown hash format '%s', expected 'base16', 'base32', 'base64' or 'sri'", format)
}

func splitHashPrefix(s string) (algo, rest string, isSRI bool) {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:], false
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return "", s, false
}

// base32Decode undoes base32Encode: the Nix base-32 digits, read as a
// big-endian integer, with the low size*8 bits being the data.
func base32Decode(s string, size int) ([]byte, error) {
	n := new(big.Int)
	for i := 0; i < len(s); i++ {
		idx := strings.IndexByte(base32Chars, s[i])
		if idx < 0 {
			return nil, fmt.Errorf("invalid base-32 digit '%c'", s[i])
		}
		n.Lsh(n, 5)
		n.Or(n, big.NewInt(int64(idx)))
	}
	// The stream is left-padded to a multiple of five bits; the data is the
	// trailing size bytes.
	mask := new(big.Int).Lsh(big.NewInt(1), uint(size*8))
	mask.Sub(mask, big.NewInt(1))
	n.And(n, mask)
	out := make([]byte, size)
	n.FillBytes(out)
	return out, nil
}
