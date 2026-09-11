package nixhash

import "testing"

// TestParseHashUnpaddedSRI pins that an SRI hash is accepted without its
// trailing '=' padding, as Nix accepts it: nixpkgs carries hashes in that form,
// and rejecting one turns a fixed-output derivation into an evaluation error.
func TestParseHashUnpaddedSRI(t *testing.T) {
	const (
		unpadded = "sha256-UlI+6OMUj5F6uVAw+Mg2wOZrjfdRq73d1qufaXVI/go"
		base16   = "52523ee8e3148f917ab95030f8c836c0e66b8df751abbdddd6ab9f697548fe0a"
	)
	got, algo, err := ParseHash(unpadded, "")
	if err != nil {
		t.Fatal(err)
	}
	if got != base16 {
		t.Errorf("ParseHash(%s) = %s, want %s", unpadded, got, base16)
	}
	if algo != "sha256" {
		t.Errorf("ParseHash(%s) algo = %s, want sha256", unpadded, algo)
	}
}

// TestBase32 covers Nix's base-32 hash encoding, whose byte order is the
// reverse of the usual one: it packs the digest starting from the last byte.
// A decoder that does not undo that yields a digest that is byte-reversed,
// which silently changes every fixed-output derivation that uses a legacy
// `sha256 = "1z4…"` hash.
func TestBase32(t *testing.T) {
	// nix hash convert --to base16 \
	//   --hash-algo sha256 1z4bibjm7ldvjwq3hmyifyb429rs2d9bdwkvs0r171vv1khpdwmb
	const (
		nix32  = "1z4bibjm7ldvjwq3hmyifyb429rs2d9bdwkvs0r171vv1khpdwmb"
		base16 = "abf276e10c7b871332d07bf2b652133a27419677d157383097bbd153e58a8bfc"
	)

	got, err := ConvertHash(nix32, "sha256", "base16")
	if err != nil {
		t.Fatal(err)
	}
	if got != base16 {
		t.Errorf("ConvertHash(%s) = %s, want %s", nix32, got, base16)
	}

	// Decoding must invert encoding.
	digest, err := base32Decode(nix32, 32)
	if err != nil {
		t.Fatal(err)
	}
	if reencoded := string(base32Encode(digest)); reencoded != nix32 {
		t.Errorf("base32Encode(base32Decode(%s)) = %s, want %s", nix32, reencoded, nix32)
	}
}
