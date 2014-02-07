package tofu

import (
	"io/ioutil"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
)

// EvalExpr evaluates the given expression node and returns the result.
func EvalExpr(node parse.Node) (val data.Value, err error) {
	state := &state{
		wr: ioutil.Discard,
	}
	defer state.errRecover(&err)
	state.walk(node)
	return state.val, nil
}
