package soy

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/robfig/soy/parse"
)

// value represents intermediate values in expression computations
type value struct {
	valueType
	intValue   int64
	floatValue float64
	boolValue  bool
	strValue   string
}

func nullValue() value {
	return value{valueType: null}
}

func intValue(val int64) value {
	return value{valueType: integer, intValue: val}
}

func floatValue(val float64) value {
	return value{valueType: float, floatValue: val}
}

func boolValue(val bool) value {
	return value{valueType: boolean, boolValue: val}
}

func strValue(val string) value {
	return value{valueType: str, strValue: val}
}

type valueType int

const (
	invalid valueType = iota
	null
	integer
	float
	boolean
	str
)

var none = value{}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl *parse.TemplateNode
	wr   io.Writer
	node parse.Node // current node, for errors
	val  value      // temp value for expression being computed
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value reflect.Value
}

var zero reflect.Value

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// doublePercent returns the string with %'s replaced by %%, if necessary,
// so it can be used safely inside a Printf format string.
func doublePercent(str string) string {
	if strings.Contains(str, "%") {
		str = strings.Replace(str, "%", "%%", -1)
	}
	return str
}

// errorf formats the error and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	name := doublePercent(s.tmpl.Name)
	format = fmt.Sprintf("template: %s: %s", name, format)
	panic(fmt.Errorf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = err
		default:
			panic(e)
		}
	}
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Template) Execute(wr io.Writer, data interface{}) (err error) {
	if t.node == nil {
		return errors.New("no template found")
	}
	defer errRecover(&err)
	value := reflect.ValueOf(data)
	state := &state{
		tmpl: t.node,
		wr:   wr,
	}
	state.walk(value, t.node)
	return
}

// Walk functions step through the major pieces of the template structure,
// generating output as they go.
func (s *state) walk(dot reflect.Value, node parse.Node) {
	s.val = none
	s.at(node)
	switch node := node.(type) {
	case *parse.TemplateNode:
		s.walk(dot, node.Body)
	case *parse.PrintNode:
		s.walk(dot, node.Arg)
		if _, err := s.wr.Write([]byte(s.toString(s.val))); err != nil {
			s.errorf("%s", err)
		}
	case *parse.DataRefNode:
		s.val = s.evalDataRef(dot, node)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(dot, node)
		}
	case *parse.RawTextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.errorf("%s", err)
		}

	case *parse.NullNode:
		s.val = nullValue()
	case *parse.StringNode:
		s.val = strValue(node.Value)
	case *parse.IntNode:
		s.val = intValue(node.Value)
	case *parse.FloatNode:
		s.val = floatValue(node.Value)
	case *parse.BoolNode:
		s.val = boolValue(node.True)

	case *parse.AddNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		switch {
		case arg1.valueType == integer && arg2.valueType == integer:
			s.val = intValue(arg1.intValue + arg2.intValue)
		case arg1.valueType == str || arg2.valueType == str:
			s.val = strValue(s.toString(arg1) + s.toString(arg2))
		default:
			s.val = floatValue(s.toFloat(arg1) + s.toFloat(arg2))
		}
	case *parse.DivNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		s.val = floatValue(s.toFloat(arg1) / s.toFloat(arg2))
	case *parse.MulNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		switch {
		case arg1.valueType == integer && arg2.valueType == integer:
			s.val = intValue(arg1.intValue * arg2.intValue)
		default:
			s.val = floatValue(s.toFloat(arg1) * s.toFloat(arg2))
		}
	case *parse.ModNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer)
		s.val = intValue(arg1.intValue % arg2.intValue)

	case *parse.EqNode:
		s.val = s.evalEq(dot, node.Arg1, node.Arg2)
	case *parse.NotEqNode:
		s.val = s.evalEq(dot, node.Arg1, node.Arg2)
		s.val.boolValue = !s.val.boolValue
	case *parse.LtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		s.val = boolValue(s.toFloat(arg1) < s.toFloat(arg2))
	case *parse.LteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		s.val = boolValue(s.toFloat(arg1) <= s.toFloat(arg2))
	case *parse.GtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		s.val = boolValue(s.toFloat(arg1) > s.toFloat(arg2))
	case *parse.GteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, integer, float)
		s.val = boolValue(s.toFloat(arg1) >= s.toFloat(arg2))

	case *parse.NotNode:
		s.val = boolValue(!truthiness(s.eval1(dot, node.Arg)))
	case *parse.AndNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = boolValue(truthiness(arg1) && truthiness(arg2))
	case *parse.OrNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = boolValue(truthiness(arg1) || truthiness(arg2))
	case *parse.ElvisNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		if truthiness(arg1) {
			s.val = arg1
		} else {
			s.val = arg2
		}
	case *parse.TernNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		var arg3 = s.eval1(dot, node.Arg3)
		if truthiness(arg1) {
			s.val = arg2
		} else {
			s.val = arg3
		}
	default:
		s.errorf("unknown node: %T", node)
	}
}

func (s *state) toString(val value) string {
	switch val.valueType {
	case integer:
		return strconv.FormatInt(val.intValue, 10)
	case float:
		return strconv.FormatFloat(val.floatValue, 'g', -1, 64)
	case boolean:
		return fmt.Sprint(val.boolValue)
	case str:
		return val.strValue
	case null:
		return "null"
	default:
		s.errorf("no value")
		return ""
	}
}

func (s *state) evalDataRef(dot reflect.Value, node *parse.DataRefNode) value {
	// TODO: Evaluate data refs
	val := dot.MapIndex(reflect.ValueOf(node.Key)).Elem()
	if !val.IsValid() {
		s.errorf("variable %s is not valid", node.Key)
	}
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return intValue(val.Int())
	case reflect.Float32, reflect.Float64:
		return floatValue(val.Float())
	case reflect.Bool:
		return boolValue(val.Bool())
	case reflect.String:
		return strValue(val.String())
	default:
		s.errorf("unexpected type for variable: %v", val.Kind())
	}
	return none
}

func (s *state) evalEq(dot reflect.Value, arg1, arg2 parse.Node) value {
	var val1, val2 = s.eval2(dot, arg1, arg2)
	if val1.valueType != val2.valueType {
		s.errorf("can only compare same types")
	}
	switch val1.valueType {
	case integer:
		return boolValue(val1.intValue == val2.intValue)
	case float:
		return boolValue(val1.floatValue == val2.floatValue)
	case boolean:
		return boolValue(val1.boolValue == val2.boolValue)
	case str:
		return boolValue(val1.strValue == val2.strValue)
	default:
		return boolValue(false)
	}
}

func truthiness(val value) bool {
	switch val.valueType {
	case integer:
		return val.intValue != 0
	case float:
		return val.floatValue != 0.0
	case boolean:
		return val.boolValue
	case str:
		return val.strValue != ""
	case null:
		return false
	}
	panic("invalid value")
}

// toFloat returns a float for given int or float value
func (s *state) toFloat(val value) float64 {
	switch val.valueType {
	case float:
		return val.floatValue
	case integer:
		return float64(val.intValue)
	}
	s.errorf("expected int or float, got: %#v", val)
	return 0
}

// eval2 is a helper for binary ops.  it evaluates the two given nodes, with
// optional type restrictions on the resulting values.
func (s *state) eval2(dot reflect.Value, n1, n2 parse.Node, resultTypes ...valueType) (value, value) {
	return s.eval1(dot, n1, resultTypes...), s.eval1(dot, n2, resultTypes...)
}

func (s *state) eval1(dot reflect.Value, n parse.Node, resultTypes ...valueType) value {
	s.walk(dot, n)
	if s.val.valueType == invalid {
		s.errorf("invalid value found: %T", n)
	}
	if len(resultTypes) == 0 {
		return s.val
	}
	for _, resultType := range resultTypes {
		if resultType == s.val.valueType {
			return s.val
		}
	}
	s.errorf("invalid value found")
	return none
}
