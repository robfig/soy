package parse

import "unicode/utf8"

type rawtextlexer struct {
	str     string
	pos     int
	lastpos int
}

func (l *rawtextlexer) eof() bool {
	return l.pos >= len(l.str)
}
func (l *rawtextlexer) next() rune {
	l.lastpos = l.pos
	var r, width = utf8.DecodeRuneInString(l.str[l.pos:])
	l.pos += width
	return r
}

// rawtext processes the raw text found in templates:
// - trim leading/trailing whitespace if either:
//  a. the whitespace includes a newline, or
//  b. the caller tells us the surrounding content is a tight joiner by trimBefore/After
// - trim leading and trailing whitespace on each internal line
// - join lines with no space if '<' or '>' are on either side, else with 1 space.
func rawtext(s string, trimBefore, trimAfter bool) []byte {
	var lex = rawtextlexer{s, 0, 0}
	var (
		spaces         = 0
		seenNewline    = trimBefore
		lastChar       rune
		charBeforeTrim rune
		result         = make([]byte, len(s))
		resultLen      = 0
	)
	if trimBefore {
		spaces = 1
	}

	for {
		if lex.eof() {
			// if we haven't seen a newline, add all the space we've been trimming.
			if !seenNewline && spaces > 0 && !trimAfter {
				for i := lex.pos - spaces; i < lex.pos; i++ {
					result[resultLen] = s[i]
					resultLen++
				}
			}
			return result[:resultLen]
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
				for i := lex.lastpos - spaces; i < lex.lastpos; i++ {
					result[resultLen] = s[i]
					resultLen++
				}
			case seenNewline && !isTightJoiner(charBeforeTrim) && !isTightJoiner(r):
				result[resultLen] = ' '
				resultLen++
			default:
				// ignore the space
			}
			spaces = 0
		}

		// begin to trim
		seenNewline = isEndOfLine(r)
		if isSpace(r) || seenNewline {
			spaces = 1
			charBeforeTrim = lastChar
			continue
		}

		// non-space characters are added verbatim.
		for i := lex.lastpos; i < lex.pos; i++ {
			result[resultLen] = lex.str[i]
			resultLen++
		}
		lastChar = r
	}
}

func isTightJoiner(r rune) bool {
	switch r {
	case 0, '<', '>':
		return true
	}
	return false
}
