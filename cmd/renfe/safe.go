package main

import "strings"

// Everything this CLI prints about a journey — station names, fare titles,
// Renfe's own warnings, the body of an HTTP error — is text a remote server
// chose. A terminal treats some of those bytes as commands, not characters:
// CSI (ESC [) repaints and clears the screen, OSC (ESC ]) sets the window
// title and, on terminals that honour OSC 52, writes to the clipboard, and a
// bare CR returns the cursor to the start of the line so the text after it
// overwrites what a user just read. A newline forges a whole extra row in a
// listing an agent is parsing line by line.
//
// Renfe is not hostile, but its replies are remote input and RENFE_BASE_URL
// points wherever it is told to, so none of it reaches a terminal unfiltered.

// safeField is for a value that belongs inside one line of output: every
// control character goes, newlines and tabs included, so a station name cannot
// invent a row.
func safeField(s string) string { return strip(s, false) }

// safeText is for a whole message, which may legitimately span lines — the
// CLI's own multi-line errors do. Line breaks survive; the escape and
// carriage-return primitives do not.
func safeText(s string) string { return strip(s, true) }

// strip removes C0 controls, DEL, and the C1 range (0x80–0x9f, which some
// terminals decode as escapes in their own right). Everything printable,
// accents and box characters included, passes through untouched.
func strip(s string, keepLineBreaks bool) string {
	if !needsStripping(s, keepLineBreaks) {
		return s // the overwhelmingly common case: nothing to do, nothing copied
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case keepLineBreaks && (r == '\n' || r == '\t'):
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			// dropped
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func needsStripping(s string, keepLineBreaks bool) bool {
	for _, r := range s {
		if keepLineBreaks && (r == '\n' || r == '\t') {
			continue
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) {
			return true
		}
	}
	return false
}

// safeValue strips control characters from every string in a decoded payload.
// It is used on the structured output: the TOON encoder refuses a control
// character outright, so one in a station name would fail the whole command
// rather than print it.
func safeValue(v any) any {
	switch x := v.(type) {
	case string:
		return strip(x, false)
	case []any:
		for i, e := range x {
			x[i] = safeValue(e)
		}
		return x
	case map[string]any:
		for k, e := range x {
			x[k] = safeValue(e)
		}
		return x
	}
	return v
}
