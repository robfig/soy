package parse

import "testing"

func TestQuote(t *testing.T) {
	var tests = []struct{ input, output string }{
		{"", `''`},
		{"a", `'a'`},
		{"\n", `'\n'`},
		{"\u2222", "'\u2222'"}, // (doesn't turn it back into escape sequence)
		{"Aa`! \n \r \t \\ ' \"", "'Aa`! \\n \\r \\t \\\\ \\' \"'"},
		{"\u2222 \uEEEE \u9EC4 \u607A", "'\u2222 \uEEEE \u9EC4 \u607A'"},
	}
	for _, test := range tests {
		if quoteString(test.input) != test.output {
			t.Errorf("%v => %v, expected %v", test.input, quoteString(test.input), test.output)
		}
	}
}

func TestUnquote(t *testing.T) {
	var tests = []struct{ input, output string }{
		{`''`, ""},
		{`'a'`, "a"},
		{`'\n'`, "\n"},
		{`'\u2222'`, "\u2222"},
		{`'\\'`, "\\"},
	}
	for _, test := range tests {
		actual, err := unquoteString(test.input)
		if err != nil {
			t.Error(err)
			continue
		}
		if actual != test.output {
			t.Errorf("%v => %v, expected %v", test.input, actual, test.output)
		}
	}
}
