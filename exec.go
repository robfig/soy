package soy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"text/template"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
)

var Logger *log.Logger

type scope []data.Map // a stack of variable scopes

// push creates a new scope
func (s *scope) push() {
	*s = append(*s, make(data.Map))
}

// pop discards the last scope pushed.
func (s *scope) pop() {
	*s = (*s)[:len(*s)-1]
}

func (s *scope) augment(m map[string]interface{}) {
	*s = append(*s, data.New(m).(data.Map))
}

// set adds a new binding to the deepest scope
func (s scope) set(k string, v data.Value) {
	s[len(s)-1][k] = v
}

// lookup checks the variable scopes, deepest out, for the given key
func (s scope) lookup(k string) data.Value {
	for i := range s {
		var elem = s[len(s)-i-1]
		if val, ok := elem[k]; ok {
			return val
		}
	}
	return data.Undefined{}
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	namespace  string
	tmpl       *parse.TemplateNode
	wr         io.Writer
	node       parse.Node           // current node, for errors
	bundle     Tofu                 // the entire bundle of templates
	val        data.Value           // temp value for expression being computed
	context    scope                // variable scope
	autoescape parse.AutoescapeType // escaping mode
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
		*errp = fmt.Errorf("%#v: %v", s.node, e)
	}
}

func EvalExpr(node parse.Node) (val data.Value, err error) {
	state := &state{
		wr: ioutil.Discard,
	}
	defer state.errRecover(&err)
	state.walk(node)
	return state.val, nil
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Template) Execute(wr io.Writer, obj interface{}) (err error) {
	if t.Node == nil {
		return errors.New("no template found")
	}
	var m data.Map
	if obj == nil {
		m = data.Map{}
	} else {
		var ok bool
		m, ok = data.New(obj).(data.Map)
		if !ok {
			return fmt.Errorf("Execute data must be a map or struct, got %T", obj)
		}
	}
	state := &state{
		tmpl:      t.Node,
		bundle:    t.tofu,
		namespace: t.namespace,
		wr:        wr,
		context:   scope{m},
	}
	defer state.errRecover(&err)
	state.walk(t.Node)
	return
}

// walk recursively goes through each node and executes the indicated logic and
// writes the output
func (s *state) walk(node parse.Node) {
	s.val = data.Undefined{}
	s.at(node)
	switch node := node.(type) {
	case *parse.TemplateNode:
		s.autoescape = node.Autoescape
		s.walk(node.Body)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(node)
		}

		// Output nodes ----------
	case *parse.PrintNode:
		s.evalPrint(node)
	case *parse.RawTextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.errorf("%s", err)
		}
	case *parse.MsgNode:
		s.walk(node.Body)
	case *parse.CssNode:
		var prefix = ""
		if node.Expr != nil {
			prefix = s.eval(node.Expr).String() + "-"
		}
		if _, err := s.wr.Write([]byte(prefix + node.Suffix)); err != nil {
			s.errorf("%s", err)
		}
	case *parse.DebuggerNode:
		// nothing to do
	case *parse.LogNode:
		Logger.Print(string(s.renderBlock(node.Body)))

		// Control flow ----------
	case *parse.IfNode:
		for _, cond := range node.Conds {
			if cond.Cond == nil || s.eval(cond.Cond).Truthy() {
				s.walk(cond.Body)
				break
			}
		}
	case *parse.ForNode:
		var list, ok = s.eval(node.List).(data.List)
		if !ok {
			s.errorf("expected list type to iterate, got %v", s.eval(node.List))
		}
		if len(list) == 0 {
			if node.IfEmpty != nil {
				s.walk(node.IfEmpty)
			}
			break
		}
		s.context.push()
		for i, item := range list {
			s.context.set(node.Var, item)
			s.context.set(node.Var+"__index", data.Int(i))
			s.context.set(node.Var+"__lastIndex", data.Int(len(list)-1))
			s.walk(node.Body)
		}
		s.context.pop()
	case *parse.SwitchNode:
		var switchValue = s.eval(node.Value)
		for _, caseNode := range node.Cases {
			for _, caseValueNode := range caseNode.Values {
				if switchValue.Equals(s.eval(caseValueNode)) {
					s.walk(caseNode.Body)
					return
				}
			}
			if len(caseNode.Values) == 0 { // default/last case
				s.walk(caseNode.Body)
				return
			}
		}
	case *parse.CallNode:
		s.evalCall(node)
	case *parse.LetValueNode:
		s.context.set(node.Name, s.eval(node.Expr))
	case *parse.LetContentNode:
		s.context.set(node.Name, data.String(s.renderBlock(node.Body)))

		// Values ----------
	case *parse.NullNode:
		s.val = data.Null{}
	case *parse.StringNode:
		s.val = data.String(node.Value)
	case *parse.IntNode:
		s.val = data.Int(node.Value)
	case *parse.FloatNode:
		s.val = data.Float(node.Value)
	case *parse.BoolNode:
		s.val = data.Bool(node.True)
	case *parse.GlobalNode:
		var value, ok = s.bundle.globals[node.Name]
		if !ok {
			s.errorf("failed to find global: %q", node.Name)
		}
		s.val = value
	case *parse.ListLiteralNode:
		var items = make(data.List, len(node.Items))
		for i, item := range node.Items {
			items[i] = s.eval(item)
		}
		s.val = data.List(items)
	case *parse.MapLiteralNode:
		var items = make(data.Map, len(node.Items))
		for k, v := range node.Items {
			items[k] = s.eval(v)
		}
		s.val = data.Map(items)
	case *parse.FunctionNode:
		s.val = s.evalFunc(node)
	case *parse.DataRefNode:
		s.val = s.evalDataRef(node)

		// Arithmetic operators ----------
	case *parse.NegateNode:
		switch arg := s.eval(node.Arg).(type) {
		case data.Int:
			s.val = data.Int(-arg)
		case data.Float:
			s.val = data.Float(-arg)
		default:
			s.errorf("can not negate non-number: %v", arg)
		}
	case *parse.AddNode:
		var arg1, arg2 = s.eval2(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) + arg2.(data.Int))
		case isString(arg1) || isString(arg2):
			s.val = data.String(arg1.String() + arg2.String())
		default:
			s.val = data.Float(toFloat(arg1) + toFloat(arg2))
		}
	case *parse.SubNode:
		var arg1, arg2 = s.eval2(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) - arg2.(data.Int))
		default:
			s.val = data.Float(toFloat(arg1) - toFloat(arg2))
		}
	case *parse.DivNode:
		var arg1, arg2 = s.eval2(node.Arg1, node.Arg2)
		s.val = data.Float(toFloat(arg1) / toFloat(arg2))
	case *parse.MulNode:
		var arg1, arg2 = s.eval2(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) * arg2.(data.Int))
		default:
			s.val = data.Float(toFloat(arg1) * toFloat(arg2))
		}
	case *parse.ModNode:
		var arg1, arg2 = s.eval2(node.Arg1, node.Arg2)
		s.val = data.Int(arg1.(data.Int) % arg2.(data.Int))

		// Arithmetic comparisons ----------
	case *parse.EqNode:
		s.val = data.Bool(s.eval(node.Arg1).Equals(s.eval(node.Arg2)))
	case *parse.NotEqNode:
		s.val = data.Bool(!s.eval(node.Arg1).Equals(s.eval(node.Arg2)))
	case *parse.LtNode:
		s.val = data.Bool(toFloat(s.eval(node.Arg1)) < toFloat(s.eval(node.Arg2)))
	case *parse.LteNode:
		s.val = data.Bool(toFloat(s.eval(node.Arg1)) <= toFloat(s.eval(node.Arg2)))
	case *parse.GtNode:
		s.val = data.Bool(toFloat(s.eval(node.Arg1)) > toFloat(s.eval(node.Arg2)))
	case *parse.GteNode:
		s.val = data.Bool(toFloat(s.eval(node.Arg1)) >= toFloat(s.eval(node.Arg2)))

		// Boolean operators ----------
	case *parse.NotNode:
		s.val = data.Bool(!s.eval(node.Arg).Truthy())
	case *parse.AndNode:
		s.val = data.Bool(s.eval(node.Arg1).Truthy() && s.eval(node.Arg2).Truthy())
	case *parse.OrNode:
		s.val = data.Bool(s.eval(node.Arg1).Truthy() || s.eval(node.Arg2).Truthy())
	case *parse.ElvisNode:
		var arg1 = s.eval(node.Arg1)
		if arg1 != (data.Null{}) && arg1 != (data.Undefined{}) {
			s.val = arg1
		} else {
			s.val = s.eval(node.Arg2)
		}
	case *parse.TernNode:
		var arg1 = s.eval(node.Arg1)
		if arg1.Truthy() {
			s.val = s.eval(node.Arg2)
		} else {
			s.val = s.eval(node.Arg3)
		}

	default:
		s.errorf("unknown node: %T", node)
	}
}

func isInt(v data.Value) bool {
	_, ok := v.(data.Int)
	return ok
}

func isString(v data.Value) bool {
	_, ok := v.(data.String)
	return ok
}

func toFloat(v data.Value) float64 {
	switch v := v.(type) {
	case data.Int:
		return float64(v)
	case data.Float:
		return float64(v)
	default:
		panic(fmt.Errorf("not a number: %v", v))
		return 0
	}
}

func (s *state) evalPrint(node *parse.PrintNode) {
	s.walk(node.Arg)
	var escapeHtml = s.autoescape == parse.AutoescapeOn
	var result = s.val
	for _, directiveNode := range node.Directives {
		var directive, ok = printDirectiveByName[directiveNode.Name]
		if !ok {
			s.errorf("print directive %q does not exist", directiveNode.Name)
		}
		// TODO: validate # args
		var args []data.Value
		for _, arg := range directiveNode.Args {
			args = append(args, s.eval(arg))
		}
		result = directive.Apply(result, args)
		if directive.CancelAutoescape {
			escapeHtml = false
		}
	}

	if escapeHtml {
		template.HTMLEscape(s.wr, []byte(result.String()))
	} else {
		if _, err := s.wr.Write([]byte(result.String())); err != nil {
			s.errorf("%s", err)
		}
	}
}

func (s *state) evalCall(node *parse.CallNode) {
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
		result, ok := s.eval(node.Data).(data.Map)
		if !ok {
			s.errorf("In 'call' command %s, the data reference does not resolve to a map.", node)
		}
		callData = scope{result}
	} else {
		callData = scope{make(data.Map)}
	}

	// resolve the params
	for _, param := range node.Params {
		switch param := param.(type) {
		case *parse.CallParamValueNode:
			callData.set(param.Key, s.eval(param.Value))
		case *parse.CallParamContentNode:
			callData.set(param.Key, data.New(string(s.renderBlock(param.Content))))
		default:
			s.errorf("unexpected call param type: %T", param)
		}
	}

	state := &state{
		tmpl:      calledTmpl,
		bundle:    s.bundle,
		namespace: namespace(fqTemplateName),
		wr:        s.wr,
		context:   callData,
	}
	state.walk(calledTmpl)
}

func (s *state) renderBlock(node parse.Node) []byte {
	var buf bytes.Buffer
	origWriter := s.wr
	s.wr = &buf
	s.walk(node)
	s.wr = origWriter
	return buf.Bytes()
}

func (s *state) evalFunc(node *parse.FunctionNode) data.Value {
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
			s.errorf("call to %v with %v args, expected %v",
				node.Name, len(node.Args), fn.ValidArgLengths)
		}

		var args []data.Value
		for _, arg := range node.Args {
			args = append(args, s.eval(arg))
		}
		return fn.Func(args)
	}
	s.errorf("unrecognized function name: %s", node.Name)
	return nil
}

func (s *state) evalDataRef(node *parse.DataRefNode) data.Value {
	// get the initial value
	var ref = s.context.lookup(node.Key)
	if len(node.Access) == 0 {
		return ref
	}

	// handle the accesses
	for _, accessNode := range node.Access {
		// resolve the index or key to look up.
		var (
			index int = -1
			key   string
		)
		switch node := accessNode.(type) {
		case *parse.DataRefIndexNode:
			index = node.Index
		case *parse.DataRefKeyNode:
			key = node.Key
		case *parse.DataRefExprNode:
			switch keyRef := s.eval(node.Arg).(type) {
			case data.Int:
				index = int(keyRef)
			default:
				key = keyRef.String()
			}
		default:
			s.errorf("unexpected access node: %T", node)
		}

		// use the key/index, depending on the data type we're accessing.
		switch obj := ref.(type) {
		case data.Undefined, data.Null:
			if isNullSafeAccess(accessNode) {
				return data.Null{}
			}
			s.errorf("%v: access failed due to null/undefined reference", accessNode)
		case data.List:
			if index == -1 {
				s.errorf("%v: access list with non-integer index failed.", accessNode)
			}
			ref = obj.Index(index)
		case data.Map:
			if key == "" {
				s.errorf("%v: access to map requires a string key.", accessNode)
			}
			ref = obj.Key(key)
		default:
			s.errorf("While evaluating \"%s\", encountered non-collection"+
				" just before accessing \"%s\".", node, accessNode)
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
func (s *state) eval2(n1, n2 parse.Node) (data.Value, data.Value) {
	return s.eval(n1), s.eval(n2)
}

func (s *state) eval(n parse.Node) data.Value {
	s.walk(n)
	return s.val
}
