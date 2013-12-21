package parse

// TODO: How to leave out ListNode when its only one item

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// Tree is the parsed representation of a single soy file.
type Tree struct {
	Name string
	Root *ListNode // top-level root of the tree.

	// Parsing only; cleared after parse.
	funcs     []map[string]interface{}
	lex       *lexer
	token     [3]item // three-token lookahead for parser.
	peekCount int
	vars      []string // variables defined at the moment.
	text      string
}

// Template is an individual template in the soy file
type Template struct {
	Name       string // fully qualified template name
	ParamNames []string
}

func Parse(name, text string, funcs ...map[string]interface{}) (f *Tree, err error) {
	f = New(name)
	_, err = f.Parse(text, funcs...)
	return
}

// New allocates a new parse tree with the given name.
func New(name string, funcs ...map[string]interface{}) *Tree {
	return &Tree{
		Name:  name,
		funcs: funcs,
	}
}

func (t *Tree) Parse(text string, funcs ...map[string]interface{}) (tree *Tree, err error) {
	defer t.recover(&err)
	t.startParse(funcs, lex(t.Name, text))
	t.text = text
	t.parse()
	t.stopParse()
	return t, nil
}

// parse parses the soy template.
// At the top level, only Namespace, SoyDoc, and Template nodes are allowed
func (t *Tree) parse() {
	t.Root = t.itemList(itemEOF)
}

// itemList:
//	textOrTag*
// Terminates when it comes across the given end tag.
func (t *Tree) itemList(until ...itemType) *ListNode {
	var (
		list = newList(0) // todo
	)
	for {
		// Two ways to end a list:
		// 1. We found the until token (e.g. EOF)
		var token = t.next()
		if isOneOf(token.typ, until) {
			return list
		}

		// 2. The until token is a command, e.g. {else} {/template}
		var token2 = t.next()
		if token.typ == itemLeftDelim && isOneOf(token2.typ, until) {
			return list
		}

		// Not exiting, so backup two tokens ago.
		t.backup2(token)
		list.append(t.textOrTag())
	}
	return list
}

func (t *Tree) textOrTag() Node {
	switch token := t.next(); token.typ {
	case itemText:
		return &RawTextNode{token.pos, rawtext(token.val)}
	case itemLeftDelim:
		return t.beginTag()
	case itemSoyDocStart:
		return t.parseSoyDoc(token)
	default:
		t.unexpected(token, "input")
	}
	return nil
}

func (t *Tree) parseSoydoc() Node {
	t.errorf("not implemented")
	return nil
}

var specialChars = map[itemType]string{
	itemTab:            "\t",
	itemNewline:        "\n",
	itemCarriageReturn: "\r",
	itemLeftBrace:      "{",
	itemRightBrace:     "}",
}

// beginTag parses the contents of delimiters (within a template)
// The contents could be a command, variable, function call, expression, etc.
// { already read.
func (t *Tree) beginTag() Node {
	switch token := t.next(); token.typ {
	case itemNamespace:
		return t.parseNamespace(token)
	case itemTemplate:
		return t.parseTemplate(token)
	case itemIf:
		return t.parseIf(token)
	case itemMsg:
		return t.parseMsg(token)
	case itemForeach, itemFor:
		return t.parseFor(token)
	case itemSwitch:
		return t.parseSwitch(token)
	case itemTab, itemNewline, itemCarriageReturn, itemLeftBrace, itemRightBrace:
		t.expect(itemRightDelim, "special char")
		return newText(token.pos, specialChars[token.typ])
	case itemIdent, itemVariable, itemNull, itemBool, itemFloat, itemInteger, itemString, itemList, itemMap, itemNot:
		// print is implicit, so the tag may also begin with any value type, or the
		// "not" operator.
		t.backup()
		n := &PrintNode{token.pos, t.parseExpr(0)}
		t.expect(itemRightDelim, "print")
		return n
	default:
		t.errorf("not implemented: %#v", token)
	}
	return nil
}

// "switch" has just been read.
func (t *Tree) parseSwitch(token item) Node {
	const ctx = "switch"
	var switchValue = t.parseExpr(0)
	t.expect(itemRightDelim, ctx)
	t.expect(itemLeftDelim, "switch")
	var cases []*SwitchCaseNode
	for {
		switch tok := t.next(); tok.typ {
		case itemCase, itemDefault:
			cases = append(cases, t.parseCase(tok))
		case itemSwitchEnd:
			t.expect(itemRightDelim, ctx)
			return &SwitchNode{token.pos, switchValue, cases}
		}
	}
}

// "case" has just been read.
func (t *Tree) parseCase(token item) *SwitchCaseNode {
	var values []Node
	for {
		if token.typ != itemDefault {
			values = append(values, t.parseExpr(0))
		}
		switch tok := t.next(); tok.typ {
		case itemComma:
			continue
		case itemRightDelim:
			var body = t.itemList(itemCase, itemDefault, itemSwitchEnd)
			t.backup()
			return &SwitchCaseNode{token.pos, values, body}
		default:
			t.errorf("unexpected item when parsing case: %v", tok)
		}
	}
}

// "for" or "foreach" has just been read.
func (t *Tree) parseFor(token item) Node {
	var ctx = token.val
	// for and foreach have the same syntax, differing only in the requirement they impose:
	// - for requires the collection to be a function call to "range"
	// - foreach requires the collection to be a variable reference.
	var vartoken = t.expect(itemVariable, ctx)
	var intoken = t.expect(itemIdent, ctx)
	if intoken.val != "in" {
		t.errorf("expected 'in' in for")
	}

	// get the collection to iterate through and enforce the requirements
	var collection = t.parseExpr(0)
	t.expect(itemRightDelim, "foreach")
	switch token.typ {
	case itemFor:
		f, ok := collection.(*FunctionNode)
		if !ok || f.Name != "range" {
			t.errorf("for: expected to iterate through range()")
		}
	case itemForeach:
		if _, ok := collection.(*DataRefNode); !ok {
			t.errorf("foreach: expected to iterate through a variable")
		}
	}

	var body = t.itemList(itemIfempty, itemForeachEnd, itemForEnd)
	t.backup()
	var ifempty Node
	if t.next().typ == itemIfempty {
		t.expect(itemRightDelim, "ifempty")
		ifempty = t.itemList(itemForeachEnd, itemForEnd)
	}
	t.expect(itemRightDelim, "/foreach")
	return &ForNode{token.pos, vartoken.val, collection, body, ifempty}
}

// "foreach" has just been read.
func (t *Tree) parseForeach(token item) Node {
	var vartoken = t.expect(itemVariable, "foreach")
	var intoken = t.expect(itemIdent, "foreach")
	if intoken.val != "in" {
		t.errorf("expected 'in' in for")
	}
	var collection = t.expect(itemVariable, "foreach")
	t.expect(itemRightDelim, "foreach")

	var body = t.itemList(itemIfempty, itemForeachEnd)
	t.backup()
	var ifempty Node
	if t.next().typ == itemIfempty {
		t.expect(itemRightDelim, "ifempty")
		ifempty = t.itemList(itemForeachEnd)
	}
	t.expect(itemRightDelim, "/foreach")
	return &ForNode{token.pos, vartoken.val,
		&DataRefNode{collection.pos, collection.val, nil}, body, ifempty}
}

// "if" has just been read.
func (t *Tree) parseIf(token item) Node {
	var conds []*IfCondNode
	var isElse = false
	for {
		var condExpr Node
		if !isElse {
			condExpr = t.parseExpr(0)
		}
		t.expect(itemRightDelim, "if")
		var body = t.itemList(itemElseif, itemElse, itemIfEnd)
		conds = append(conds, &IfCondNode{token.pos, condExpr, body})
		t.backup()
		switch t.next().typ {
		case itemElseif:
			// continue
		case itemElse:
			isElse = true
		case itemIfEnd:
			t.expect(itemRightDelim, "/if")
			return &IfNode{token.pos, conds}
		}
	}
}

func (t *Tree) parseSoyDoc(token item) Node {
	const ctx = "soydoc"
	// TODO: params
	var text = t.expect(itemText, ctx)
	t.expect(itemSoyDocEnd, ctx)
	return newSoyDoc(token.pos, text.val)
}

func inStringSlice(item string, group []string) bool {
	for _, x := range group {
		if x == item {
			return true
		}
	}
	return false
}

func (t *Tree) parseAttrs(allowedNames ...string) map[string]string {
	var result = make(map[string]string)
	for {
		switch tok := t.next(); tok.typ {
		case itemIdent:
			if !inStringSlice(tok.val, allowedNames) {
				t.errorf("unexpected attribute: %s", tok.val)
			}
			t.expect(itemEquals, "attribute")
			var attrval = t.expect(itemString, "attribute")
			var err error
			result[tok.val], err = strconv.Unquote(attrval.val)
			if err != nil {
				t.error(err)
			}
		case itemRightDelim:
			return result
		default:
			t.errorf("unexpected item parsing attributes: %v", tok)
		}
	}
}

// "msg" has just been read.
func (t *Tree) parseMsg(token item) Node {
	const ctx = "msg"
	var attrs = t.parseAttrs("desc", "meaning")
	var node = &MsgNode{token.pos, attrs["desc"], t.itemList(itemMsgEnd)}
	t.expect(itemRightDelim, ctx)
	return node
}

func (t *Tree) parseNamespace(token item) Node {
	const ctx = "namespace"
	var id = t.expect(itemIdent, ctx)
	t.expect(itemRightDelim, ctx)
	return newNamespace(token.pos, id.val)
}

func (t *Tree) parseTemplate(token item) Node {
	const ctx = "template"
	t.expect(itemDot, ctx)
	var id = t.expect(itemIdent, ctx)
	t.expect(itemRightDelim, ctx)
	tmpl := newTemplate(token.pos, id.val)
	tmpl.Body = t.itemList(itemTemplateEnd)
	t.expect(itemRightDelim, "template tag")
	return tmpl
}

// Expressions ----------

var precedence = map[itemType]int{
	itemNot:    6,
	itemNegate: 6,
	itemMul:    5,
	itemDiv:    5,
	itemMod:    5,
	itemAdd:    4,
	itemSub:    4,
	itemEq:     3,
	itemNotEq:  3,
	itemGt:     3,
	itemGte:    3,
	itemLt:     3,
	itemLte:    3,
	itemOr:     2,
	itemAnd:    1,
	itemElvis:  0,
}

// parseExpr parses an arbitrary expression involving function applications and
// arithmetic.
//
// For handling binary operators, we use the Precedence Climbing algorithm described in:
//   http://www.engr.mun.ca/~theo/Misc/exp_parsing.htm
func (t *Tree) parseExpr(prec int) Node {
	n := t.parseExprFirstTerm()
	var tok item
	for {
		tok = t.next()
		q := precedence[tok.typ]
		if !isBinaryOp(tok.typ) || q < prec {
			break
		}
		q++
		n = newBinaryOpNode(tok, n, t.parseExpr(q))
	}
	if prec == 0 && tok.typ == itemTernIf {
		return t.parseTernary(n)
	}
	t.backup()
	return n
}

// Primary ->   "(" Expr ")"
//            | u=UnaryOp PrecExpr(prec(u))
//            | FunctionCall | DataRef | Global | ListLiteral | MapLiteral | Primitive
func (t *Tree) parseExprFirstTerm() Node {
	switch tok := t.next(); {
	case isUnaryOp(tok):
		return newUnaryOpNode(tok, t.parseExpr(precedence[tok.typ]))
	case tok.typ == itemLeftParen:
		n := t.parseExpr(0)
		t.expect(itemRightParen, "expression")
		return n
	case isValue(tok):
		return t.newValueNode(tok)
	default:
		t.errorf("unexpected token %q", tok)
	}
	return nil
}

// DataRef ->  ( "$ij." Ident | "$ij?." Ident | DollarIdent )
//             (   DotIdent | QuestionDotIdent | DotIndex | QuestionDotIndex
//               | "[" Expr "]" | "?[" Expr "]" )*
// TODO: Injected data
func (t *Tree) parseDataRef(tok item) Node {
	var ref = &DataRefNode{tok.pos, tok.val[1:], nil}
	for {
		var accessNode Node
		switch tok := t.next(); tok.typ {
		case itemDot:
			accessNode = t.parseDataRefDot(tok, false)
		case itemQuestionDot:
			accessNode = t.parseDataRefDot(tok, true)
		case itemKey:
			accessNode = t.parseDataRefExpr(tok, false)
		case itemQuestionKey:
			accessNode = t.parseDataRefExpr(tok, true)
		default:
			t.backup()
			return ref
		}
		ref.Access = append(ref.Access, accessNode)
	}
}

func (t *Tree) parseDataRefDot(start item, nullsafe bool) Node {
	switch tok := t.next(); tok.typ {
	case itemInteger:
		index, err := strconv.ParseInt(tok.val, 10, 0)
		if err != nil {
			t.error(err)
		}
		return &DataRefIndexNode{start.pos, nullsafe, int(index)}
	case itemIdent:
		return &DataRefKeyNode{start.pos, nullsafe, tok.val}
	}
	t.errorf("unexpected type in data reference")
	return nil
}

func (t *Tree) parseDataRefExpr(start item, nullsafe bool) Node {
	var expr = t.parseExpr(0)
	t.expect(itemKeyEnd, "dataref")
	return &DataRefExprNode{start.pos, nullsafe, expr}
}

//  ListLiteral -> "[" [ Expr ( "," Expr )* [ "," ] ] "]"
func (t *Tree) parseListLiteral(token item) Node {
	panic("unimplemented")
}

// MapLiteral -> "[" ( ":" | Expr ":" Expr ( "," Expr ":" Expr )* [ "," ] ) "]"
func (t *Tree) parseMapLiteral(token item) Node {
	panic("unimplemented")
}

// parseTernary parses the ternary operator within an expression.
// itemTernIf has already been read, and the condition is provided.
func (t *Tree) parseTernary(cond Node) Node {
	n1 := t.parseExpr(0)
	t.expect(itemTernElse, "ternary")
	n2 := t.parseExpr(0)
	result := &TernNode{cond.Position(), cond, n1, n2}
	if t.peek().typ == itemTernElse {
		t.next()
		return t.parseTernary(result)
	}
	return result
}

func isBinaryOp(typ itemType) bool {
	switch typ {
	case itemMul, itemDiv, itemMod,
		itemAdd, itemSub,
		itemEq, itemNotEq, itemGt, itemGte, itemLt, itemLte,
		itemOr, itemAnd, itemElvis:
		return true
	}
	return false
}

func isUnaryOp(t item) bool {
	switch t.typ {
	case itemNot, itemNegate:
		return true
	}
	return false
}

func isValue(t item) bool {
	switch t.typ {
	case itemNull, itemBool, itemInteger, itemFloat, itemVariable, itemString:
		return true
	case itemIdent:
		return true // function application returns a value
	}
	return false
}

func op(n binaryOpNode, name string) binaryOpNode {
	n.Name = name
	return n
}

func newBinaryOpNode(t item, n1, n2 Node) Node {
	var bin = binaryOpNode{"", t.pos, n1, n2}
	switch t.typ {
	case itemMul:
		return &MulNode{op(bin, "*")}
	case itemDiv:
		return &DivNode{op(bin, "/")}
	case itemMod:
		return &ModNode{op(bin, "%")}
	case itemAdd:
		return &AddNode{op(bin, "+")}
	case itemSub:
		return &SubNode{op(bin, "-")}
	case itemEq:
		return &EqNode{op(bin, "=")}
	case itemNotEq:
		return &NotEqNode{op(bin, "!=")}
	case itemGt:
		return &GtNode{op(bin, ">")}
	case itemGte:
		return &GteNode{op(bin, ">=")}
	case itemLt:
		return &LtNode{op(bin, "<")}
	case itemLte:
		return &LteNode{op(bin, "<=")}
	case itemOr:
		return &OrNode{op(bin, "or")}
	case itemAnd:
		return &AndNode{op(bin, "and")}
	case itemElvis:
		return &ElvisNode{op(bin, "?:")}
	}
	panic("unimplemented")
}

func newUnaryOpNode(t item, n1 Node) Node {
	switch t.typ {
	case itemNot:
		return &NotNode{t.pos, n1}
	case itemNegate:
		return &NegateNode{t.pos, n1}
	}
	panic("unreachable")
}

func (t *Tree) newValueNode(tok item) Node {
	switch tok.typ {
	case itemNull:
		return &NullNode{tok.pos}
	case itemBool:
		return &BoolNode{tok.pos, tok.val == "true"}
	case itemInteger:
		var base = 10
		if strings.HasPrefix(tok.val, "0x") {
			base = 16
		}
		value, err := strconv.ParseInt(tok.val, base, 64)
		if err != nil {
			t.error(err)
		}
		return &IntNode{tok.pos, value}
	case itemFloat:
		// todo: support scientific notation e.g. 6.02e23
		value, err := strconv.ParseFloat(tok.val, 64)
		if err != nil {
			t.error(err)
		}
		return &FloatNode{tok.pos, value}
	case itemString:
		s, err := unquoteString(tok.val)
		if err != nil {
			t.errorf("error unquoting %s: %s", tok.val, err)
		}
		return &StringNode{tok.pos, s}
	case itemList:
		panic("unimplemented")
	case itemMap:
		panic("unimplemented")
	case itemVariable:
		return t.parseDataRef(tok)
	case itemIdent:
		// this is a function call.  get all the arguments.
		node := &FunctionNode{tok.pos, tok.val, nil}
		t.expect(itemLeftParen, "expression: function call")
		for {
			node.Args = append(node.Args, t.parseExpr(0))
			switch tok := t.next(); tok.typ {
			case itemComma:
				// continue to get the next arg
			case itemRightParen:
				return node // all done
			case eof:
				t.errorf("unexpected eof reading function params")
			default:
				t.errorf("unexpected %v reading function params", tok)
			}
		}
	}
	panic("unreachable")
}

// Helpers ----------

// startParse initializes the parser, using the lexer.
func (t *Tree) startParse(funcs []map[string]interface{}, lex *lexer) {
	t.Root = nil
	t.lex = lex
	t.funcs = funcs
}

// stopParse terminates parsing.
func (t *Tree) stopParse() {
	t.lex = nil
	t.vars = nil
	t.funcs = nil
}

// next returns the next token.
func (t *Tree) next() item {
	if t.peekCount > 0 {
		t.peekCount--
	} else {
		t.token[0] = t.lex.nextItem()
	}
	return t.token[t.peekCount]
}

// backup backs the input stream up one token.
func (t *Tree) backup() {
	t.peekCount++
}

// backup2 backs the input stream up two tokens.
// The zeroth token is already there.
func (t *Tree) backup2(t1 item) {
	t.token[1] = t1
	t.peekCount = 2
}

// backup3 backs the input stream up three tokens
// The zeroth token is already there.
func (t *Tree) backup3(t2, t1 item) { // Reverse order: we're pushing back.
	t.token[1] = t1
	t.token[2] = t2
	t.peekCount = 3
}

// peek returns but does not consume the next token.
func (t *Tree) peek() item {
	if t.peekCount > 0 {
		return t.token[t.peekCount-1]
	}
	t.peekCount = 1
	t.token[0] = t.lex.nextItem()
	return t.token[0]
}

// recover is the handler that turns panics into returns from the top level of Parse.
func (t *Tree) recover(errp *error) {
	e := recover()
	if e != nil {
		if _, ok := e.(runtime.Error); ok {
			panic(e)
		}
		if t != nil {
			t.stopParse()
		}
		if str, ok := e.(string); ok {
			*errp = errors.New(str)
			return
		}
		*errp = e.(error)
	}
	return
}

// nextNonSpace returns the next non-space token.
func (t *Tree) nextNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	return token
}

// peekNonSpace returns but does not consume the next non-space token.
func (t *Tree) peekNonSpace() (token item) {
	for {
		token = t.next()
		if token.typ != itemSpace {
			break
		}
	}
	t.backup()
	return token
}

// expect consumes the next token and guarantees it has the required type.
func (t *Tree) expect(expected itemType, context string) item {
	token := t.next()
	if token.typ != expected {
		t.unexpected(token, context)
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Tree) unexpected(token item, context string) {
	t.errorf("unexpected %#v in %s", token, context)
}

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("template: %s:%d: %s", t.Name, t.lex.lineNumber(), format)
	panic(fmt.Errorf(format, args...))
}

// error terminates processing.
func (t *Tree) error(err error) {
	t.errorf("%s", err)
}

func isOneOf(tocheck itemType, against []itemType) bool {
	for _, x := range against {
		if tocheck == x {
			return true
		}
	}
	return false
}
