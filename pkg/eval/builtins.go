package eval

// A builtin is a primitive operation exposed through the `builtins` set.
//
// Arity is the number of arguments the implementation takes at once; Nix
// functions are curried, so applying fewer yields a partial application.
// Global is set for the handful of builtins Nix also exposes unprefixed, such
// as `map` and `throw`; the rest are reachable as `builtins.name` and as the
// underscored `__name`.
type builtin struct {
	arity  int
	fn     func(...*Expression) NixValue
	doc    string
	global bool
}

var builtins = map[string]builtin{
	// Arithmetic and comparison.
	"add":      {2, bAdd, "Add two numbers.", false},
	"sub":      {2, bSub, "Subtract the second number from the first.", false},
	"mul":      {2, bMul, "Multiply two numbers.", false},
	"div":      {2, bDiv, "Divide the first number by the second.", false},
	"lessThan": {2, bLessThan, "Compare two numbers or strings.", false},
	"bitAnd":   {2, bBitAnd, "Bitwise AND of two integers.", false},
	"bitOr":    {2, bBitOr, "Bitwise OR of two integers.", false},
	"bitXor":   {2, bBitXor, "Bitwise XOR of two integers.", false},
	"ceil":     {1, bCeil, "Round a number up to an integer.", false},
	"floor":    {1, bFloor, "Round a number down to an integer.", false},
	"compareVersions": {2, bCompareVersions,
		"Compare two version strings; -1, 0 or 1 as the first sorts before, with, or after the second.", false},

	// Types.
	"typeOf":     {1, bTypeOf, "Name of the type of a value.", false},
	"isAttrs":    {1, bIsAttrs, "Whether a value is a set.", false},
	"isBool":     {1, bIsBool, "Whether a value is a Boolean.", false},
	"isFloat":    {1, bIsFloat, "Whether a value is a float.", false},
	"isFunction": {1, bIsFunction, "Whether a value is a function.", false},
	"isInt":      {1, bIsInt, "Whether a value is an integer.", false},
	"isList":     {1, bIsList, "Whether a value is a list.", false},
	"isNull":     {1, bIsNull, "Whether a value is null.", true},
	"isPath":     {1, bIsPath, "Whether a value is a path.", false},
	"isString":   {1, bIsString, "Whether a value is a string.", false},

	// Evaluation control.
	"seq":     {2, bSeq, "Evaluate the first argument, then return the second.", false},
	"deepSeq": {2, bDeepSeq, "Evaluate the first argument recursively, then return the second.", false},
	"throw":   {1, bThrow, "Fail evaluation with a message.", true},
	"abort":   {1, bAbort, "Abort evaluation with a message; not catchable by tryEval.", true},
	"tryEval": {1, bTryEval, "Evaluate a value, reporting whether it threw.", false},
	"trace":   {2, bTrace, "Print the first argument to stderr, then return the second.", false},

	// Lists.
	"all":         {2, bAll, "Whether a predicate holds for every element.", false},
	"any":         {2, bAny, "Whether a predicate holds for any element.", false},
	"concatLists": {1, bConcatLists, "Concatenate a list of lists.", false},
	"elem":        {2, bElem, "Whether a value occurs in a list.", false},
	"elemAt":      {2, bElemAt, "Element of a list at an index, counting from zero.", false},
	"filter":      {2, bFilter, "Elements of a list satisfying a predicate.", false},
	"foldl'":      {3, bFoldl, "Fold a list from the left, forcing each accumulator.", false},
	"genList":     {2, bGenList, "List of n values built from their indices.", false},
	"head":        {1, bHead, "First element of a list.", false},
	"length":      {1, bLength, "Number of elements in a list.", false},
	"map":         {2, bMap, "Apply a function to every element of a list.", true},
	"partition":   {2, bPartition, "Split a list into elements that do and do not satisfy a predicate.", false},
	"sort":        {2, bSort, "Sort a list with a strict less-than comparator.", false},
	"tail":        {1, bTail, "All but the first element of a list.", false},

	// Sets.
	"attrNames":    {1, bAttrNames, "Names of the attributes of a set, sorted.", false},
	"attrValues":   {1, bAttrValues, "Values of the attributes of a set, ordered by name.", false},
	"catAttrs":     {2, bCatAttrs, "Values of an attribute across a list of sets, skipping those without it.", false},
	"functionArgs": {1, bFunctionArgs, "Formal arguments of a function, mapped to whether they have a default.", false},
	"getAttr":      {2, bGetAttr, "Value of a named attribute of a set.", false},
	"groupBy":      {2, bGroupBy, "Group list elements into a set by a key function.", false},
	"hasAttr":      {2, bHasAttr, "Whether a set has a named attribute.", false},
	"intersectAttrs": {2, bIntersectAttrs,
		"Attributes of the second set whose names also occur in the first.", false},
	"listToAttrs": {1, bListToAttrs, "Build a set from a list of { name, value } sets.", false},
	"removeAttrs": {2, bRemoveAttrs, "A set without the named attributes.", true},

	// Strings.
	"concatStringsSep": {2, bConcatStringsSep, "Join a list of strings with a separator.", false},
	"match":            {2, bMatch, "Match a string against an anchored POSIX regular expression.", false},
	"replaceStrings":   {3, bReplaceStrings, "Replace occurrences of each string with its replacement.", false},
	"split":            {2, bSplit, "Split a string on a POSIX regular expression.", false},
	"stringLength":     {1, bStringLength, "Length of a string in bytes.", false},
	"substring":        {3, bSubstring, "Substring of a string, by start offset and length.", false},
	"toString":         {1, bToString, "Coerce a value to a string.", true},

	// Serialisation.
	"fromJSON": {1, bFromJSON, "Parse a JSON string into a Nix value.", false},
	"toJSON":   {1, bToJSON, "Render a value as JSON.", false},
}

// globals are the non-function names Nix predefines.
var globals = map[string]NixValue{
	"true":  True,
	"false": False,
	"null":  Null,
}

// DefaultScope is the scope every evaluation starts in: the globals, the
// underscored builtins, and `builtins` itself.
var DefaultScope = newDefaultScope()

func newDefaultScope() *Scope {
	builtinsSet := make(NixSet, len(builtins)+len(globals)+1)
	mainSet := make(NixSet, len(builtins)+len(globals)+1)

	for name, b := range builtins {
		sym := Intern(name)
		op := &NixPrimop{Func: b.fn, ArgNum: b.arity, Doc: b.doc, Sym: sym}
		builtinsSet[sym] = value(op)
		if b.global {
			mainSet[sym] = value(op)
		}
		mainSet[Intern("__"+name)] = value(op)
	}
	for name, val := range globals {
		sym := Intern(name)
		builtinsSet[sym] = value(val)
		mainSet[sym] = value(val)
	}

	builtinsSet[symBuiltins] = value(builtinsSet)
	mainSet[symBuiltins] = value(builtinsSet)
	return &Scope{Binds: mainSet}
}
