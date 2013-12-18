package parse

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
func (t *Tree) itemList(until itemType) *ListNode {
	var (
		list = newList(0) // todo
	)
	for {
		// Two ways to end a list:
		// 1. We found the until token (e.g. EOF)
		var token = t.next()
		if token.typ == until {
			return list
		}

		// 2. The until token is a command end, e.g. {/template}
		var token2 = t.next()
		if token.typ == itemLeftDelim && token2.typ == until {
			t.expect(itemRightDelim, "close tag")
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
		return newText(token.pos, token.val)
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

// beginTag parses the contents of delimiters (within a template)
// The contents could be a command, variable, function call, expression, etc.
// { already read.
func (t *Tree) beginTag() Node {
	switch token := t.next(); token.typ {
	case itemNamespace:
		return t.parseNamespace(token)
	case itemTemplate:
		return t.parseTemplate(token)
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

func (t *Tree) parseSoyDoc(token item) Node {
	const ctx = "soydoc"
	// TODO: params
	var text = t.expect(itemText, ctx)
	t.expect(itemSoyDocEnd, ctx)
	return newSoyDoc(token.pos, text.val)
}

func (t *Tree) parseNamespace(token item) Node {
	const ctx = "namespace"
	var id = t.expect(itemIdent, ctx)
	t.expect(itemRightDelim, ctx)
	return newNamespace(token.pos, id.val)
}

func (t *Tree) parseTemplate(token item) Node {
	const ctx = "template"
	var id = t.expect(itemIdent, ctx)
	t.expect(itemRightDelim, ctx)
	tmpl := newTemplate(token.pos, id.val)
	tmpl.Body = t.itemList(itemTemplateEnd)
	return tmpl
}

// // requireNamespace return the namespace name
// func (t *Tree) requireNamespace() string {
// 	const ctx = "namespace declaration"
// 	t.expect(itemLeftDelim, ctx)
// 	t.expect(itemNamespace, ctx)
// 	ident := t.expect(itemIdent, ctx)
// 	t.expect(itemRightDelim, ctx)
// 	return ident.value
// }

// Expressions ----------

var precedence = map[itemType]int{
	itemNot:   6,
	itemMul:   5,
	itemDiv:   5,
	itemMod:   5,
	itemAdd:   4,
	itemSub:   4,
	itemEq:    3,
	itemNotEq: 3,
	itemGt:    3,
	itemGte:   3,
	itemLt:    3,
	itemLte:   3,
	itemOr:    2,
	itemAnd:   1,
	itemElvis: 0,
}

// parseExpr parses an arbitrary expression involving function applications and
// arithmetic.
// test: ((2*4-6/3)*(3*5+8/4))-(2+3)
// test: not $var and (isFirst($foo) or $x - 5 > 3)
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
	case itemNot:
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
		s, err := strconv.Unquote(tok.val)
		if err != nil {
			t.error(err)
		}
		return &StringNode{tok.pos, tok.val, s}
	case itemList:
		panic("unimplemented")
	case itemMap:
		panic("unimplemented")
	case itemVariable:
		return &VariableNode{tok.pos, tok.val}
	case itemIdent:
		// this is a function call.  get all the arguments.
		node := &FunctionNode{tok.pos, tok.val, nil}
		t.expect(itemLeftParen, "expression: function call")
		for {
			switch tok := t.next(); tok.typ {
			case itemRightParen:
				return node
			case eof:
				t.errorf("unexpected eof reading function params")
			default:
				if !isValue(tok) {
					t.errorf("expected value type in function params")
				}
				node.Args = append(node.Args, t.newValueNode(tok))
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
