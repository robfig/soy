package soyhtml

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"runtime"
	"runtime/debug"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/soymsg"
	soyt "github.com/robfig/soy/template"
)

// Logger collects output from {log} commands.
var Logger *log.Logger

// state represents the state of an execution.
type state struct {
	namespace  string
	tmpl       soyt.Template
	wr         io.Writer
	node       ast.Node           // current node, for errors
	registry   soyt.Registry      // the entire bundle of templates
	val        data.Value         // temp value for expression being computed
	context    scope              // variable scope
	autoescape ast.AutoescapeType // escaping mode
	ij         data.Map           // injected data available to all templates.
	msgs       soymsg.Bundle      // replacement text for {msg} tags
}

// at marks the state to be on node n, for error reporting.
func (s *state) at(node ast.Node) {
	s.node = node
}

// errorf formats the error and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	format = fmt.Sprintf("template %s:%d: %s", s.tmpl.Node.Name,
		s.registry.LineNumber(s.tmpl.Node.Name, s.node), format)
	panic(fmt.Errorf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func (s *state) errRecover(errp *error) {
	if e := recover(); e != nil {
		switch e := e.(type) {
		case runtime.Error:
			*errp = fmt.Errorf("template %s:%d: %v\n%v", s.tmpl.Node.Name,
				s.registry.LineNumber(s.tmpl.Node.Name, s.node), e, string(debug.Stack()))
		case error:
			*errp = e
		default:
			*errp = fmt.Errorf("template %s:%d: %v", s.tmpl.Node.Name,
				s.registry.LineNumber(s.tmpl.Node.Name, s.node), e)
		}
	}
}

// walk recursively goes through each node and executes the indicated logic and
// writes the output
func (s *state) walk(node ast.Node) {
	s.val = data.Undefined{}
	s.at(node)
	switch node := node.(type) {
	case *ast.SoyFileNode:
		for _, node := range node.Body {
			s.walk(node)
		}
	case *ast.TemplateNode:
		if node.Autoescape != ast.AutoescapeUnspecified {
			s.autoescape = node.Autoescape
		}
		s.walk(node.Body)
	case *ast.ListNode:
		for _, node := range node.Nodes {
			s.walk(node)
		}

		// Output nodes ----------
	case *ast.PrintNode:
		s.evalPrint(node)
	case *ast.RawTextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.errorf("%s", err)
		}
	case *ast.MsgNode:
		s.evalMsg(node)
	case *ast.CssNode:
		var prefix = ""
		if node.Expr != nil {
			prefix = s.eval(node.Expr).String() + "-"
		}
		if _, err := io.WriteString(s.wr, prefix+node.Suffix); err != nil {
			s.errorf("%s", err)
		}
	case *ast.DebuggerNode:
		// nothing to do
	case *ast.LogNode:
		Logger.Print(string(s.renderBlock(node.Body)))

		// Control flow ----------
	case *ast.IfNode:
		for _, cond := range node.Conds {
			if cond.Cond == nil || s.eval(cond.Cond).Truthy() {
				s.walk(cond.Body)
				break
			}
		}
	case *ast.ForNode:
		var list, ok = s.eval(node.List).(data.List)
		if !ok {
			s.errorf("In for loop %q, %q does not resolve to a list.",
				node.String(), node.List.String())
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
	case *ast.SwitchNode:
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
	case *ast.CallNode:
		s.evalCall(node)
	case *ast.LetValueNode:
		s.context.set(node.Name, s.eval(node.Expr))
	case *ast.LetContentNode:
		s.context.set(node.Name, data.String(s.renderBlock(node.Body)))

		// Values ----------
	case *ast.NullNode:
		s.val = data.Null{}
	case *ast.StringNode:
		s.val = data.String(node.Value)
	case *ast.IntNode:
		s.val = data.Int(node.Value)
	case *ast.FloatNode:
		s.val = data.Float(node.Value)
	case *ast.BoolNode:
		s.val = data.Bool(node.True)
	case *ast.GlobalNode:
		s.val = node.Value
	case *ast.ListLiteralNode:
		var items = make(data.List, len(node.Items))
		for i, item := range node.Items {
			items[i] = s.eval(item)
		}
		s.val = data.List(items)
	case *ast.MapLiteralNode:
		var items = make(data.Map, len(node.Items))
		for k, v := range node.Items {
			items[k] = s.eval(v)
		}
		s.val = data.Map(items)
	case *ast.FunctionNode:
		s.val = s.evalFunc(node)
	case *ast.DataRefNode:
		s.val = s.evalDataRef(node)

		// Arithmetic operators ----------
	case *ast.NegateNode:
		switch arg := s.evaldef(node.Arg).(type) {
		case data.Int:
			s.val = data.Int(-arg)
		case data.Float:
			s.val = data.Float(-arg)
		default:
			s.errorf("can not negate non-number: %q", arg.String())
		}
	case *ast.AddNode:
		var arg1, arg2 = s.eval2def(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) + arg2.(data.Int))
		case isString(arg1) || isString(arg2):
			s.val = data.String(arg1.String() + arg2.String())
		default:
			s.val = data.Float(toFloat(arg1) + toFloat(arg2))
		}
	case *ast.SubNode:
		var arg1, arg2 = s.eval2def(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) - arg2.(data.Int))
		default:
			s.val = data.Float(toFloat(arg1) - toFloat(arg2))
		}
	case *ast.DivNode:
		var arg1, arg2 = s.eval2def(node.Arg1, node.Arg2)
		s.val = data.Float(toFloat(arg1) / toFloat(arg2))
	case *ast.MulNode:
		var arg1, arg2 = s.eval2def(node.Arg1, node.Arg2)
		switch {
		case isInt(arg1) && isInt(arg2):
			s.val = data.Int(arg1.(data.Int) * arg2.(data.Int))
		default:
			s.val = data.Float(toFloat(arg1) * toFloat(arg2))
		}
	case *ast.ModNode:
		var arg1, arg2 = s.eval2def(node.Arg1, node.Arg2)
		s.val = data.Int(arg1.(data.Int) % arg2.(data.Int))

		// Arithmetic comparisons ----------
	case *ast.EqNode:
		s.val = data.Bool(s.eval(node.Arg1).Equals(s.eval(node.Arg2)))
	case *ast.NotEqNode:
		s.val = data.Bool(!s.eval(node.Arg1).Equals(s.eval(node.Arg2)))
	case *ast.LtNode:
		s.val = data.Bool(toFloat(s.evaldef(node.Arg1)) < toFloat(s.evaldef(node.Arg2)))
	case *ast.LteNode:
		s.val = data.Bool(toFloat(s.evaldef(node.Arg1)) <= toFloat(s.evaldef(node.Arg2)))
	case *ast.GtNode:
		s.val = data.Bool(toFloat(s.evaldef(node.Arg1)) > toFloat(s.evaldef(node.Arg2)))
	case *ast.GteNode:
		s.val = data.Bool(toFloat(s.evaldef(node.Arg1)) >= toFloat(s.evaldef(node.Arg2)))

		// Boolean operators ----------
	case *ast.NotNode:
		s.val = data.Bool(!s.eval(node.Arg).Truthy())
	case *ast.AndNode:
		s.val = data.Bool(s.eval(node.Arg1).Truthy() && s.eval(node.Arg2).Truthy())
	case *ast.OrNode:
		s.val = data.Bool(s.eval(node.Arg1).Truthy() || s.eval(node.Arg2).Truthy())
	case *ast.ElvisNode:
		var arg1 = s.eval(node.Arg1)
		if arg1 != (data.Null{}) && arg1 != (data.Undefined{}) {
			s.val = arg1
		} else {
			s.val = s.eval(node.Arg2)
		}
	case *ast.TernNode:
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
	case data.Undefined:
		panic("not a number: undefined")
	default:
		panic(fmt.Sprintf("not a number: %v (%T)", v, v))
	}
}

func (s *state) evalPrint(node *ast.PrintNode) {
	s.walk(node.Arg)
	if _, ok := s.val.(data.Undefined); ok {
		s.errorf("In 'print' tag, expression %q evaluates to undefined.", node.Arg.String())
	}
	var escapeHtml = s.autoescape != ast.AutoescapeOff
	var result = s.val
	for _, directiveNode := range node.Directives {
		var directive, ok = PrintDirectives[directiveNode.Name]
		if !ok {
			s.errorf("Print directive %q does not exist", directiveNode.Name)
		}

		if !checkNumArgs(directive.ValidArgLengths, len(directiveNode.Args)) {
			s.errorf("Print directive %q called with %v args, expected one of: %v",
				directiveNode.Name, len(directiveNode.Args), directive.ValidArgLengths)
		}

		var args = make([]data.Value, len(directiveNode.Args))
		for i, arg := range directiveNode.Args {
			args[i] = s.eval(arg)
		}
		func() {
			defer func() {
				if err := recover(); err != nil {
					s.errorf("panic in %v: %v\nexecuted: %v(%q, %v)\n%v",
						directiveNode, err,
						directiveNode.Name, result, args,
						string(debug.Stack()))
				}
			}()
			result = directive.Apply(result, args)
		}()
		if directive.CancelAutoescape {
			escapeHtml = false
		}
	}

	var resultStr = result.String()
	if escapeHtml {
		htmlEscapeString(s.wr, resultStr)
	} else {
		if _, err := io.WriteString(s.wr, resultStr); err != nil {
			s.errorf("%s", err)
		}
	}
}

func (s *state) evalMsg(node *ast.MsgNode) {
	// If no bundle was provided, walk the message sub-nodes.
	if s.msgs == nil {
		s.walkMsg(node)
		return
	}

	// Look up the message in the bundle.
	var msg = s.msgs.Message(node.ID)
	if msg == nil {
		s.walkMsg(node)
		return
	}

	// Translated message found.  Render each part.
	for _, part := range msg.Parts {
		if part.Content != "" {
			if _, err := io.WriteString(s.wr, part.Content); err != nil {
				s.errorf("%s", err)
			}
			continue
		}

		// It's a placeholder
		// Find the right node to walk.
		var found = false
		for _, phnode := range node.Body {
			if phnode, ok := phnode.(*ast.MsgPlaceholderNode); ok && phnode.Name == part.Placeholder {
				s.walk(phnode.Body)
				found = true
				break
			}
		}
		if !found {
			s.errorf("failed to find placeholder %q in %v", part.Placeholder, node.PlaceholderString())
		}
	}
}

func (s *state) walkMsg(node *ast.MsgNode) {
	for _, n := range node.Body {
		switch n := n.(type) {
		case *ast.RawTextNode:
			s.walk(n)
		case *ast.MsgPlaceholderNode:
			s.walk(n.Body)
		}
	}
}

func (s *state) evalCall(node *ast.CallNode) {
	// get template node we're calling
	var calledTmpl, ok = s.registry.Template(node.Name)
	if !ok {
		s.errorf("failed to find template: %s", node.Name)
	}

	// sort out the data to pass
	var callData scope
	if node.AllData {
		callData = s.context.alldata()
		callData.push()
	} else if node.Data != nil {
		result, ok := s.eval(node.Data).(data.Map)
		if !ok {
			s.errorf("In 'call' command %q, the data reference %q does not resolve to a map.",
				node.String(), node.Data.String())
		}
		callData = newScope(result)
	} else {
		callData = newScope(make(data.Map))
	}

	// resolve the params
	for _, param := range node.Params {
		switch param := param.(type) {
		case *ast.CallParamValueNode:
			callData.set(param.Key, s.eval(param.Value))
		case *ast.CallParamContentNode:
			callData.set(param.Key, data.New(string(s.renderBlock(param.Content))))
		default:
			s.errorf("unexpected call param type: %T", param)
		}
	}

	callData.enter()
	state := &state{
		tmpl:       calledTmpl,
		registry:   s.registry,
		namespace:  calledTmpl.Namespace.Name,
		autoescape: calledTmpl.Namespace.Autoescape,
		wr:         s.wr,
		context:    callData,
		ij:         s.ij,
	}
	state.walk(calledTmpl.Node)
}

// renderBlock is a helper that renders the given node to a temporary output
// buffer and returns that result.  nothing is written to the main output.
func (s *state) renderBlock(node ast.Node) []byte {
	var buf bytes.Buffer
	origWriter := s.wr
	s.wr = &buf
	s.walk(node)
	s.wr = origWriter
	return buf.Bytes()
}

func checkNumArgs(allowedNumArgs []int, numArgs int) bool {
	for _, length := range allowedNumArgs {
		if numArgs == length {
			return true
		}
	}
	return false
}

func (s *state) evalFunc(node *ast.FunctionNode) data.Value {
	if fn, ok := loopFuncs[node.Name]; ok {
		return fn(s, node.Args[0].(*ast.DataRefNode).Key)
	}
	if fn, ok := Funcs[node.Name]; ok {
		if !checkNumArgs(fn.ValidArgLengths, len(node.Args)) {
			s.errorf("Function %q called with %v args, expected: %v",
				node.Name, len(node.Args), fn.ValidArgLengths)
		}

		var args = make([]data.Value, len(node.Args))
		for i, arg := range node.Args {
			args[i] = s.eval(arg)
		}
		defer func() {
			if err := recover(); err != nil {
				s.errorf("panic in %s(%v): %v\n%v", node.Name, args, err, string(debug.Stack()))
			}
		}()
		r := fn.Apply(args)
		if r == nil {
			return data.Null{}
		}
		return r
	}
	s.errorf("unrecognized function name: %s", node.Name)
	panic("unreachable")
}

func (s *state) evalDataRef(node *ast.DataRefNode) data.Value {
	// get the initial value
	var ref data.Value
	if node.Key == "ij" {
		if s.ij == nil {
			s.errorf("Injected data not provided, yet referenced: %q", node.String())
		}
		ref = s.ij
	} else {
		ref = s.context.lookup(node.Key)
	}
	if len(node.Access) == 0 {
		return ref
	}

	// handle the accesses
	for i, accessNode := range node.Access {
		// resolve the index or key to look up.
		var (
			index int = -1
			key   string
		)
		switch node := accessNode.(type) {
		case *ast.DataRefIndexNode:
			index = node.Index
		case *ast.DataRefKeyNode:
			key = node.Key
		case *ast.DataRefExprNode:
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
			s.errorf("%q is null or undefined",
				(&ast.DataRefNode{node.Pos, node.Key, node.Access[:i]}).String())
		case data.List:
			if index == -1 {
				s.errorf("%q is a list, but was accessed with a non-integer index",
					(&ast.DataRefNode{node.Pos, node.Key, node.Access[:i]}).String())
			}
			ref = obj.Index(index)
		case data.Map:
			if key == "" {
				s.errorf("%q is a map, and requires a string key to access",
					(&ast.DataRefNode{node.Pos, node.Key, node.Access[:i]}).String())
			}
			ref = obj.Key(key)
		default:
			s.errorf("While evaluating \"%v\", encountered non-collection"+
				" just before accessing \"%v\".", node, accessNode)
		}
	}

	return ref
}

// isNullSafeAccess returns true if the data ref access node is a nullsafe
// access.
func isNullSafeAccess(n ast.Node) bool {
	switch node := n.(type) {
	case *ast.DataRefIndexNode:
		return node.NullSafe
	case *ast.DataRefKeyNode:
		return node.NullSafe
	case *ast.DataRefExprNode:
		return node.NullSafe
	}
	panic("unexpected")
}

// eval2def is a helper for binary ops.  it evaluates the two given nodes and
// requires the result of each to not be Undefined.
func (s *state) eval2def(n1, n2 ast.Node) (data.Value, data.Value) {
	return s.evaldef(n1), s.evaldef(n2)
}

func (s *state) eval(n ast.Node) data.Value {
	var prev = s.node
	s.walk(n)
	s.node = prev
	return s.val
}

func (s *state) evaldef(n ast.Node) data.Value {
	var val = s.eval(n)
	if _, ok := val.(data.Undefined); ok {
		s.errorf("%v is undefined", n)
	}
	return val
}

var (
	htmlQuot = []byte("&#34;") // shorter than "&quot;"
	htmlApos = []byte("&#39;") // shorter than "&apos;" and apos was not in HTML until HTML5
	htmlAmp  = []byte("&amp;")
	htmlLt   = []byte("&lt;")
	htmlGt   = []byte("&gt;")
)

// htmlEscapeString is a modified veresion of the stdlib HTMLEscape routine
// escapes a string without making copies.
func htmlEscapeString(w io.Writer, str string) {
	last := 0
	for i := 0; i < len(str); i++ {
		var html []byte
		switch str[i] {
		case '"':
			html = htmlQuot
		case '\'':
			html = htmlApos
		case '&':
			html = htmlAmp
		case '<':
			html = htmlLt
		case '>':
			html = htmlGt
		default:
			continue
		}
		io.WriteString(w, str[last:i])
		w.Write(html)
		last = i + 1
	}
	io.WriteString(w, str[last:])
}
