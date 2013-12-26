package parse

import "unicode/utf8"

type rawtextlexer struct {
	str      string
	pos      int
	lastpos  int
	lastpos2 int
}

func (l *rawtextlexer) eof() bool {
	return l.pos >= len(l.str)
}
func (l *rawtextlexer) next() rune {
	l.lastpos2 = l.lastpos
	l.lastpos = l.pos
	var r, width = utf8.DecodeRuneInString(l.str[l.pos:])
	l.pos += width
	return r
}
func (l *rawtextlexer) backup() {
	l.pos = l.lastpos
	l.lastpos = l.lastpos2
	l.lastpos2 = 0
}
func (l *rawtextlexer) emitRune(result []byte) []byte {
	return append(result, []byte(l.str[l.lastpos:l.pos])...)
}

// rawtext processes the raw text found in templates:
// - trim leading/trailing whitespace if either:
//  a. the whitespace includes a newline, or
//  b. the caller tells us the surrounding content is a tight joiner  by start/endTightJoin
// - trim leading and trailing whitespace on each internal line
// - join lines with no space if '<' or '>' are on either side, else with 1 space.
func rawtext(s string, trimBefore, trimAfter bool) []byte {
	var lex = rawtextlexer{s, 0, 0, 0}
	var (
		spaces         = 0
		seenNewline    = trimBefore
		lastChar       rune
		charBeforeTrim rune
		result         = make([]byte, 0, len(s))
	)
	if trimBefore {
		spaces = 1
	}

	for {
		if lex.eof() {
			// if we haven't seen a newline, add all the space we've been trimming.
			if !seenNewline && spaces > 0 && !trimAfter {
				result = append(result, s[lex.pos-spaces:lex.pos]...)
			}
			return result
		}
		var r = lex.next()

		// join lines
		if spaces > 0 {
			// more space, keep going
			if isSpace(r) {
				spaces++
				continue
			}
			if isEndOfLine(r) {
				spaces++
				seenNewline = true
				continue
			}

			// done with scanning a set of space.  actions:
			// - add the run of space to the result if we haven't seen a newline.
			// - add one space if the character before and after the newline are not tight joiners.
			// - else, ignore the space.
			switch {
			case !seenNewline:
				result = append(result, s[lex.lastpos-spaces:lex.lastpos]...)
			case seenNewline && !isTightJoiner(charBeforeTrim) && !isTightJoiner(r):
				result = append(result, ' ')
			default:
				// ignore the space
			}
			spaces = 0
			seenNewline = false
		}

		// begin to trim
		seenNewline = isEndOfLine(r)
		if isSpace(r) || seenNewline {
			spaces = 1
			charBeforeTrim = lastChar
			continue
		}

		// non-space characters are added verbatim.
		result = lex.emitRune(result)
		lastChar = r
	}
	return result
}

func isTightJoiner(r rune) bool {
	switch r {
	case 0, '<', '>':
		return true
	}
	return false
}
