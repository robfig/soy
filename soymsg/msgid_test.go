package soymsg

import (
	"testing"

	"github.com/robfig/soy/ast"
)

func TestCalcMsgID(t *testing.T) {
	type test struct {
		msg *ast.MsgNode
		id  uint64
	}

	// test data taken from closure-templates/exaples/examples_extract.xlf
	var tests = []test{
		{msg("noun", "The word 'Archive' used as a noun, i.e. an information store.", txt("Archive")),
			7224011416745566687},
		{msg("verb", "The word 'Archive' used as a verb, i.e. to store information.", txt("Archive")),
			4826315192146469447},
		{msg("", "", txt("A trip was taken.")),
			3329840836245051515},
	}

	for _, test := range tests {
		actual := CalcID(test.msg)
		if actual != test.id {
			t.Errorf("(actual) %v != %v (expected)", actual, test.id)
		}
	}
}

func msg(meaning, desc string, body ...ast.Node) *ast.MsgNode {
	return &ast.MsgNode{0, meaning, desc, body}
}

func txt(str string) *ast.RawTextNode {
	return &ast.RawTextNode{0, []byte(str)}
}
