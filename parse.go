package soy

import (
	"fmt"
	"runtime"
	"strconv"
)

// Tofu aggregates an application's soy files, providing convenient access.
type Tofu struct{}

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
	case itemVariable:
		return t.parseVariable(token)
	}
	t.errorf("not implemented")
	return nil
}

func (t *Tree) parseVariable(token item) Node {
	const ctx = "variable"
	// TODO: directives
	t.expect(itemRightDelim, ctx)
	return newVariable(token.pos, token.val)
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

// term:
//	literal (number, string, nil, boolean)
//	function (identifier)
//	.
//	$
//	'(' pipeline ')'
// A term is a simple "expression".
// A nil return means the next item is not a term.
func (t *Tree) term() Node {
	switch token := t.next(); token.typ {
	case itemError:
		t.errorf("%s", token.val)
	case itemIdent:
		return NewIdent(token.val).SetPos(token.pos)
	case itemVariable:
	case itemBool:
		return newBool(token.pos, token.val == "true")
	// case itemCharConstant, itemComplex, itemNumber:
	// 	number, err := newNumber(token.pos, token.val, token.typ)
	// 	if err != nil {
	// 		t.error(err)
	// 	}
	// 	return number
	case itemString:
		s, err := strconv.Unquote(token.val)
		if err != nil {
			t.error(err)
		}
		return newString(token.pos, token.val, s)
	}
	t.backup()
	return nil
}

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
