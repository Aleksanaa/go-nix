package eval

import (
	"testing"
	"unsafe"
)

// TestExpressionSize pins the size of the two objects evaluation allocates
// most of. Both sit in a size class with nothing to spare: an added field
// makes every thunk in a large evaluation cost the next class up. A thunk is
// 32 bytes because it reads its two words by kind rather than keeping a field
// for each thing it might be; see Expression.
func TestExpressionSize(t *testing.T) {
	if got, want := unsafe.Sizeof(Expression{}), uintptr(32); got != want {
		t.Errorf("Expression is %d bytes, want %d", got, want)
	}
	if got, want := unsafe.Sizeof(Scope{}), uintptr(32); got != want {
		t.Errorf("Scope is %d bytes, want %d", got, want)
	}
}
