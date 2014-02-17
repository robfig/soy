package tofu

import (
	"io/ioutil"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
)

// EvalExpr evaluates the given expression node and returns the result.
func EvalExpr(node ast.Node) (val data.Value, err error) {
	state := &state{
		wr: ioutil.Discard,
	}
	defer state.errRecover(&err)
	state.walk(node)
	return state.val, nil
}
