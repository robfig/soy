package soyjs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/soymsg"
)

type state struct {
	wr           io.Writer
	node         ast.Node // current node, for errors
	indentLevels int
	namespace    string
	bufferName   string
	varnum       int
	scope        scope
	autoescape   ast.AutoescapeType
	lastNode     ast.Node
	options      Options
	funcsCalled  map[string]string
	funcsInFile  map[string]bool
}

func difference(a map[string]string, b map[string]bool) []string {
	new := []string{}
	for key1 := range a {
		if _, ok := b[key1]; !ok {
			new = append(new, key1)
		}
	}
	return new
}

// Write writes the javascript represented by the given node to the given
// writer.  The first error encountered is returned.
func Write(out io.Writer, node ast.Node, options Options) (err error) {
	defer errRecover(&err)

	if options.Formatter == nil {
		options.Formatter = &ES5Formatter{}
	}

	var (
		tmpOut     = &bytes.Buffer{}
		importsBuf = &bytes.Buffer{}
		s          = &state{
			wr:          tmpOut,
			options:     options,
			funcsCalled: map[string]string{},
			funcsInFile: map[string]bool{},
		}
	)

	s.scope.push()
	s.walk(node)

	if len(s.funcsCalled) > 0 {
		for _, f := range difference(s.funcsCalled, s.funcsInFile) {
			importsBuf.WriteString(s.funcsCalled[f])
			importsBuf.WriteRune('\n')
		}
		importsBuf.WriteRune('\n')
	}

	out.Write(importsBuf.Bytes())
	out.Write(tmpOut.Bytes())

	return nil
}

// at marks the state to be on node n, for error reporting.
func (s *state) at(node ast.Node) {
	s.lastNode = s.node
	s.node = node
}

// errorf formats the error and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		*errp = fmt.Errorf("%v", e)
	}
}

// walk recursively goes through each node and translates the nodes to
// javascript, writing the result to s.wr
func (s *state) walk(node ast.Node) {
	s.at(node)
	switch node := node.(type) {
	case *ast.SoyFileNode:
		s.visitSoyFile(node)
	case *ast.NamespaceNode:
		s.visitNamespace(node)
	case *ast.SoyDocNode:
		return
	case *ast.TemplateNode:
		s.visitTemplate(node)
	case *ast.ListNode:
		s.visitChildren(node)

		// Output nodes ----------
	case *ast.RawTextNode:
		s.writeRawText(node.Text)
	case *ast.PrintNode:
		s.visitPrint(node)
	case *ast.MsgNode:
		s.visitMsg(node)
	case *ast.MsgHtmlTagNode:
		s.writeRawText(node.Text)
	case *ast.CssNode:
		if node.Expr != nil {
			s.jsln(s.bufferName, " += ", node.Expr, " + '-';")
		}
		s.writeRawText([]byte(node.Suffix))
	case *ast.DebuggerNode:
		s.jsln("debugger;")
	case *ast.LogNode:
		s.bufferName += "_"
		s.jsln("var ", s.bufferName, " = '';")
		s.walk(node.Body)
		s.jsln("console.log(", s.bufferName, ");")
		s.bufferName = s.bufferName[:len(s.bufferName)-1]

	// Control flow ----------
	case *ast.IfNode:
		s.visitIf(node)
	case *ast.ForNode:
		s.visitFor(node)
	case *ast.SwitchNode:
		s.visitSwitch(node)
	case *ast.CallNode:
		s.visitCall(node)
	case *ast.LetValueNode:
		s.jsln("var ", s.scope.makevar(node.Name), " = ", node.Expr, ";")
	case *ast.LetContentNode:
		var oldBufferName = s.bufferName
		s.bufferName = s.scope.makevar(node.Name)
		s.jsln("var ", s.bufferName, " = '';")
		s.walk(node.Body)
		s.bufferName = oldBufferName

	// Values ----------
	case *ast.NullNode:
		s.js("null")
	case *ast.StringNode:
		s.js("'")
		template.JSEscape(s.wr, []byte(node.Value))
		s.js("'")
	case *ast.IntNode:
		s.js(node.String())
	case *ast.FloatNode:
		s.js(node.String())
	case *ast.BoolNode:
		s.js(node.String())
	case *ast.GlobalNode:
		s.visitGlobal(node)
	case *ast.ListLiteralNode:
		s.js("[")
		for i, item := range node.Items {
			if i != 0 {
				s.js(",")
			}
			s.walk(item)
		}
		s.js("]")
	case *ast.MapLiteralNode:
		s.js("{")
		var (
			first = true
			keys  = make([]string, len(node.Items))
			i     = 0
		)
		for k := range node.Items {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !first {
				s.js(",")
			}
			first = false
			s.js("\"", k, "\"", ":")
			s.walk(node.Items[k])
		}
		s.js("}")
	case *ast.FunctionNode:
		s.visitFunction(node)
	case *ast.DataRefNode:
		s.visitDataRef(node)

	// Arithmetic operators ----------
	case *ast.NegateNode:
		s.js("(-", node.Arg, ")")
	case *ast.AddNode:
		s.op("+", node)
	case *ast.SubNode:
		s.op("-", node)
	case *ast.DivNode:
		s.op("/", node)
	case *ast.MulNode:
		s.op("*", node)
	case *ast.ModNode:
		s.op("%", node)

		// Arithmetic comparisons ----------
	case *ast.EqNode:
		s.op("==", node)
	case *ast.NotEqNode:
		s.op("!=", node)
	case *ast.LtNode:
		s.op("<", node)
	case *ast.LteNode:
		s.op("<=", node)
	case *ast.GtNode:
		s.op(">", node)
	case *ast.GteNode:
		s.op(">=", node)

	// Boolean operators ----------
	case *ast.NotNode:
		s.js("!(", node.Arg, ")")
	case *ast.AndNode:
		s.op("&&", node)
	case *ast.OrNode:
		s.op("||", node)
	case *ast.ElvisNode:
		// ?: is specified to check for null.
		s.js("((", node.Arg1, ") != null ? ", node.Arg1, " : ", node.Arg2, ")")
	case *ast.TernNode:
		s.js("((", node.Arg1, ") ?", node.Arg2, ":", node.Arg3, ")")

	default:
		s.errorf("unknown node (%T): %v", node, node)
	}
}

func (s *state) visitSoyFile(node *ast.SoyFileNode) {
	s.jsln("// This file was automatically generated from ", node.Name, ".")
	s.jsln("// Please don't edit this file by hand.")
	s.jsln("")
	s.visitChildren(node)
}

func (s *state) visitChildren(parent ast.ParentNode) {
	for _, child := range parent.Children() {
		s.walk(child)
	}
}

func (s *state) visitNamespace(node *ast.NamespaceNode) {
	s.namespace = node.Name
	s.autoescape = node.Autoescape

	// iterate through the dot segments.
	var i = 0
	for i < len(node.Name) {
		var decl = "var "
		var prev = i + 1
		i = strings.Index(node.Name[prev:], ".")
		if i == -1 {
			i = len(node.Name)
		} else {
			i += prev
		}
		if strings.Contains(node.Name[:i], ".") {
			decl = ""
		}
		s.jsln("if (typeof ", node.Name[:i], " == 'undefined') { ", decl, node.Name[:i], " = {}; }")
	}
}

func (s *state) visitTemplate(node *ast.TemplateNode) {
	var oldAutoescape = s.autoescape
	if node.Autoescape != ast.AutoescapeUnspecified {
		s.autoescape = node.Autoescape
	}

	// Determine if we need nullsafe initialization for opt_data
	var allOptionalParams = false
	if soydoc, ok := s.lastNode.(*ast.SoyDocNode); ok {
		allOptionalParams = len(soydoc.Params) > 0
		for _, param := range soydoc.Params {
			if !param.Optional {
				allOptionalParams = false
			}
		}
	}

	s.jsln("")
	callName, callStyle := s.options.Formatter.Template(node.Name)
	s.jsln(callStyle, "(opt_data, opt_sb, opt_ijData) {")
	s.funcsInFile[callName] = true
	s.indentLevels++
	if allOptionalParams {
		s.jsln("opt_data = opt_data || {};")
	}
	s.jsln("var output = '';")
	s.bufferName = "output"
	s.scope.push()
	defer s.scope.pop()
	s.walk(node.Body)
	s.jsln("return output;")
	s.indentLevels--
	s.jsln("};")
	s.autoescape = oldAutoescape
}

// TODO: unify print directives
func (s *state) visitPrint(node *ast.PrintNode) {
	var escape = s.autoescape
	var directives []*ast.PrintDirectiveNode
	for _, dir := range node.Directives {
		var directive, ok = PrintDirectives[dir.Name]
		if !ok {
			s.errorf("Print directive %q not found", dir.Name)
		}
		if directive.CancelAutoescape {
			escape = ast.AutoescapeOff
		}
		switch dir.Name {
		case "id", "noAutoescape":
			// no implementation, they just serve as a marker to cancel autoescape.
		default:
			directives = append(directives, dir)
			if impt := s.options.Formatter.Directive(directive); impt != "" {
				s.funcsCalled[dir.Name] = impt
			}
		}
	}
	if escape != ast.AutoescapeOff {
		directives = append([]*ast.PrintDirectiveNode{{0, "escapeHtml", nil}}, directives...)
	}

	s.indent()
	s.js(s.bufferName, " += ")
	for _, dir := range directives {
		s.js(PrintDirectives[dir.Name].Name, "(")
	}
	s.walk(node.Arg)
	for i := range directives {
		var dir = directives[len(directives)-1-i]
		for _, arg := range dir.Args {
			s.js(",")
			s.walk(arg)
		}
		// soy specifies truncate adds ellipsis by default, so we have to pass
		// doAddEllipsis = true to soy.$$truncate
		if dir.Name == "truncate" && len(dir.Args) == 1 {
			s.js(",true")
		}
		s.js(")")
	}
	s.js(";\n")
}

func (s *state) visitFunction(node *ast.FunctionNode) {
	if fn, ok := Funcs[node.Name]; ok {
		fn.Apply(s, node.Args)
		if impt := s.options.Formatter.Function(fn); impt != "" {
			s.funcsCalled[node.Name] = impt
		}
		return
	}

	switch node.Name {
	case "isFirst":
		// TODO: Add compile-time check that this is only called on loop variable.
		s.js("(", s.scope.loopindex(), " == 0)")
	case "isLast":
		s.js("(", s.scope.loopindex(), " == ", s.scope.looplimit(), " - 1)")
	case "index":
		s.js(s.scope.loopindex())
	default:
		s.errorf("unimplemented function: %v", node.Name)
	}
}

func (s *state) visitDataRef(node *ast.DataRefNode) {
	var expr string
	if node.Key == "ij" {
		expr = "opt_ijData"
	} else if genVarName := s.scope.lookup(node.Key); genVarName != "" {
		expr = genVarName
	} else {
		expr = "opt_data." + node.Key
	}

	// Nullsafe access makes this complicated.
	// FOO.BAR?.BAZ => (FOO.BAR == null ? null : FOO.BAR.BAZ)
	for _, accessNode := range node.Access {
		switch node := accessNode.(type) {
		case *ast.DataRefIndexNode:
			if node.NullSafe {
				s.js("(", expr, " == null) ? null : ")
			}
			expr += "[" + strconv.Itoa(node.Index) + "]"
		case *ast.DataRefKeyNode:
			if node.NullSafe {
				s.js("(", expr, " == null) ? null : ")
			}
			expr += "." + node.Key
		case *ast.DataRefExprNode:
			if node.NullSafe {
				s.js("(", expr, " == null) ? null : ")
			}
			expr += "[" + s.block(node.Arg) + "]"
		}
	}
	s.js(expr)
}

func (s *state) visitCall(node *ast.CallNode) {
	var dataExpr = "{}"
	if node.Data != nil {
		dataExpr = s.block(node.Data)
	} else if node.AllData {
		dataExpr = "opt_data"
	}

	if len(node.Params) > 0 {
		dataExpr = "soy.$$augmentMap(" + dataExpr + ", {"
		for i, param := range node.Params {
			if i > 0 {
				dataExpr += ", "
			}
			switch param := param.(type) {
			case *ast.CallParamValueNode:
				dataExpr += param.Key + ": " + s.block(param.Value)
			case *ast.CallParamContentNode:
				var oldBufferName = s.bufferName
				s.bufferName = s.scope.makevar("param")
				s.jsln("var ", s.bufferName, " = '';")
				s.walk(param.Content)
				dataExpr += param.Key + ": " + s.bufferName
				s.bufferName = oldBufferName
			}
		}
		dataExpr += "})"
	}
	callName, importString := s.options.Formatter.Call(node.Name)
	s.jsln(s.bufferName, " += ", callName, "(", dataExpr, ", opt_sb, opt_ijData);")
	if importString != "" {
		s.funcsCalled[callName] = importString
	}
}

func (s *state) visitIf(node *ast.IfNode) {
	s.indent()
	for i, branch := range node.Conds {
		if i > 0 {
			s.js(" else ")
		}
		if branch.Cond != nil {
			s.js("if (", branch.Cond, ") ")
		}
		s.js("{\n")
		s.indentLevels++
		s.walk(branch.Body)
		s.indentLevels--
		s.indent()
		s.js("}")
	}
	s.js("\n")
}

func (s *state) visitFor(node *ast.ForNode) {
	if rangeNode, ok := node.List.(*ast.FunctionNode); ok && rangeNode.Name == "range" {
		s.visitForRange(node)
	} else {
		s.visitForeach(node)
	}
}

func (s *state) visitForRange(node *ast.ForNode) {
	var rangeNode = node.List.(*ast.FunctionNode)
	var (
		increment ast.Node = &ast.IntNode{0, 1}
		init      ast.Node = &ast.IntNode{0, 0}
		limit     ast.Node
	)
	switch len(rangeNode.Args) {
	case 3:
		increment = rangeNode.Args[2]
		fallthrough
	case 2:
		init = rangeNode.Args[0]
		limit = rangeNode.Args[1]
	case 1:
		limit = rangeNode.Args[0]
	}

	var varIndex,
		varLimit = s.scope.pushForRange(node.Var)
	defer s.scope.pop()
	s.jsln("var ", varLimit, " = ", limit, ";")
	s.jsln("for (var ", varIndex, " = ", init, "; ",
		varIndex, " < ", varLimit, "; ",
		varIndex, " += ", increment, ") {")
	s.indentLevels++
	s.walk(node.Body)
	s.indentLevels--
	s.jsln("}")
}

func (s *state) visitForeach(node *ast.ForNode) {
	var itemData,
		itemList,
		itemListLen,
		itemIndex = s.scope.pushForEach(node.Var)
	defer s.scope.pop()
	s.jsln("var ", itemList, " = ", node.List, ";")
	s.jsln("var ", itemListLen, " = ", itemList, ".length;")
	if node.IfEmpty != nil {
		s.jsln("if (", itemListLen, " > 0) {")
		s.indentLevels++
	}
	s.jsln("for (var ", itemIndex, " = 0; ", itemIndex, " < ", itemListLen, "; ", itemIndex, "++) {")
	s.indentLevels++
	s.jsln("var ", itemData, " = ", itemList, "[", itemIndex, "];")
	s.walk(node.Body)
	s.indentLevels--
	s.jsln("}")
	if node.IfEmpty != nil {
		s.indentLevels--
		s.jsln("} else {")
		s.indentLevels++
		s.walk(node.IfEmpty)
		s.indentLevels--
		s.jsln("}")
	}
}

func (s *state) visitSwitch(node *ast.SwitchNode) {
	s.jsln("switch (", node.Value, ") {")
	s.indentLevels++
	for _, switchCase := range node.Cases {
		for _, switchCaseValue := range switchCase.Values {
			s.jsln("case ", switchCaseValue, ":")
		}
		if len(switchCase.Values) == 0 {
			s.jsln("default:")
		}
		s.indentLevels++
		s.walk(switchCase.Body)
		s.jsln("break;")
		s.indentLevels--
	}
	s.indentLevels--
	s.jsln("}")
}

func (s *state) visitMsg(node *ast.MsgNode) {
	// If no bundle was provided, walk the message sub-nodes.
	if s.options.Messages == nil {
		s.visitMsgNode(node)
		return
	}

	// Look up the message in the bundle.
	var msg = s.options.Messages.Message(node.ID)
	if msg == nil {
		s.visitMsgNode(node)
		return
	}

	// Render each part.
	s.evalMsgParts(node, msg.Parts)
}

func (s *state) evalMsgParts(msgNode *ast.MsgNode, parts []soymsg.Part) {
	for _, part := range parts {
		switch part := part.(type) {

		case soymsg.RawTextPart:
			s.writeRawText([]byte(part.Text))

		case soymsg.PlaceholderPart:
			// Find the node corresponding to the placeholder, and walk it.
			var phnode = msgNode.Placeholder(part.Name)
			if phnode == nil {
				s.errorf("failed to find placeholder %q in %q",
					part.Name, soymsg.PlaceholderString(msgNode))
			}
			s.walk(phnode.Body)

		case soymsg.PluralPart:
			// Find the corresponding node for this part.
			child := s.findPluralNode(msgNode, part.VarName)

			s.jsln("switch (soy.$$pluralIndex(", child.Value, ")) {")
			s.indentLevels++

			for i, pluralPart := range part.Cases {
				s.jsln("case ", i, ":")
				s.indentLevels++
				s.evalMsgParts(msgNode, pluralPart.Parts)
				s.jsln("break;")
				s.indentLevels--
			}

			s.indentLevels--
			s.jsln("}")
		}
	}
}

func (s *state) findPluralNode(node *ast.MsgNode, pluralVarName string) *ast.MsgPluralNode {
	for _, plnode := range node.Body.Children() {
		if plnode, ok := plnode.(*ast.MsgPluralNode); ok && plnode.VarName == pluralVarName {
			return plnode
		}
	}
	s.errorf("failed to find placeholder %q in %v", pluralVarName, node.Body)
	panic("unreachable")
}

func (s *state) visitMsgNode(n ast.ParentNode) {
	for _, child := range n.Children() {
		switch child := child.(type) {
		case *ast.RawTextNode:
			s.walk(child)
		case *ast.MsgPlaceholderNode:
			s.walk(child.Body)
		case *ast.MsgPluralNode:
			s.walkPlural(child)
		}
	}
}

func (s *state) walkPlural(n *ast.MsgPluralNode) {
	s.jsln("switch (", n.Value, ") {")
	s.indentLevels++
	for _, pluralCase := range n.Cases {
		s.jsln("case ", pluralCase.Value, ":")
		s.indentLevels++
		s.visitMsgNode(pluralCase.Body)
		s.jsln("break;")
		s.indentLevels--
	}
	{
		s.jsln("default:")
		s.indentLevels++
		s.visitMsgNode(n.Default)
		s.indentLevels--
	}
	s.indentLevels--
	s.jsln("}")
}

// visitGlobal constructs a primitive node from its value and uses walk to
// render the right thing.
func (s *state) visitGlobal(node *ast.GlobalNode) {
	s.walk(s.nodeFromValue(node.Pos, node.Value))
}

func (s *state) nodeFromValue(pos ast.Pos, val data.Value) ast.Node {
	switch val := val.(type) {
	case data.Undefined:
		s.errorf("undefined value can not be converted to node")
	case data.Null:
		return &ast.NullNode{pos}
	case data.Bool:
		return &ast.BoolNode{pos, bool(val)}
	case data.Int:
		return &ast.IntNode{pos, int64(val)}
	case data.Float:
		return &ast.FloatNode{pos, float64(val)}
	case data.String:
		return &ast.StringNode{pos, "<unused>", string(val)}
	case data.List:
		var items = make([]ast.Node, len(val))
		for i, item := range val {
			items[i] = s.nodeFromValue(pos, item)
		}
		return &ast.ListLiteralNode{pos, items}
	case data.Map:
		var items = make(map[string]ast.Node, len(val))
		for k, v := range val {
			items[k] = s.nodeFromValue(pos, v)
		}
		return &ast.MapLiteralNode{pos, items}
	}
	panic("unreachable")
}

func (s *state) writeRawText(text []byte) {
	s.indent()
	s.js(s.bufferName, " += '")
	template.JSEscape(s.wr, text)
	s.js("';\n")
}

// block renders the given node to a temporary buffer and returns the string.
func (s *state) block(node ast.Node) string {
	var buf bytes.Buffer
	(&state{
	    wr: &buf,
	    scope: s.scope,
	    options: s.options,
	    funcsCalled: s.funcsCalled,
	    funcsInFile: s.funcsInFile,
	}).walk(node)
	return buf.String()
}

func (s *state) op(symbol string, node ast.ParentNode) {
	var children = node.Children()
	s.js("((", children[0], ") ", symbol, " (", children[1], "))")
}

func (s *state) indent() {
	for i := 0; i < s.indentLevels; i++ {
		s.wr.Write([]byte("  "))
	}
}

func (s *state) js(args ...interface{}) {
	for _, arg := range args {
		switch arg := arg.(type) {
		case string:
			s.wr.Write([]byte(arg))
		case ast.Node:
			s.walk(arg)
		default:
			fmt.Fprintf(s.wr, "%v", arg)
		}
	}
}

func (s *state) jsln(args ...interface{}) {
	s.indent()
	s.js(args...)
	s.wr.Write([]byte("\n"))
}
