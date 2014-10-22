package soyhtml

import (
	"testing"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
)

func TestEvalExpr(t *testing.T) {
	var tests = []struct {
		input    string
		expected interface{}
	}{
		{"0", 0},
		{"1+1", 2},
		{"'abc'", "abc"},
	}

	for _, test := range tests {
		var tree, err = parse.SoyFile("", "{"+test.input+"}")
		if err != nil {
			t.Error(err)
			return
		}

		actual, err := EvalExpr(tree)
		if err != nil {
			t.Error(err)
			continue
		}
		if actual != data.New(test.expected) {
			t.Errorf("EvalExpr(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}
