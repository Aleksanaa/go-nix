package nixjson

import (
	"math"
	"testing"
)

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		f    float64
		want string
	}{
		{0, "0.0"},
		{math.Copysign(0, -1), "-0.0"},
		{1, "1.0"},
		{1.5, "1.5"},
		{1000000, "1000000.0"},
		{123.456, "123.456"},
		{0.0001, "0.0001"},
		{0.000001, "1e-06"},
		{1e20, "1e+20"},
	}
	for _, test := range tests {
		if got := FormatFloat(test.f); got != test.want {
			t.Errorf("FormatFloat(%v) = %q, want %q", test.f, got, test.want)
		}
	}
}

func TestMarshal(t *testing.T) {
	got, err := Marshal(map[string]any{"b": 1, "a": "<>&"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":"<>&","b":1}`
	if string(got) != want {
		t.Errorf("Marshal = %s, want %s", got, want)
	}
}

func TestFloatMarshal(t *testing.T) {
	// nlohmann::json writes non-finite floats as null; JSON cannot hold them.
	got, err := Marshal([]any{Float(1), Float(1.5), Float(math.Inf(1)), Float(math.Inf(-1)), Float(math.NaN())})
	if err != nil {
		t.Fatal(err)
	}
	want := `[1.0,1.5,null,null,null]`
	if string(got) != want {
		t.Errorf("Marshal = %s, want %s", got, want)
	}
}
