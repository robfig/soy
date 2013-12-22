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

type scope []map[string]interface{} // a stack of variable scopes

// push creates a new scope
func (s scope) push() {
	s = append(s, make(map[string]interface{}))
}

// pop discards the last scope pushed.
func (s scope) pop() {
	s = s[:len(s)-1]
}

// set adds a new binding to the deepest scope
func (s scope) set(k string, v interface{}) {
	s[len(s)-1][k] = v
}

// lookup checks the variable scopes, outer to inner, for the given key
func (s scope) lookup(k string) interface{} {
	for i := range s {
		if v, ok := s[i][k]; ok {
			return v
		}
	}
	return nil
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl    *parse.TemplateNode
	wr      io.Writer
	node    parse.Node // current node, for errors
	val     value      // temp value for expression being computed
	context scope      // variable scope
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
	format = fmt.Sprintf("template: %s:%s %s", name, s.node, format)
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
func (t Template) Execute(wr io.Writer, data map[string]interface{}) (err error) {
	if t.node == nil {
		return errors.New("no template found")
	}
	defer errRecover(&err)
	value := reflect.ValueOf(data)
	state := &state{
		tmpl:    t.node,
		wr:      wr,
		context: []map[string]interface{}{data},
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

	case *parse.IfNode:
		fmt.Println("if..")
		for _, cond := range node.Conds {
			fmt.Println("checking cond", cond)
			if cond.Cond == nil || truthiness(s.eval1(dot, cond.Cond)) {
				fmt.Println("walking if condition!")
				s.walk(dot, cond.Body)
				break
			}
		}
	case *parse.ForNode:
		var list = s.eval1(dot, node.List)
		if list.valueType != listType {
			s.errorf("expected list type to iterate")
		}
		if len(list.listValue) == 0 && node.IfEmpty != nil {
			s.walk(dot, node.IfEmpty)
			break
		}
		s.context.push()
		for _, item := range list.listValue {
			s.context.set(node.Var, item)
			s.walk(dot, node.Body)
		}
		s.context.pop()
	//case *parse.SwitchNode:

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
		case arg1.valueType == intType && arg2.valueType == intType:
			s.val = intValue(arg1.intValue + arg2.intValue)
		case arg1.valueType == stringType || arg2.valueType == stringType:
			s.val = strValue(s.toString(arg1) + s.toString(arg2))
		default:
			s.val = floatValue(s.toFloat(arg1) + s.toFloat(arg2))
		}
	case *parse.DivNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType)
		s.val = floatValue(s.toFloat(arg1) / s.toFloat(arg2))
	case *parse.MulNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType)
		switch {
		case arg1.valueType == intType && arg2.valueType == intType:
			s.val = intValue(arg1.intValue * arg2.intValue)
		default:
			s.val = floatValue(s.toFloat(arg1) * s.toFloat(arg2))
		}
	case *parse.ModNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType)
		s.val = intValue(arg1.intValue % arg2.intValue)

	case *parse.EqNode:
		s.val = s.evalEq(dot, node.Arg1, node.Arg2)
	case *parse.NotEqNode:
		s.val = s.evalEq(dot, node.Arg1, node.Arg2)
		s.val.boolValue = !s.val.boolValue
	case *parse.LtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType, nullType)
		s.val = boolValue(s.toFloat(arg1) < s.toFloat(arg2))
	case *parse.LteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType, nullType)
		s.val = boolValue(s.toFloat(arg1) <= s.toFloat(arg2))
	case *parse.GtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType, nullType)
		s.val = boolValue(s.toFloat(arg1) > s.toFloat(arg2))
	case *parse.GteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2, intType, floatType, nullType)
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
	case intType:
		return strconv.FormatInt(val.intValue, 10)
	case floatType:
		return strconv.FormatFloat(val.floatValue, 'g', -1, 64)
	case boolType:
		return fmt.Sprint(val.boolValue)
	case stringType:
		return val.strValue
	case nullType:
		return ""
	case listType:
		return fmt.Sprint(val.listValue)
	case mapType:
		return fmt.Sprint(val.mapValue)
	default:
		s.errorf("no value")
		return ""
	}
}

func (s *state) evalDataRef(dot reflect.Value, node *parse.DataRefNode) value {
	fmt.Printf("evalDataRef: %s on %v\n", node.Key, dot.Interface())
	val := reflect.ValueOf(s.context.lookup(node.Key))
	fmt.Println("val:", val)
	if !val.IsValid() {
		return nullValue()
	}

	// handle the accesses
	for _, accessNode := range node.Access {
		fmt.Println("access:", accessNode)
		switch val.Kind() {
		case reflect.Slice:
			indexNode, ok := accessNode.(*parse.DataRefIndexNode)
			if !ok {
				s.errorf("expecting an index node, got %T", accessNode)
			}
			if !val.IsNil() {
				val = val.Index(indexNode.Index)
			} else if indexNode.NullSafe {
				return nullValue()
			} else {
				s.errorf("null slice")
			}
			fmt.Println("val =>", val)
			continue
		case reflect.Map:
			keyNode, ok := accessNode.(*parse.DataRefKeyNode)
			if !ok {
				s.errorf("expecting a key node, got %T", accessNode)
			}
			if !val.IsNil() {
				val = val.MapIndex(reflect.ValueOf(keyNode.Key))
			} else if keyNode.NullSafe {
				return nullValue()
			} else {
				s.errorf("null map")
			}
			fmt.Println("val =>", val)
			continue
		default:
			s.errorf("expected a slice or map, got: %T", val.Interface())
		}
	}

	// handle the terminal value
	fmt.Println("terminal:", val)
	if val.Kind() == reflect.Interface {
		val = val.Elem() // drill through the interface
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
	case intType:
		return boolValue(val1.intValue == val2.intValue)
	case floatType:
		return boolValue(val1.floatValue == val2.floatValue)
	case boolType:
		return boolValue(val1.boolValue == val2.boolValue)
	case stringType:
		return boolValue(val1.strValue == val2.strValue)
	default:
		return boolValue(false)
	}
}

func truthiness(val value) bool {
	switch val.valueType {
	case intType:
		return val.intValue != 0
	case floatType:
		return val.floatValue != 0.0
	case boolType:
		return val.boolValue
	case stringType:
		return val.strValue != ""
	case nullType:
		return false
	}
	panic("invalid value")
}

// toFloat returns a float for given int or float value
func (s *state) toFloat(val value) float64 {
	switch val.valueType {
	case floatType:
		return val.floatValue
	case intType:
		return float64(val.intValue)
	case nullType:
		return float64(0)
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

// expression values

// value represents intermediate values in expression computations
type value struct {
	valueType
	intValue   int64
	floatValue float64
	boolValue  bool
	strValue   string
	listValue  []interface{}
	mapValue   map[string]interface{}
}

func nullValue() value {
	return value{valueType: nullType}
}

func intValue(val int64) value {
	return value{valueType: intType, intValue: val}
}

func floatValue(val float64) value {
	return value{valueType: floatType, floatValue: val}
}

func boolValue(val bool) value {
	return value{valueType: boolType, boolValue: val}
}

func strValue(val string) value {
	return value{valueType: stringType, strValue: val}
}

func listValue(val []interface{}) value {
	return value{valueType: listType, listValue: val}
}

func mapValue(val map[string]interface{}) value {
	return value{valueType: mapType, mapValue: val}
}

type valueType int

const (
	invalid valueType = iota
	nullType
	intType
	floatType
	boolType
	stringType
	listType
	mapType
)

var none = value{}
