package soy

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/tofu"
)

// ParseGlobals parses the given input, expecting the form:
//  <global_name> = <primitive_data>
//
// Furthermore:
//  - Empty lines and lines beginning with '//' are ignored.
//  - <primitive_data> must be a valid template expression literal for a primitive
//   type (null, boolean, integer, float, or string)
func ParseGlobals(input io.Reader) (data.Map, error) {
	var globals = make(data.Map)
	var scanner = bufio.NewScanner(input)
	for scanner.Scan() {
		var line = scanner.Text()
		if len(line) == 0 || strings.HasPrefix(line, "//") {
			continue
		}
		var eq = strings.Index(line, "=")
		if eq == -1 {
			return nil, fmt.Errorf("no equals on line: %q", line)
		}
		var (
			name = strings.TrimSpace(line[:eq])
			expr = strings.TrimSpace(line[eq+1:])
		)
		var node, err = parse.Expr(expr)
		if err != nil {
			return nil, err
		}
		exprValue, err := tofu.EvalExpr(node)
		if err != nil {
			return nil, err
		}
		globals[name] = exprValue
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return globals, nil
}
