package parse

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/robfig/soy/data"
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
	text      string                // the full text of the soy file
	namespace string                // the namespace found in the soy file being parsed.
	aliases   map[string]string     // map from alias to namespace e.g. {"c": "a.b.c"}
	globals   map[string]data.Value // global (compile-time constants) values by name
}

// New allocates a new parse tree with the given name.
// Any globals used in the soy files must be provided ahead of time.
func New(name string, globals map[string]data.Value) *Tree {
	return &Tree{
		Name:    name,
		aliases: make(map[string]string),
		globals: globals,
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
	var list *ListNode
	for {
		var token = t.next()
		if list == nil {
			list = newList(token.pos)
		}
		var node, halt = t.textOrTag(token, until)
		if halt {
			return list
		}
		if node != nil {
			list.append(node)
		}
	}
}

// textOrTag reads raw text or recognizes the start of tags until the end tag.
func (t *Tree) textOrTag(token item, until []itemType) (node Node, halt bool) {
	var seenComment = token.typ == itemComment
	for token.typ == itemComment {
		token = t.next() // skip any comments
	}

	// Two ways to end a list:
	// 1. We found the until token (e.g. EOF)
	if isOneOf(token.typ, until) {
		return nil, true
	}

	// 2. The until token is a command, e.g. {else} {/template}
	var token2 = t.next()
	if token.typ == itemLeftDelim && isOneOf(token2.typ, until) {
		return nil, true
	}

	t.backup()
	switch token.typ {
	case itemText:
		var text = token.val
		var next item
		for {
			next = t.next()
			if next.typ != itemText {
				break
			}
			text += next.val
		}
		t.backup()
		var textvalue = rawtext(text, seenComment, next.typ == itemComment)
		if len(textvalue) == 0 {
			return nil, false
		}
		return &RawTextNode{token.pos, textvalue}, false
	case itemLeftDelim:
		return t.beginTag(), false
	case itemSoyDocStart:
		return t.parseSoyDoc(token), false
	default:
		t.unexpected(token, "input")
	}
	return nil, false
}

var specialChars = map[itemType]string{
	itemNil:            "",
	itemSpace:          " ",
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
	case itemCall:
		return t.parseCall(token)
	case itemLiteral:
		t.expect(itemRightDelim, "literal")
		literalText := t.expect(itemText, "literal")
		n := &RawTextNode{literalText.pos, []byte(literalText.val)}
		t.expect(itemLeftDelim, "literal")
		t.expect(itemLiteralEnd, "literal")
		t.expect(itemRightDelim, "literal")
		return n
	case itemCss:
		return t.parseCss(token)
	case itemLog:
		t.expect(itemRightDelim, "log")
		logBody := t.itemList(itemLogEnd)
		t.expect(itemRightDelim, "log")
		return &LogNode{token.pos, logBody}
	case itemDebugger:
		t.expect(itemRightDelim, "debugger")
		return &DebuggerNode{token.pos}
	case itemLet:
		return t.parseLet(token)
	case itemAlias:
		t.parseAlias(token)
		return nil
	case itemNil, itemSpace, itemTab, itemNewline, itemCarriageReturn, itemLeftBrace, itemRightBrace:
		t.expect(itemRightDelim, "special char")
		return newText(token.pos, specialChars[token.typ])
	case itemIdent, itemDollarIdent, itemNull, itemBool, itemFloat, itemInteger, itemString, itemNegate, itemNot, itemLeftBracket:
		// print is implicit, so the tag may also begin with any value type or unary op.
		t.backup()
		fallthrough
	case itemPrint:
		return t.parsePrint(token)
	default:
		t.errorf("not implemented: %#v", token)
	}
	return nil
}

// print has just been read (or inferred)
func (t *Tree) parsePrint(token item) Node {
	var expr = t.parseExpr(0)
	var directives []*PrintDirectiveNode
	for {
		switch tok := t.next(); tok.typ {
		case itemRightDelim:
			return &PrintNode{token.pos, expr, directives}
		case itemPipe:
			// read the directive name and see if there are arguments
			var id = t.expect(itemIdent, "print directive")
			var args []Node
			for {
				// each argument is preceeded by a colon (first arg) or comma (subsequent)
				switch next := t.next(); next.typ {
				case itemColon, itemComma:
					args = append(args, t.parseExpr(0))
					continue
				}
				t.backup()
				directives = append(directives, &PrintDirectiveNode{tok.pos, id.val, args})
				break
			}
		default:
			t.unexpected(tok, "print. (expected '|' or '}')")
		}
	}
}

// parseAlias updates the Tree with the given alias.
// Aliases are applied at immediately (at parse time) to new nodes.
// "alias" has just been read.
func (t *Tree) parseAlias(token item) {
	var name = t.expect(itemIdent, "alias").val
	var lastSegment = name
	for {
		switch next := t.next(); next.typ {
		case itemDotIdent:
			name += next.val
			lastSegment = next.val[1:]
		case itemRightDelim:
			t.aliases[lastSegment] = name
			return
		default:
			t.unexpected(next, "alias. (expected '}')")
		}
	}
}

// "let" has just been read.
func (t *Tree) parseLet(token item) Node {
	var name = t.expect(itemDollarIdent, "let")
	switch next := t.next(); next.typ {
	case itemColon:
		var node = &LetValueNode{token.pos, name.val[1:], t.parseExpr(0)}
		t.expect(itemRightDelimEnd, "let")
		return node
	case itemRightDelim:
		var node = &LetContentNode{token.pos, name.val[1:], t.itemList(itemLetEnd)}
		t.expect(itemRightDelim, "let")
		return node
	default:
		t.unexpected(next, "{let}")
	}
	panic("unreachable")
}

// "css" has just been read.
func (t *Tree) parseCss(token item) Node {
	var cmdText = t.expect(itemText, "css")
	t.expect(itemRightDelim, "css")
	var lastComma = strings.LastIndex(cmdText.val, ",")
	if lastComma == -1 {
		return &CssNode{token.pos, nil, strings.TrimSpace(cmdText.val)}
	}
	var exprText = strings.TrimSpace(cmdText.val[:lastComma])
	return &CssNode{
		token.pos,
		t.parseQuotedExpr(exprText),
		strings.TrimSpace(cmdText.val[lastComma+1:]),
	}
}

// "call" has just been read.
func (t *Tree) parseCall(token item) Node {
	var templateName string
	switch tok := t.next(); tok.typ {
	case itemDotIdent:
		templateName = tok.val
	case itemIdent:
		// this ident could either be {call fully.qualified.name} or attributes.
		switch tok2 := t.next(); tok2.typ {
		case itemDotIdent:
			templateName = tok.val + tok2.val
			for tokn := t.next(); tokn.typ == itemDotIdent; tokn = t.next() {
				templateName += tokn.val
			}
			t.backup()
		default:
			t.backup2(tok)
		}
	default:
		t.backup()
	}
	attrs := t.parseAttrs("name", "data")

	if templateName == "" {
		templateName = attrs["name"]
	}
	if templateName == "" {
		t.errorf("call: template name not found")
	}

	// If it's not a fully qualified template name, apply the namespace or aliases
	if templateName[0] == '.' {
		templateName = t.namespace + templateName
	} else if dot := strings.Index(templateName, "."); dot != -1 {
		if alias, ok := t.aliases[templateName[:dot]]; ok {
			templateName = alias + templateName[dot:]
		}
	}

	var allData = false
	var dataNode Node = nil
	if data, ok := attrs["data"]; ok {
		if data == "all" {
			allData = true
		} else {
			dataNode = t.parseQuotedExpr(data)
		}
	}

	switch tok := t.next(); tok.typ {
	case itemRightDelimEnd:
		return &CallNode{token.pos, templateName, allData, dataNode, nil}
	case itemRightDelim:
		body := t.parseCallParams()
		t.expect(itemLeftDelim, "call")
		t.expect(itemCallEnd, "call")
		t.expect(itemRightDelim, "call")
		return &CallNode{token.pos, templateName, allData, dataNode, body}
	default:
		t.unexpected(tok, "error scanning {call}")
	}
	panic("unreachable")
}

// parseCallParams collects a list of call params, of which there are many
// different forms:
// {param a: 'expr'/}
// {param a}expr{/param}
// {param key="a" value="'expr'"/}
// {param key="a"}expr{/param}
// The closing delimiter of the {call} has just been read.
func (t *Tree) parseCallParams() []Node {
	var params []Node
	for {
		var (
			key   string
			value Node
		)

		var initial = t.nextNonComment()
		for initial.typ == itemText {
			// content is not allowed outside a param, but it's ok if it's a comment.
			// see if anything is left after running it through rawtext()
			var text = rawtext(initial.val, true, true)
			if len(text) != 0 {
				t.unexpected(initial, "{call}, in between {param}'s (orphan content)")
			}
			initial = t.nextNonComment()
		}
		if initial.typ != itemLeftDelim {
			t.unexpected(initial, "param list (expected '{')")
		}

		var cmd = t.next()
		if cmd.typ == itemCallEnd {
			t.backup2(initial)
			return params
		}
		if cmd.typ != itemParam {
			t.errorf("expected param declaration")
		}

		var firstIdent = t.expect(itemIdent, "param")
		switch tok := t.next(); tok.typ {
		case itemColon:
			key = firstIdent.val
			value = t.parseExpr(0)
			t.expect(itemRightDelimEnd, "param")
			params = append(params, &CallParamValueNode{initial.pos, key, value})
			continue
		case itemRightDelim:
			key = firstIdent.val
			value = t.itemList(itemParamEnd)
			t.expect(itemRightDelim, "param")
			params = append(params, &CallParamContentNode{initial.pos, key, value})
			continue
		case itemIdent:
			key = firstIdent.val
			t.backup()
		case itemEquals:
			t.backup2(firstIdent)
		default:
			t.unexpected(tok, "param. (expected ':', '}', or '=')")
		}

		attrs := t.parseAttrs("key", "value", "kind")
		var ok bool
		if key == "" {
			if key, ok = attrs["key"]; !ok {
				t.errorf("param key not found.  (attrs: %v)", attrs)
			}
		}
		var valueStr string
		if valueStr, ok = attrs["value"]; !ok {
			t.expect(itemRightDelim, "param")
			value = t.itemList(itemParamEnd)
			t.expect(itemRightDelim, "param")
			params = append(params, &CallParamContentNode{initial.pos, key, value})
		} else {
			value = t.parseQuotedExpr(valueStr)
			t.expect(itemRightDelimEnd, "param")
			params = append(params, &CallParamValueNode{initial.pos, key, value})
		}
	}
	return params
}

// "switch" has just been read.
func (t *Tree) parseSwitch(token item) Node {
	const ctx = "switch"
	var switchValue = t.parseExpr(0)
	t.expect(itemRightDelim, ctx)

	var cases []*SwitchCaseNode
	for {
		switch tok := t.next(); tok.typ {
		case itemLeftDelim:
		case itemText: // ignore spaces between tags. text is an error though.
			if allSpace(tok.val) {
				continue
			}
			t.unexpected(tok, "between switch cases")
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
			t.unexpected(tok, "switch case")
		}
	}
}

// "for" or "foreach" has just been read.
func (t *Tree) parseFor(token item) Node {
	var ctx = token.val
	// for and foreach have the same syntax, differing only in the requirement they impose:
	// - for requires the collection to be a function call to "range"
	// - foreach requires the collection to be a variable reference.
	var vartoken = t.expect(itemDollarIdent, ctx)
	var intoken = t.expect(itemIdent, ctx)
	if intoken.val != "in" {
		t.unexpected(intoken, "for loop (expected 'in')")
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
	return &ForNode{token.pos, vartoken.val[1:], collection, body, ifempty}
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
	var params []*SoyDocParamNode
	for {
		var optional = false
		switch next := t.next(); next.typ {
		case itemText:
			// ignore
		case itemSoyDocOptionalParam:
			optional = true
			fallthrough
		case itemSoyDocParam:
			var ident = t.expect(itemIdent, "soydoc param")
			params = append(params, &SoyDocParamNode{next.pos, ident.val, optional})
		case itemSoyDocEnd:
			return &SoyDocNode{token.pos, params}
		default:
			t.unexpected(next, "soydoc")
		}
	}
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
				t.unexpected(tok, fmt.Sprintf("attributes. allowed: %v", allowedNames))
			}
			t.expect(itemEquals, "attribute")
			var attrval = t.expect(itemString, "attribute")
			var err error
			result[tok.val], err = strconv.Unquote(attrval.val)
			if err != nil {
				t.error(err)
			}
		case itemRightDelim, itemRightDelimEnd:
			t.backup()
			return result
		default:
			t.unexpected(tok, "attributes")
		}
	}
}

// "msg" has just been read.
func (t *Tree) parseMsg(token item) Node {
	const ctx = "msg"
	var attrs = t.parseAttrs("desc", "meaning", "hidden")
	if _, ok := attrs["desc"]; !ok {
		t.errorf("Tag 'msg' must have a 'desc' attribute")
	}
	t.expect(itemRightDelim, ctx)
	var node = &MsgNode{token.pos, attrs["desc"], t.itemList(itemMsgEnd)}
	t.expect(itemRightDelim, ctx)
	return node
}

func (t *Tree) parseNamespace(token item) Node {
	if t.namespace != "" {
		t.errorf("file may have only one namespace declaration")
	}
	const ctx = "namespace"
	var name = t.expect(itemIdent, ctx).val
	for {
		switch part := t.next(); part.typ {
		case itemDotIdent:
			name += part.val
		case itemRightDelim:
			t.namespace = name
			return newNamespace(token.pos, name)
		default:
			t.unexpected(part, "namespace")
		}
	}
}

func (t *Tree) parseTemplate(token item) Node {
	const ctx = "template"
	var id = t.expect(itemDotIdent, ctx)
	var attrs = t.parseAttrs("autoescape", "private")
	var autoescape = AutoescapeOn
	switch str, ok := attrs["autoescape"]; {
	case !ok:
	case str == "true":
	case str == "false":
		autoescape = AutoescapeOff
	case str == "contextual":
		autoescape = AutoescapeContextual
	default:
		t.errorf(`expected "true", "false", or "contextual" for autoescape, got %v`, str)
	}
	var private = t.boolAttr(attrs, "private", false)
	t.expect(itemRightDelim, ctx)
	tmpl := &TemplateNode{
		token.pos,
		t.namespace + id.val,
		t.itemList(itemTemplateEnd),
		autoescape,
		private,
	}
	t.expect(itemRightDelim, "template tag")
	return tmpl
}

// Expressions ----------

func ParseExpr(str string) (node Node, err error) {
	var t = &Tree{lex: lexExpr("", str)}
	defer t.recover(&err)
	node = t.parseExpr(0)
	return
}

// boolAttr returns a boolean value from the given attribute map.
func (t *Tree) boolAttr(attrs map[string]string, key string, defaultValue bool) bool {
	switch str, ok := attrs[key]; {
	case !ok:
		return defaultValue
	case str == "true":
		return true
	case str == "false":
		return false
	default:
		t.errorf("expected 'true' or 'false', got %q", str)
	}
	panic("")
}

// parseQuotedExpr ignores the current lex/parse state and parses the given
// string as a standalone expression.
func (t *Tree) parseQuotedExpr(str string) Node {
	return (&Tree{
		lex:   lexExpr("", str),
		funcs: t.funcs,
	}).parseExpr(0)
}

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
		t.expect(itemRightParen, "soy expression")
		return n
	case isValue(tok):
		return t.newValueNode(tok)
	default:
		t.unexpected(tok, "soy expression")
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
		var nullsafe = 0
		switch tok := t.next(); tok.typ {
		case itemQuestionDotIdent:
			nullsafe = 1
			fallthrough
		case itemDotIdent:
			accessNode = &DataRefKeyNode{tok.pos, nullsafe == 1, tok.val[nullsafe+1:]}
		case itemQuestionDotIndex:
			nullsafe = 1
			fallthrough
		case itemDotIndex:
			index, err := strconv.ParseInt(tok.val[nullsafe+1:], 10, 0)
			if err != nil {
				t.error(err)
			}
			accessNode = &DataRefIndexNode{tok.pos, nullsafe == 1, int(index)}
		case itemQuestionKey:
			nullsafe = 1
			fallthrough
		case itemLeftBracket:
			accessNode = &DataRefExprNode{tok.pos, nullsafe == 1, t.parseExpr(0)}
			t.expect(itemRightBracket, "dataref")
		default:
			t.backup()
			return ref
		}
		ref.Access = append(ref.Access, accessNode)
	}
}

// "[" has just been read
func (t *Tree) parseListOrMap(token item) Node {
	// check if it's empty
	switch t.next().typ {
	case itemColon:
		t.expect(itemRightBracket, "map literal")
		return &MapLiteralNode{token.pos, nil}
	case itemRightBracket:
		return &ListLiteralNode{token.pos, nil}
	}
	t.backup()

	// parse the first expression, and check the subsequent delimiter
	var firstExpr = t.parseExpr(0)
	switch tok := t.next(); tok.typ {
	case itemColon:
		return t.parseMapLiteral(token, firstExpr)
	case itemComma:
		return t.parseListLiteral(token, firstExpr)
	case itemRightBracket:
		return &ListLiteralNode{token.pos, []Node{firstExpr}}
	default:
		t.unexpected(tok, "list/map literal")
	}
	return nil
}

// the first item in the list is provided.
// "," has just been read.
//  ListLiteral -> "[" [ Expr ( "," Expr )* [ "," ] ] "]"
func (t *Tree) parseListLiteral(first item, expr Node) Node {
	var items []Node
	items = append(items, expr)
	for {
		items = append(items, t.parseExpr(0))
		next := t.next()
		if next.typ == itemRightBracket {
			return &ListLiteralNode{first.pos, items}
		}
		if next.typ != itemComma {
			t.unexpected(next, "parsing value list")
		}
	}
}

// the first key in the map is provided
// ":" has just been read.
// MapLiteral -> "[" ( ":" | Expr ":" Expr ( "," Expr ":" Expr )* [ "," ] ) "]"
func (t *Tree) parseMapLiteral(first item, expr Node) Node {
	firstKey, ok := expr.(*StringNode)
	if !ok {
		t.errorf("expected a string as map key, got: %T", expr)
	}

	var items = make(map[string]Node)
	var key = firstKey.Value
	for {
		items[key] = t.parseExpr(0)
		next := t.next()
		if next.typ == itemRightBracket {
			return &MapLiteralNode{first.pos, items}
		}
		if next.typ != itemComma {
			t.unexpected(next, "map literal")
		}
		tok := t.expect(itemString, "map literal")
		var err error
		key, err = unquoteString(tok.val)
		if err != nil {
			t.error(err)
		}
		t.expect(itemColon, "map literal")
	}
}

// parseTernary parses the ternary operator within an expression.
// itemTernIf has already been read, and the condition is provided.
func (t *Tree) parseTernary(cond Node) Node {
	n1 := t.parseExpr(0)
	t.expect(itemColon, "ternary")
	n2 := t.parseExpr(0)
	result := &TernNode{cond.Position(), cond, n1, n2}
	if t.peek().typ == itemColon {
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
	case itemNull, itemBool, itemInteger, itemFloat, itemDollarIdent, itemString:
		return true
	case itemIdent:
		return true // function / global returns a value
	case itemLeftBracket:
		return true // list or map literal
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
	case itemLeftBracket:
		return t.parseListOrMap(tok)
	case itemDollarIdent:
		return t.parseDataRef(tok)
	case itemIdent:
		next := t.next()
		if next.typ != itemLeftParen {
			return t.newGlobalNode(tok, next)
		}
		return t.newFunctionNode(tok)
	}
	panic("unreachable")
}

func (t *Tree) newGlobalNode(tok, next item) Node {
	var name = tok.val
	for next.typ == itemDotIdent {
		name += next.val
		next = t.next()
	}
	t.backup()
	if value, ok := t.globals[name]; ok {
		return &GlobalNode{tok.pos, name, value}
	}
	t.errorf("global %q is undefined", name)
	return nil
}

func (t *Tree) newFunctionNode(tok item) Node {
	node := &FunctionNode{tok.pos, tok.val, nil}
	if t.peek().typ == itemRightParen {
		t.next()
		return node
	}
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
			t.unexpected(tok, "reading function params")
		}
	}
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

func (t *Tree) nextNonComment() item {
	for {
		if tok := t.next(); tok.typ != itemComment {
			return tok
		}
	}
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

// expect consumes the next token and guarantees it has the required type.
func (t *Tree) expect(expected itemType, context string) item {
	token := t.next()
	if token.typ != expected {
		t.unexpected(token, fmt.Sprintf("%v (expected %v)", context, expected.String()))
	}
	return token
}

// unexpected complains about the token and terminates processing.
func (t *Tree) unexpected(token item, context string) {
	if token.typ == itemError {
		t.errorf("lexical error: %v", token)
	}
	t.errorf("unexpected %v in %s", token, context)
}

// errorf formats the error and terminates processing.
func (t *Tree) errorf(format string, args ...interface{}) {
	t.Root = nil
	format = fmt.Sprintf("template %s:%d:%d: %s", t.Name,
		t.lex.lineNumber(), t.lex.columnNumber(), format)
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

func allSpace(str string) bool {
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			return false
		}
	}
	return true
}
