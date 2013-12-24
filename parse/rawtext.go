package parse

import (
	"strings"
	"unicode/utf8"
)

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
// - strip comments (// to end of line)
// - trim leading/trailing whitespace if it includes a newline
// - trim leading and trailing whitespace on each internal line
// - join lines with no space if '<' or '>' are on either side, else with 1 space.
func rawtext(s string) []byte {
	var lex = rawtextlexer{s, 0, 0, 0}
	var (
		spaces         = 0
		seenNewline    = false
		seenComment    = false
		lastChar       rune
		charBeforeTrim rune
		result         = make([]byte, 0, len(s))
	)

TOP:
	for {
		if lex.eof() {
			// if we haven't seen a newline, add all the space we've been trimming.
			if !seenNewline && spaces > 0 {
				if !seenComment {
					result = append(result, s[lex.pos-spaces:lex.pos]...)
				} else {
					result = append(result, strings.Repeat(" ", spaces)...)
				}
			}
			return result
		}
		var r = lex.next()

		// '//' comment removal
		if (spaces > 0 || lastChar == 0) && r == '/' {
			if lex.next() == '/' {
				seenComment = true
				for {
					r = lex.next()
					if lex.eof() {
						return result
					}
					if isEndOfLine(r) {
						break
					}
				}
			}
			lex.backup()
		}

		// '/*' comment removal
		if r == '/' {
			if lex.next() == '*' {
				seenComment = true
				var asterisk = false
				for {
					r = lex.next()
					switch {
					case lex.eof():
						return result
					case r == '*':
						asterisk = true
					case r == '/' && asterisk:
						continue TOP
					default:
						asterisk = false
					}
				}
			}
			lex.backup()
		}

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
				// if there was no comment, we can copy in the bytes directly.
				if !seenComment {
					result = append(result, s[lex.lastpos-spaces:lex.lastpos]...)
				} else {
					// if there was a comment in between, then we have to fake it.
					// note: this may incorrectly transform \t into ' ' for the string '\t/**/a'
					// but, the case seems rare enough that it's worth having a second buffer to solve it.
					result = append(result, strings.Repeat(" ", spaces)...)
				}
			case seenNewline && !isTightJoiner(charBeforeTrim) && !isTightJoiner(r):
				result = append(result, ' ')
			default:
				// ignore the space
			}
			spaces = 0
			seenNewline = false
			seenComment = false
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
