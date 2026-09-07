package eval

import "strings"

// This file turns the text of a string literal into its value: escape
// sequences, and the indentation stripping of indented strings.

// unescapeQuoted expands the escapes of a "..." string. The lexer keeps the
// text as written, so a backslash always introduces an escape here.
func unescapeQuoted(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		b.WriteByte(escapeByte(s[i]))
	}
	return b.String()
}

// escapeByte maps the character after a backslash to what it stands for. Any
// other character stands for itself, which is how \" and \$ work.
func escapeByte(c byte) byte {
	switch c {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	}
	return c
}

// unescapeIndented expands the escapes of an indented string, where the escape
// character is a doubled quote: ” for a literal ” (written as three quotes
// in the source), ”$ for a bare dollar, and ”\c for the same escapes a
// quoted string uses.
func unescapeIndented(s string) string {
	if !strings.Contains(s, "''") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' || i+2 >= len(s) || s[i+1] != '\'' {
			b.WriteByte(s[i])
			continue
		}
		switch c := s[i+2]; c {
		case '\'':
			b.WriteString("''")
			i += 2
		case '$':
			b.WriteByte('$')
			i += 2
		case '\\':
			if i+3 < len(s) {
				b.WriteByte(escapeByte(s[i+3]))
				i += 3
			} else {
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// stringPart is one piece of a string literal: either literal text, or the
// value an interpolation evaluated to.
type stringPart struct {
	text   string
	interp *NixString
}

// stripIndentation removes the common indentation of an indented string, as
// well as its leading newline. Lines that hold nothing but whitespace do not
// count towards the common indentation, so the line the closing quotes sit on
// never forces it to zero.
func stripIndentation(parts []stringPart) []stringPart {
	indent := commonIndentation(parts)
	out := make([]stringPart, 0, len(parts))
	atLineStart := true
	skipped := 0
	for _, part := range parts {
		if part.interp != nil {
			out = append(out, part)
			atLineStart = false
			continue
		}
		var b strings.Builder
		b.Grow(len(part.text))
		for i := 0; i < len(part.text); i++ {
			c := part.text[i]
			switch {
			case c == '\n':
				b.WriteByte(c)
				atLineStart, skipped = true, 0
			case atLineStart && isIndent(c) && skipped < indent:
				skipped++
			default:
				b.WriteByte(c)
				atLineStart = false
			}
		}
		out = append(out, stringPart{text: b.String()})
	}
	// An indented string conventionally opens on its own line; that newline is
	// part of the layout, not of the value.
	for i, part := range out {
		if part.interp != nil {
			break
		}
		if part.text != "" {
			if part.text[0] == '\n' {
				out[i].text = part.text[1:]
			}
			break
		}
	}
	return out
}

// commonIndentation is the smallest indentation of any line with content.
func commonIndentation(parts []stringPart) int {
	const none = int(^uint(0) >> 1)
	indent, cur := none, 0
	atLineStart := true
	note := func() {
		if atLineStart && cur < indent {
			indent = cur
		}
	}
	for _, part := range parts {
		if part.interp != nil {
			// An interpolation is content, so the whitespace before it on this
			// line is indentation.
			note()
			atLineStart = false
			continue
		}
		for i := 0; i < len(part.text); i++ {
			switch c := part.text[i]; {
			case c == '\n':
				atLineStart, cur = true, 0
			case atLineStart && isIndent(c):
				cur++
			case atLineStart:
				note()
				atLineStart = false
			}
		}
	}
	if indent == none {
		return 0
	}
	return indent
}

func isIndent(c byte) bool { return c == ' ' || c == '\t' }
