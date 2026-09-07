package eval

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	p "github.com/orivej/go-nix/pkg/parser"
)

// Arithmetic.

func bAdd(args ...*Expression) NixValue { return Arith(args[0].Eval(), args[1].Eval(), p.OpAddNode) }
func bSub(args ...*Expression) NixValue { return Arith(args[0].Eval(), args[1].Eval(), p.OpReduceNode) }
func bMul(args ...*Expression) NixValue {
	return Arith(args[0].Eval(), args[1].Eval(), p.OpMultiplyNode)
}
func bDiv(args ...*Expression) NixValue { return Arith(args[0].Eval(), args[1].Eval(), p.OpDivideNode) }

// bLessThan orders values the way the < operator does.
func bLessThan(args ...*Expression) NixValue {
	return NixBool(CompareOrder(args[0].Eval(), args[1].Eval()) < 0)
}

func bBitAnd(args ...*Expression) NixValue {
	return assertInt(args[0].Eval()) & assertInt(args[1].Eval())
}

func bBitOr(args ...*Expression) NixValue {
	return assertInt(args[0].Eval()) | assertInt(args[1].Eval())
}

func bBitXor(args ...*Expression) NixValue {
	return assertInt(args[0].Eval()) ^ assertInt(args[1].Eval())
}

// bCeil and bFloor accept an integer as well, where they are the identity.
func bCeil(args ...*Expression) NixValue {
	return roundTo(args[0].Eval(), math.Ceil)
}

func bFloor(args ...*Expression) NixValue {
	return roundTo(args[0].Eval(), math.Floor)
}

func roundTo(val NixValue, round func(float64) float64) NixValue {
	f, ok := toFloat(val)
	if !ok {
		throwf(ErrType, "value is %s while a number was expected", anTypeName(val))
	}
	return NixInt(round(float64(f)))
}

// bCompareVersions implements Nix version ordering: versions are split into
// runs of digits and of letters, and compared component by component, with an
// empty component and "pre" sorting before anything else.
func bCompareVersions(args ...*Expression) NixValue {
	a := splitVersion(assertString(args[0].Eval()).Content)
	b := splitVersion(assertString(args[1].Eval()).Content)
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y string
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if c := compareVersionComponent(x, y); c != 0 {
			return NixInt(c)
		}
	}
	return NixInt(0)
}

func splitVersion(s string) []string {
	var parts []string
	for i := 0; i < len(s); {
		c := s[i]
		if c == '.' || c == '-' {
			i++
			continue
		}
		digit := isDigit(c)
		j := i
		for j < len(s) && s[j] != '.' && s[j] != '-' && isDigit(s[j]) == digit {
			j++
		}
		parts = append(parts, s[i:j])
		i = j
	}
	return parts
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func compareVersionComponent(a, b string) int {
	ai, aerr := strconv.Atoi(a)
	bi, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		return sign(ai - bi)
	case a == b:
		return 0
	// A "pre" component sorts before everything, an absent one before
	// everything but "pre".
	case a == "pre":
		return -1
	case b == "pre":
		return 1
	case a == "":
		return -1
	case b == "":
		return 1
	case aerr == nil: // a number sorts after a letter component
		return 1
	case berr == nil:
		return -1
	}
	return sign(strings.Compare(a, b))
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}

// Types.

func bTypeOf(args ...*Expression) NixValue { return String(TypeName(args[0].Eval())) }

func isType[T NixValue](x *Expression) NixValue {
	_, ok := x.Eval().(T)
	return NixBool(ok)
}

func bIsAttrs(args ...*Expression) NixValue    { return isType[NixSet](args[0]) }
func bIsBool(args ...*Expression) NixValue     { return isType[NixBool](args[0]) }
func bIsFloat(args ...*Expression) NixValue    { return isType[NixFloat](args[0]) }
func bIsFunction(args ...*Expression) NixValue { return isType[NixLambda](args[0]) }
func bIsInt(args ...*Expression) NixValue      { return isType[NixInt](args[0]) }
func bIsList(args ...*Expression) NixValue     { return isType[NixList](args[0]) }
func bIsNull(args ...*Expression) NixValue     { return isType[*NixNull](args[0]) }
func bIsPath(args ...*Expression) NixValue     { return isType[*NixPath](args[0]) }
func bIsString(args ...*Expression) NixValue   { return isType[*NixString](args[0]) }

// Evaluation control.

func bSeq(args ...*Expression) NixValue {
	args[0].Eval()
	return args[1].Eval()
}

func bDeepSeq(args ...*Expression) NixValue {
	deepForce(args[0].Eval())
	return args[1].Eval()
}

// deepForce evaluates a value and everything reachable from it.
func deepForce(val NixValue) {
	switch v := val.(type) {
	case NixList:
		for _, x := range v {
			deepForce(x.Eval())
		}
	case NixSet:
		for _, x := range v {
			deepForce(x.Eval())
		}
	}
}

func bThrow(args ...*Expression) NixValue {
	throwf(ErrThrown, "%s", assertString(args[0].Eval()).Content)
	return nil
}

func bAbort(args ...*Expression) NixValue {
	throwf(ErrAborted, "evaluation aborted with the following error message: '%s'",
		assertString(args[0].Eval()).Content)
	return nil
}

// bTryEval evaluates its argument shallowly, reporting failure instead of
// propagating it. As in Nix, only assertions and throws are caught: a type
// error still aborts the whole evaluation.
func bTryEval(args ...*Expression) (result NixValue) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if err := asEvalError(v); err == nil || !err.Kind.Catchable() {
			panic(v)
		}
		result = NixSet{symSuccess: value(False), symValue: value(False)}
	}()
	val := args[0].Eval()
	return NixSet{symSuccess: value(True), symValue: value(val)}
}

func bTrace(args ...*Expression) NixValue {
	fmt.Fprintln(os.Stderr, "trace:", printTraced(args[0].Eval()))
	return args[1].Eval()
}

// printTraced renders a traced value the way Nix does: strings unquoted,
// everything else printed one level deep.
func printTraced(val NixValue) string {
	if s, ok := val.(*NixString); ok {
		return s.Content
	}
	return val.Print(1)
}
