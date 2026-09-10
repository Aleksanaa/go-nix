package eval

import (
	"regexp"
	"strings"
)

func bConcatStringsSep(w *worker, args ...*Expression) NixValue {
	sep := assertString(w, args[0].Eval(w))
	list := assertList(w, args[1].Eval(w))
	result := &NixString{}
	result.absorb(sep)
	parts := make([]string, len(list))
	for i, x := range list {
		str := CoerceToString(w, x.Eval(w))
		parts[i] = str.Content
		result.absorb(str)
	}
	result.Content = strings.Join(parts, sep.Content)
	return StrValue(result)
}

func bStringLength(w *worker, args ...*Expression) NixValue {
	// Nix measures strings in bytes, not in characters.
	return Int(int64(len(CoerceToString(w, args[0].Eval(w)).Content)))
}

// bSubstring returns the substring at [start, start+length). A length of -1
// means "to the end", and a range past the end is silently truncated.
func bSubstring(w *worker, args ...*Expression) NixValue {
	start := int64(assertInt(w, args[0].Eval(w)))
	length := int64(assertInt(w, args[1].Eval(w)))
	str := CoerceToString(w, args[2].Eval(w))
	if start < 0 {
		w.throwf(ErrEval, "negative start position %d in substring", start)
	}
	if start >= int64(len(str.Content)) {
		return String("")
	}
	end := int64(len(str.Content))
	if length >= 0 && start+length < end {
		end = start + length
	}
	result := newString(str.Content[start:end])
	result.absorb(str)
	return StrValue(result)
}

func bToString(w *worker, args ...*Expression) NixValue {
	return StrValue(ToString(w, args[0].Eval(w)))
}

// bReplaceStrings replaces every occurrence of each string in the first list
// with the string at the same index of the second, scanning left to right and
// preferring the earliest listed match.
func bReplaceStrings(w *worker, args ...*Expression) NixValue {
	from := assertList(w, args[0].Eval(w))
	to := assertList(w, args[1].Eval(w))
	if len(from) != len(to) {
		w.throwf(ErrEval, "replaceStrings needs lists of equal length, got %d and %d", len(from), len(to))
	}
	str := assertString(w, args[2].Eval(w))
	needles := make([]string, len(from))
	for i, x := range from {
		needles[i] = assertString(w, x.Eval(w)).Content
	}

	result := &NixString{}
	var b strings.Builder
	s := str.Content
	for i := 0; i < len(s); {
		matched := false
		for j, needle := range needles {
			if needle == "" || !strings.HasPrefix(s[i:], needle) {
				continue
			}
			replacement := assertString(w, to[j].Eval(w))
			b.WriteString(replacement.Content)
			result.absorb(replacement)
			i += len(needle)
			matched = true
			break
		}
		if !matched {
			b.WriteByte(s[i])
			i++
		}
	}
	result.Content = b.String()
	result.absorb(str)
	return StrValue(result)
}

// bMatch matches an anchored POSIX regular expression, returning null when it
// does not match and the list of capture groups when it does.
func bMatch(w *worker, args ...*Expression) NixValue {
	re := compileRegex(w, assertString(w, args[0].Eval(w)).Content)
	str := assertString(w, args[1].Eval(w))
	m := re.FindStringSubmatchIndex(str.Content)
	if m == nil || m[0] != 0 || m[1] != len(str.Content) {
		return Null
	}
	return ListValue(groupList(w, re, str.Content, m))
}

// bSplit splits a string on a regular expression. The result alternates
// between the text between matches and the list of capture groups of each
// match, as Nix specifies.
func bSplit(w *worker, args ...*Expression) NixValue {
	re := compileRegex(w, assertString(w, args[0].Eval(w)).Content)
	str := assertString(w, args[1].Eval(w)).Content
	matches := re.FindAllStringSubmatchIndex(str, -1)
	result := make(NixList, 0, 2*len(matches)+1)
	last := 0
	for _, m := range matches {
		if m[1] == m[0] && m[0] == last && last != 0 {
			continue // skip an empty match adjacent to the previous one
		}
		result = append(result, value(w, String(str[last:m[0]])), value(w, ListValue(groupList(w, re, str, m))))
		last = m[1]
	}
	return ListValue(append(result, value(w, String(str[last:]))))
}

func compileRegex(w *worker, pattern string) *regexp.Regexp {
	re, err := regexp.CompilePOSIX(pattern)
	if err != nil {
		w.throwf(ErrEval, "invalid regular expression %q: %s", pattern, err)
	}
	// POSIX requires leftmost-longest matching, which Go only does on request.
	re.Longest()
	return re
}

// groupList turns the capture groups of a match into a list, with null for
// each group that did not participate.
func groupList(w *worker, re *regexp.Regexp, s string, m []int) NixList {
	groups := make(NixList, re.NumSubexp())
	for i := range groups {
		start, end := m[2*(i+1)], m[2*(i+1)+1]
		if start < 0 {
			groups[i] = value(w, Null)
		} else {
			groups[i] = value(w, String(s[start:end]))
		}
	}
	return groups
}
