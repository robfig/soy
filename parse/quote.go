package parse

import (
	"errors"
	"strconv"
	"unicode/utf8"
)

var unescapes = map[rune]rune{
	'\\': '\\',
	'\'': '\'',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
	'b':  '\b',
	'f':  '\f',
}

var escapes = make(map[rune]rune)

func init() {
	for k, v := range unescapes {
		escapes[v] = k
	}
}

// quoteString quotes the given string with single quotes, according to the Soy
// spec for string literals.
func quoteString(s string) string {
	var q = make([]rune, 1, len(s)+10)
	q[0] = '\''
	for _, ch := range s {
		if seq, ok := escapes[ch]; ok {
			q = append(q, '\\', seq)
			continue
		}
		q = append(q, ch)
	}
	return string(append(q, '\''))
}

// unquoteString takes a quoted Soy string (including the surrounding quotes)
// and returns the unquoted string, along with any error encountered.
func unquoteString(s string) (string, error) {
	n := len(s)
	if n < 2 {
		return "", errors.New("too short a string")
	}

	if '\'' != s[0] || '\'' != s[n-1] {
		return "", errors.New("string not surrounded by quotes")
	}

	s = s[1 : n-1]
	if !contains(s, '\\') && !contains(s, '\'') {
		return s, nil
	}

	var escaping = false
	var result = make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size

		if escaping {
			if r == 'u' {
				if i+4 > len(s) {
					return "", errors.New("error scanning unicode escape, expect \\uNNNN")
				}
				num, err := strconv.ParseInt(s[i:i+4], 16, 0)
				if err != nil {
					return "", err
				}
				r = rune(num)
				i += 4
			} else {
				replacement, ok := unescapes[r]
				if !ok {
					return "", errors.New("unrecognized escape code: \\" + s[i-1:i])
				}
				r = rune(replacement)
			}
		}

		escaping = ((r == '\\') && !escaping)
		if !escaping {
			result = append(result, r)
		}
	}
	return string(result), nil
}

func contains(s string, c byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return true
		}
	}
	return false
}
