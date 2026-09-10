package eval

import (
	"regexp"
	"strings"
)

func bConcatStringsSep(args ...*Expression) NixValue {
	sep := assertString(args[0].Eval())
	list := assertList(args[1].Eval())
	result := &NixString{}
	result.absorb(sep)
	parts := make([]string, len(list))
	for i, x := range list {
		str := CoerceToString(x.Eval())
		parts[i] = str.Content
		result.absorb(str)
	}
	result.Content = strings.Join(parts, sep.Content)
	return StrValue(result)
}

func bStringLength(args ...*Expression) NixValue {
	// Nix measures strings in bytes, not in characters.
	return Int(int64(len(CoerceToString(args[0].Eval()).Content)))
}

// bSubstring returns the substring at [start, start+length). A length of -1
// means "to the end", and a range past the end is silently truncated.
func bSubstring(args ...*Expression) NixValue {
	start := int64(assertInt(args[0].Eval()))
	length := int64(assertInt(args[1].Eval()))
	str := CoerceToString(args[2].Eval())
	if start < 0 {
		throwf(ErrEval, "negative start position %d in substring", start)
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

func bToString(args ...*Expression) NixValue { return StrValue(ToString(args[0].Eval())) }

// bReplaceStrings replaces every occurrence of each string in the first list
// with the string at the same index of the second, scanning left to right and
// preferring the earliest listed match.
func bReplaceStrings(args ...*Expression) NixValue {
	from := assertList(args[0].Eval())
	to := assertList(args[1].Eval())
	if len(from) != len(to) {
		throwf(ErrEval, "replaceStrings needs lists of equal length, got %d and %d", len(from), len(to))
	}
	str := assertString(args[2].Eval())
	needles := make([]string, len(from))
	for i, x := range from {
		needles[i] = assertString(x.Eval()).Content
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
			replacement := assertString(to[j].Eval())
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
func bMatch(args ...*Expression) NixValue {
	re := compileRegex(assertString(args[0].Eval()).Content)
	str := assertString(args[1].Eval())
	m := re.FindStringSubmatchIndex(str.Content)
	if m == nil || m[0] != 0 || m[1] != len(str.Content) {
		return Null
	}
	return ListValue(groupList(re, str.Content, m))
}

// bSplit splits a string on a regular expression. The result alternates
// between the text between matches and the list of capture groups of each
// match, as Nix specifies.
func bSplit(args ...*Expression) NixValue {
	re := compileRegex(assertString(args[0].Eval()).Content)
	str := assertString(args[1].Eval()).Content
	matches := re.FindAllStringSubmatchIndex(str, -1)
	result := make(NixList, 0, 2*len(matches)+1)
	last := 0
	for _, m := range matches {
		if m[1] == m[0] && m[0] == last && last != 0 {
			continue // skip an empty match adjacent to the previous one
		}
		result = append(result, value(String(str[last:m[0]])), value(ListValue(groupList(re, str, m))))
		last = m[1]
	}
	return ListValue(append(result, value(String(str[last:]))))
}

func compileRegex(pattern string) *regexp.Regexp {
	re, err := regexp.CompilePOSIX(pattern)
	if err != nil {
		throwf(ErrEval, "invalid regular expression %q: %s", pattern, err)
	}
	// POSIX requires leftmost-longest matching, which Go only does on request.
	re.Longest()
	return re
}

// groupList turns the capture groups of a match into a list, with null for
// each group that did not participate.
func groupList(re *regexp.Regexp, s string, m []int) NixList {
	groups := make(NixList, re.NumSubexp())
	for i := range groups {
		start, end := m[2*(i+1)], m[2*(i+1)+1]
		if start < 0 {
			groups[i] = value(Null)
		} else {
			groups[i] = value(String(s[start:end]))
		}
	}
	return groups
}
