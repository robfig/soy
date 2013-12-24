package soy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
	"runtime"

	"github.com/robfig/soy/parse"
)

var Logger *log.Logger

type scope []map[string]interface{} // a stack of variable scopes

// push creates a new scope
func (s *scope) push() {
	*s = append(*s, make(map[string]interface{}))
}

// pop discards the last scope pushed.
func (s *scope) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *scope) augment(m map[string]interface{}) {
	*s = append(*s, m)
}

// set adds a new binding to the deepest scope
func (s scope) set(k string, v reflect.Value) {
	s[len(s)-1][k] = v.Interface()
}

// lookup checks the variable scopes, deepest out, for the given key
func (s scope) lookup(k string) reflect.Value {
	for i := range s {
		if v, ok := s[len(s)-i-1][k]; ok {
			vv := val(v)
			for vv.Kind() == reflect.Interface {
				vv = vv.Elem()
			}
			return vv
		}
	}
	return undefinedValue
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	namespace string
	tmpl      *parse.TemplateNode
	wr        io.Writer
	node      parse.Node    // current node, for errors
	bundle    Tofu          // the entire bundle of templates
	val       reflect.Value // temp value for expression being computed
	context   scope         // variable scope
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value reflect.Value
}

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// errorf formats the error and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("template: %s:%s %s", s.tmpl.Name, s.node, format)
	panic(fmt.Errorf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func (s *state) errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = fmt.Errorf("%#v: %v", s.node, err)
			//			*errp = err
		default:
			*errp = fmt.Errorf("%#v: %v", s.node, e)
		}
	}
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Template) Execute(wr io.Writer, data map[string]interface{}) (err error) {
	if t.node == nil {
		return errors.New("no template found")
	}
	state := &state{
		tmpl:      t.node,
		bundle:    t.tofu,
		namespace: t.namespace,
		wr:        wr,
		context:   []map[string]interface{}{data},
	}
	defer state.errRecover(&err)
	state.walk(reflect.ValueOf(nil), t.node)
	return
}

// walk recursively goes through each node and executes the indicated logic and
// writes the output
func (s *state) walk(dot reflect.Value, node parse.Node) {
	s.val = undefinedValue
	s.at(node)
	switch node := node.(type) {
	case *parse.TemplateNode:
		s.walk(dot, node.Body)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(dot, node)
		}

		// Output nodes
	case *parse.PrintNode:
		s.walk(dot, node.Arg)
		if _, err := s.wr.Write([]byte(toString(s.val))); err != nil {
			s.errorf("%s", err)
		}
	case *parse.RawTextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.errorf("%s", err)
		}
	case *parse.MsgNode:
		s.walk(dot, node.Body)
	case *parse.CssNode:
		var prefix = ""
		if node.Expr != nil {
			prefix = toString(s.eval(dot, node.Expr)) + "-"
		}
		if _, err := s.wr.Write([]byte(prefix + node.Suffix)); err != nil {
			s.errorf("%s", err)
		}
	case *parse.DebuggerNode:
		// nothing to do
	case *parse.LogNode:
		Logger.Print(string(s.renderBlock(dot, node.Body)))

		// Control flow
	case *parse.IfNode:
		for _, cond := range node.Conds {
			if cond.Cond == nil || truthiness(s.eval(dot, cond.Cond)) {
				s.walk(dot, cond.Body)
				break
			}
		}
	case *parse.ForNode:
		var list = s.eval(dot, node.List)
		if list.Kind() != reflect.Slice {
			s.errorf("expected list type to iterate, got %v", list.Kind())
		}
		if list.Len() == 0 && node.IfEmpty != nil {
			s.walk(dot, node.IfEmpty)
			break
		}
		s.context.push()
		for i := 0; i < list.Len(); i++ {
			s.context.set(node.Var, list.Index(i))
			s.context.set(node.Var+"__index", val(i))
			s.context.set(node.Var+"__lastIndex", val(list.Len()-1))
			s.walk(dot, node.Body)
		}
		s.context.pop()
	case *parse.SwitchNode:
		var switchValue = s.eval(dot, node.Value)
		for _, caseNode := range node.Cases {
			for _, caseValueNode := range caseNode.Values {
				if equals(switchValue, s.eval(dot, caseValueNode)) {
					s.walk(dot, caseNode.Body)
					return
				}
			}
			if len(caseNode.Values) == 0 { // default/last case
				s.walk(dot, caseNode.Body)
				return
			}
		}
	case *parse.CallNode:
		s.evalCall(dot, node)

		// Values
	case *parse.NullNode:
		s.val = nullValue
	case *parse.StringNode:
		s.val = val(node.Value)
	case *parse.IntNode:
		s.val = val(node.Value)
	case *parse.FloatNode:
		s.val = val(node.Value)
	case *parse.BoolNode:
		s.val = val(node.True)
	case *parse.ListLiteralNode:
		var items = make([]interface{}, len(node.Items))
		for i, item := range node.Items {
			items[i] = s.eval(dot, item)
		}
		s.val = val(items)
	case *parse.MapLiteralNode:
		var items = make(map[string]interface{}, len(node.Items))
		for k, v := range node.Items {
			items[k] = s.eval(dot, v)
		}
		s.val = val(items)
	case *parse.FunctionNode:
		s.val = s.evalFunc(node)
	case *parse.DataRefNode:
		s.val = s.evalDataRef(dot, node)

		// Arithmetic operators
	case *parse.NegateNode:
		var arg = s.eval(dot, node.Arg)
		if isInt(arg) {
			s.val = val(-arg.Int())
		} else {
			s.val = val(-toFloat(arg))
		}
	case *parse.AddNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = val(arg1.Int() + arg2.Int())
		case arg1.Kind() == reflect.String || arg2.Kind() == reflect.String:
			s.val = val(toString(arg1) + toString(arg2))
		default:
			s.val = val(toFloat(arg1) + toFloat(arg2))
		}
	case *parse.SubNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = val(arg1.Int() - arg2.Int())
		default:
			s.val = val(toFloat(arg1) - toFloat(arg2))
		}
	case *parse.DivNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(toFloat(arg1) / toFloat(arg2))
	case *parse.MulNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = val(arg1.Int() * arg2.Int())
		default:
			s.val = val(toFloat(arg1) * toFloat(arg2))
		}
	case *parse.ModNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(arg1.Int() % arg2.Int())

		// Arithmetic comparisons
	case *parse.EqNode:
		s.val = val(equals(s.eval2(dot, node.Arg1, node.Arg2)))
	case *parse.NotEqNode:
		s.val = val(!equals(s.eval2(dot, node.Arg1, node.Arg2)))
	case *parse.LtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(toFloat(arg1) < toFloat(arg2))
	case *parse.LteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(toFloat(arg1) <= toFloat(arg2))
	case *parse.GtNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(toFloat(arg1) > toFloat(arg2))
	case *parse.GteNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(toFloat(arg1) >= toFloat(arg2))

		// Boolean operators
	case *parse.NotNode:
		s.val = val(!truthiness(s.eval(dot, node.Arg)))
	case *parse.AndNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(truthiness(arg1) && truthiness(arg2))
	case *parse.OrNode:
		var arg1, arg2 = s.eval2(dot, node.Arg1, node.Arg2)
		s.val = val(truthiness(arg1) || truthiness(arg2))
	case *parse.ElvisNode:
		var arg1 = s.eval(dot, node.Arg1)
		if arg1 != nullValue && arg1 != undefinedValue {
			s.val = arg1
		} else {
			s.val = s.eval(dot, node.Arg2)
		}
	case *parse.TernNode:
		var arg1 = s.eval(dot, node.Arg1)
		if truthiness(arg1) {
			s.val = s.eval(dot, node.Arg2)
		} else {
			s.val = s.eval(dot, node.Arg3)
		}

	default:
		s.errorf("unknown node: %T", node)
	}
}

func (s *state) evalCall(dot reflect.Value, node *parse.CallNode) {
	// get template node we're calling
	var fqTemplateName = node.Name
	if node.Name[0] == '.' {
		fqTemplateName = s.namespace + node.Name
	}
	calledTmpl, ok := s.bundle.templates[fqTemplateName]
	if !ok {
		s.errorf("failed to find template: %s", fqTemplateName)
	}

	// sort out the data to pass
	var callData scope
	if node.AllData {
		callData = s.context
		callData.push()
	} else if node.Data != nil {
		result := s.eval(dot, node.Data)
		if result.Kind() != reflect.Map {
			s.errorf("In 'call' command %s, the data reference does not resolve to a map.", node)
		}
		callData = scope{
			result.Convert(reflect.TypeOf(map[string]interface{}{})).
				Interface().(map[string]interface{})}
	} else {
		callData = scope{make(map[string]interface{})}
	}

	// resolve the params
	for _, param := range node.Params {
		switch param := param.(type) {
		case *parse.CallParamValueNode:
			callData.set(param.Key, s.eval(dot, param.Value))
		case *parse.CallParamContentNode:
			callData.set(param.Key, val(string(s.renderBlock(dot, param.Content))))
		default:
			panic(fmt.Errorf("unexpected call param type: %T", param))
		}
	}

	state := &state{
		tmpl:      calledTmpl,
		bundle:    s.bundle,
		namespace: namespace(fqTemplateName),
		wr:        s.wr,
		context:   callData,
	}
	state.walk(dot, calledTmpl.Body)
}

func (s *state) renderBlock(dot reflect.Value, node parse.Node) []byte {
	var buf bytes.Buffer
	origWriter := s.wr
	s.wr = &buf
	s.walk(dot, node)
	s.wr = origWriter
	return buf.Bytes()
}

func (s *state) evalFunc(node *parse.FunctionNode) reflect.Value {
	if fn, ok := loopFuncs[node.Name]; ok {
		return fn(s, node.Args[0].(*parse.DataRefNode).Key)
	}
	if fn, ok := soyFuncs[node.Name]; ok {
		var valid = false
		for _, length := range fn.ValidArgLengths {
			if len(node.Args) == length {
				valid = true
			}
		}
		if !valid {
			panic(fmt.Errorf("call to %v with %v args, expected %v",
				node.Name, len(node.Args), fn.ValidArgLengths))
		}

		var args []reflect.Value
		for _, arg := range node.Args {
			args = append(args, s.eval(reflect.Value{}, arg))
		}
		return fn.Func(args...)
	}
	panic(fmt.Errorf("unrecognized function name: %s", node.Name))
}

func (s *state) evalDataRef(dot reflect.Value, node *parse.DataRefNode) reflect.Value {
	// get the initial value
	var ref = s.context.lookup(node.Key)
	if len(node.Access) == 0 {
		return ref
	}

	// handle the accesses
	for _, accessNode := range node.Access {
		// require val to be a slice/map at the start of each iteration.
		var kind = ref.Kind()
		if kind != reflect.Slice && kind != reflect.Map {
			if isNullSafeAccess(accessNode) {
				if ref == undefinedValue || ref == nullValue {
					return nullValue
				}
				panic(fmt.Sprintf("While evaluating \"%s\", encountered non-collection"+
					" just before accessing \"%s\".", node, accessNode))
			}
			return undefinedValue
		}

		// get a string or integer index
		switch node := accessNode.(type) {
		case *parse.DataRefIndexNode:
			ref = accessIndex(ref, val(node), node.Index)
		case *parse.DataRefKeyNode:
			ref = accessKey(ref, val(node), node.Key)
		case *parse.DataRefExprNode:
			switch keyRef := s.eval(dot, node.Arg); {
			case isInt(keyRef):
				ref = accessIndex(ref, keyRef, int(keyRef.Int()))
			default:
				ref = accessKey(ref, keyRef, toString(keyRef))
			}
		default:
			panic(fmt.Sprintf("unexpected access node: %T", node))
		}
	}

	return ref
}

// isNullSafeAccess returns true if the data ref access node is a nullsafe
// access.
func isNullSafeAccess(n parse.Node) bool {
	switch node := n.(type) {
	case *parse.DataRefIndexNode:
		return node.NullSafe
	case *parse.DataRefKeyNode:
		return node.NullSafe
	case *parse.DataRefExprNode:
		return node.NullSafe
	}
	panic("unexpected")
}

// eval2 is a helper for binary ops.  it evaluates the two given nodes.
func (s *state) eval2(dot reflect.Value, n1, n2 parse.Node) (reflect.Value, reflect.Value) {
	return s.eval(dot, n1), s.eval(dot, n2)
}

func (s *state) eval(dot reflect.Value, n parse.Node) reflect.Value {
	s.walk(dot, n)
	return drill(s.val)
}
