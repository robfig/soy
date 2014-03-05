package tofu

import (
	"io/ioutil"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
)

// EvalExpr evaluates the given expression node and returns the result.  The
// given node must be a simple Soy expression, such as what may appear inside a
// print tag.
//
// This is useful for evaluating Globals, or anything returned from parse.Expr.
func EvalExpr(node ast.Node) (val data.Value, err error) {
	state := &state{
		wr: ioutil.Discard,
	}
	defer state.errRecover(&err)
	state.walk(node)
	return state.val, nil
}
