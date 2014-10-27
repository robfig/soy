package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/robfig/soy/ast"
)

// Lexer design from text/template

// Tokens ---------------------------------------------------------------------

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos ast.Pos  // The starting position, in bytes, of this item in the input string.
	val string   // The value of this item.
}

func (i item) String() string {
	switch {
	case i.typ == itemEOF:
		return "EOF"
	case i.typ == itemError:
		return i.val
	case i.typ > itemCommand:
		return fmt.Sprintf("<%s>", i.val)
	case len(i.val) > 10:
		return fmt.Sprintf("%.10q...", i.val)
	}
	return fmt.Sprintf("%q", i.val)
}

// itemType identifies the type of lexical items.
type itemType int

// All items.
const (
	itemInvalid itemType = iota // not used
	itemEOF                     // EOF
	itemError                   // error occurred; value is text of error

	// Command delimiters
	itemLeftDelim     // tag left delimiter: {
	itemRightDelim    // tag right delimiter: }
	itemRightDelimEnd // tag right self-closing delimiter: /}

	itemText   // plain text
	itemEquals // =

	// Expression values
	itemNull    // e.g. null
	itemBool    // e.g. true
	itemInteger // e.g. 42
	itemFloat   // e.g. 1.0
	itemString  // e.g. 'hello world'
	itemComma   // , (used in function invocations, lists, maps, print directives)
	itemColon   // : (used in maps, print directives, operators)
	itemPipe    // | (used in print directives)

	// Data ref access tokens
	itemIdent            // identifier (e.g. function name)
	itemDollarIdent      // $ident
	itemDotIdent         // .ident
	itemQuestionDotIdent // ?.ident
	itemDotIndex         // .N
	itemQuestionDotIndex // ?.N
	itemLeftBracket      // [
	itemRightBracket     // ]
	itemQuestionKey      // ?[

	// Expression operations
	itemNegate // - (unary)
	itemMul    // *
	itemDiv    // /
	itemMod    // %
	itemAdd    // +
	itemSub    // - (binary)
	itemEq     // ==
	itemNotEq  // !=
	itemGt     // >
	itemGte    // >=
	itemLt     // <
	itemLte    // <=
	itemNot    // not
	itemOr     // or
	itemAnd    // and
	itemTernIf // ?
	itemElvis  // ?:

	itemLeftParen  // (
	itemRightParen // )

	// Soy doc
	itemSoyDocStart         // /**
	itemSoyDocParam         // @param name name
	itemSoyDocOptionalParam // @param? name
	itemSoyDocEnd           // */
	itemComment             // line comments (//) or block comments (/*)

	// Commands
	itemCommand     // used only to delimit the commands
	itemAlias       // {alias ...}
	itemCall        // {call ...}
	itemCase        // {case ...}
	itemCss         // {css ...}
	itemDefault     // {default}
	itemDelcall     // {delcall ...}
	itemDelpackage  // {delpackage ...}
	itemDeltemplate // {deltemplate ...}
	itemElse        // {else}
	itemElseif      // {elseif ...}
	itemFor         // {for ...}
	itemForeach     // {foreach ...}
	itemIf          // {if ...}
	itemIfempty     // {ifempty}
	itemLet         // {let}
	itemLiteral     // {literal}
	itemMsg         // {msg ...}
	itemNamespace   // {namespace}
	itemParam       // {param ...}
	itemPlural      // {plural ...}
	itemPrint       // {print ...}
	itemSwitch      // {switch ...}
	itemTemplate    // {template ...}
	itemLog         // {log}
	itemDebugger    // {debugger}
	// Character commands.
	itemSpecialChar
	itemSpace          // {sp}
	itemNil            // {nil}
	itemTab            // {\t}
	itemCarriageReturn // {\r}
	itemNewline        // {\n}
	itemLeftBrace      // {lb}
	itemRightBrace     // {rb}
	// Close commands.
	itemCommandEnd     // used only to delimit the commend ends.
	itemCallEnd        // {/call}
	itemDelcallEnd     // {/delcall}
	itemDeltemplateEnd // {/deltemplate}
	itemForEnd         // {/for}
	itemForeachEnd     // {/foreach}
	itemIfEnd          // {/if}
	itemLetEnd         // {/let}
	itemLiteralEnd     // {/literal}
	itemMsgEnd         // {/msg}
	itemParamEnd       // {/param}
	itemPluralEnd      // {/plural}
	itemSwitchEnd      // {/switch}
	itemTemplateEnd    // {/template}
	itemLogEnd         // {/log}

	// These commands are defined in TemplateParser.jj but not in the docs.
	// Apparently they are not available in the open source version of Soy.
	// See http://goo.gl/V0wsd
	// itemPlural               // {plural}{/plural}
	// itemSelect               // {select}{/select}
)

// isOp returns true if the item is an expression operation
func (t itemType) isOp() bool {
	return itemNegate <= t && t <= itemElvis
}

var builtinIdents = map[string]itemType{
	"alias":     itemAlias,
	"call":      itemCall,
	"case":      itemCase,
	"css":       itemCss,
	"debugger":  itemDebugger,
	"default":   itemDefault,
	"else":      itemElse,
	"elseif":    itemElseif,
	"for":       itemFor,
	"foreach":   itemForeach,
	"if":        itemIf,
	"ifempty":   itemIfempty,
	"let":       itemLet,
	"literal":   itemLiteral,
	"log":       itemLog,
	"msg":       itemMsg,
	"namespace": itemNamespace,
	"param":     itemParam,
	"plural":    itemPlural,
	"print":     itemPrint,
	"switch":    itemSwitch,
	"template":  itemTemplate,

	"/call":        itemCallEnd,
	"/delcall":     itemDelcallEnd,
	"/deltemplate": itemDeltemplateEnd,
	"/for":         itemForEnd,
	"/foreach":     itemForeachEnd,
	"/if":          itemIfEnd,
	"/let":         itemLetEnd,
	"/literal":     itemLiteralEnd,
	"/log":         itemLogEnd,
	"/msg":         itemMsgEnd,
	"/param":       itemParamEnd,
	"/plural":      itemPluralEnd,
	"/switch":      itemSwitchEnd,
	"/template":    itemTemplateEnd,

	"sp":  itemSpace,
	"nil": itemNil,
	`\t`:  itemTab,
	`\n`:  itemNewline,
	`\r`:  itemCarriageReturn,
	"lb":  itemLeftBrace,
	"rb":  itemRightBrace,

	"true":  itemBool,
	"false": itemBool,
	"and":   itemAnd,
	"null":  itemNull,
	"or":    itemOr,
	"not":   itemNot,
}

var arithmeticItemsBySymbol = map[string]itemType{
	"*":   itemMul,
	"/":   itemDiv,
	"%":   itemMod,
	"+":   itemAdd,
	"-":   itemSub,
	"==":  itemEq,
	"!=":  itemNotEq,
	">":   itemGt,
	">=":  itemGte,
	"<":   itemLt,
	"<=":  itemLte,
	"or":  itemOr,
	"and": itemAnd,
	"?":   itemTernIf,
	":":   itemColon,
	"?:":  itemElvis,
	"not": itemNot,
	"(":   itemLeftParen,
	")":   itemRightParen,
}

// isCommandEnd returns true if this is a command closing tag.
func (t itemType) isCommandEnd() bool {
	return t > itemCommandEnd
}

// String converts the itemType into its source string.
// It is fantastically inefficient and should only be used for error messages.
func (t itemType) String() string {
	for k, v := range builtinIdents {
		if v == t {
			return k
		}
	}
	for k, v := range arithmeticItemsBySymbol {
		if v == t {
			return k
		}
	}
	var r, ok = map[itemType]string{
		itemEOF:              "<eof>",
		itemEquals:           "=",
		itemError:            "<error>",
		itemText:             "<text>",
		itemLeftDelim:        "{",
		itemRightDelim:       "}",
		itemRightDelimEnd:    "/}",
		itemIdent:            "<ident>",
		itemDollarIdent:      "<$ident>",
		itemDotIdent:         "<.ident>",
		itemQuestionDotIdent: "<?.ident>",
		itemDotIndex:         "<.N>",
		itemQuestionDotIndex: "<?.N>",
		itemLeftBracket:      "[",
		itemRightBracket:     "]",
		itemQuestionKey:      "?[",
	}[t]
	if ok {
		return r
	}
	return fmt.Sprintf("item(%d)", t)
}

// Lexer ----------------------------------------------------------------------

const (
	eof        = -1
	leftDelim  = "{"
	rightDelim = "}"
	decDigits  = "0123456789"
	hexDigits  = "0123456789ABCDEF"
)

// stateFn represents the state of the lexer as a function that returns the
// next state.
type stateFn func(*lexer) stateFn

// lexer holds the state of the lexical scanning.
//
// Based on the lexer from the "text/template" package.
// See http://www.youtube.com/watch?v=HxaD_trXwRE
type lexer struct {
	name        string    // the name of the input; used only during errors.
	input       string    // the string being scanned.
	state       stateFn   // the next lexing function to enter.
	pos         ast.Pos   // current position in the input.
	start       ast.Pos   // start position of this item.
	width       int       // width of last rune read from input.
	items       chan item // channel of scanned items.
	doubleDelim bool      // flag for tags starting with double braces.
	lastEmit    item      // type of most recent item emitted
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	return <-l.items
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
		state: lexText,
	}
	go l.run()
	return l
}

// lexExpr lexes a single expression.
func lexExpr(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
		state: lexInsideTag,
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.state != nil {
		l.state = l.state(l)
	}
	close(l.items)
}

// next returns the next rune in the input.
func (l *lexer) next() (r rune) {
	if l.pos >= ast.Pos(len(l.input)) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += ast.Pos(l.width)
	return r
}

// peek returns but does not consume the next rune in the input.
func (l *lexer) peek() rune {
	r := l.next()
	l.backup()
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos -= ast.Pos(l.width)
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	if l.pos > ast.Pos(len(l.input)) {
		l.pos = ast.Pos(len(l.input))
	}
	l.lastEmit = item{t, l.pos, l.input[l.start:l.pos]}
	l.items <- l.lastEmit
	l.start = l.pos
}

// ignore skips over the pending input before this point.
func (l *lexer) ignore() {
	l.start = l.pos
}

// accept consumes the next rune if it's from the valid set.
func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.next()) >= 0 {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *lexer) acceptRun(valid string) bool {
	pos := l.pos
	for strings.IndexRune(valid, l.next()) >= 0 {
	}
	l.backup()
	return l.pos > pos
}

// lineNumber reports which line we're on. Doing it this way
// means we don't have to worry about peek double counting.
func (l *lexer) lineNumber(pos ast.Pos) int {
	return 1 + strings.Count(l.input[:pos], "\n")
}

// columnNumber reports which column in the current line we're on.
func (l *lexer) columnNumber(pos ast.Pos) int {
	n := strings.LastIndex(l.input[:pos], "\n")
	if n == -1 {
		n = 0
	}
	return int(pos) - n
}

// errorf returns an error item and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.pos, fmt.Sprintf(format, args...)}
	return nil
}

// State functions ------------------------------------------------------------

func maybeEmitText(l *lexer, backup int) {
	if l.pos-ast.Pos(backup) > l.start {
		l.pos -= ast.Pos(backup)
		if allSpaceWithNewline(l.input[l.start:l.pos]) {
			l.ignore()
		} else {
			l.emit(itemText)
		}
		l.pos += ast.Pos(backup)
	}
}

// lexText scans until an opening command delimiter, "{".
// it ignores line comments (//) and block comments (/* */).
func lexText(l *lexer) stateFn {
	var r, lastChar rune
	for {
		lastChar = r
		r = l.next()

		// comment / soydoc handling
		if r == '/' {
			switch l.next() {
			case '/':
				// '//' only begins a comment if the previous character is whitespace,
				// or if we are the start of input.
				var lastCharEmitted = lastChar
				if lastChar == 0 && l.lastEmit.val != "" {
					lastCharEmitted = rune(l.lastEmit.val[len(l.lastEmit.val)-1])
				}
				if lastCharEmitted == 0 || isSpaceEOL(lastCharEmitted) {
					maybeEmitText(l, 3)
					if lastChar != 0 {
						l.start++ // ignore the preceeding space, if present.
					}
					return lexLineComment(l)
				}
			case '*':
				maybeEmitText(l, 2)
				if l.next() == '*' {
					return lexSoyDoc(l)
				}
				l.backup()
				return lexBlockComment(l)
			}
			l.backup()
		}

		// eof or entering a tag?
		switch r {
		case '{':
			l.backup()
			maybeEmitText(l, 0)
			return lexLeftDelim
		case '}':
			return l.errorf("unexpected closing delimiter } found in input.")
		case eof:
			l.backup()
			maybeEmitText(l, 0)
			l.emit(itemEOF)
			return nil
		}
	}
}

// lexLeftDelim scans the left template tag delimiter
//
// If there are brace characters within a template tag, double braces must
// be used, so we differentiate them to match double closing braces later.
// Double braces are also optional for other cases.
func lexLeftDelim(l *lexer) stateFn {
	l.next() // read the first {
	// check the next character to see if it's a double delimiter
	if r := l.next(); r == '{' {
		l.doubleDelim = true
	} else {
		l.backup()
		l.doubleDelim = false
	}
	l.emit(itemLeftDelim)
	return lexBeginTag
}

// lexRightDelim scans the right template tag delimiter
// } has already been read.
func lexRightDelim(l *lexer) stateFn {
	if l.doubleDelim && l.next() != '}' {
		return l.errorf("expected double closing braces in tag")
	}
	l.emit(itemRightDelim)
	return lexText
}

// lexRightDelim scans the self-closing right template tag delimiter
// / has already been read.
func lexRightDelimEnd(l *lexer) stateFn {
	l.next()
	if l.doubleDelim && l.next() != '}' {
		return l.errorf("expected double closing braces in tag")
	}
	l.emit(itemRightDelimEnd)
	return lexText
}

// lexBeginTag handles an ambiguity:
//  - "/" is arithmetic or begins the "/if" command?
func lexBeginTag(l *lexer) stateFn {
	switch l.peek() {
	case '/', '\\':
		return lexIdent
	}
	return lexInsideTag
}

// lexInsideTag is called repeatedly to scan elements inside a template tag.
// itemLeftDelim has just been emitted.
func lexInsideTag(l *lexer) stateFn {
	switch r := l.next(); {
	case isSpaceEOL(r):
		l.ignore()
	case r == '/' && l.peek() == '}':
		return lexRightDelimEnd
	case r == '$', r == '.':
		l.backup()
		return lexIdent
	case r == '[':
		l.emit(itemLeftBracket)
	case r == ']':
		l.emit(itemRightBracket)
	case r == '?': // used by data refs and arithmetic
		switch l.next() {
		case '.':
			l.pos -= 2
			return lexIdent
		case '[':
			l.emit(itemQuestionKey)
		case ':':
			l.emit(itemElvis)
		default:
			l.backup()
			l.emit(itemTernIf)
		}
	case r == '-':
		return lexNegative(l)
	case r == '}':
		return lexRightDelim
	case r >= '0' && r <= '9':
		l.backup()
		return lexNumber
	case r == '*', r == '/', r == '%', r == '+', r == ':', r == '(', r == ')':
		// the single-character symbols
		l.emit(arithmeticItemsBySymbol[string(r)])
	case r == '>', r == '!', r == '<', r == '=' && l.peek() == '=':
		// 1 or 2 character symbols
		l.accept("*/%+-=!<>|&?:")
		sym := l.input[l.start:l.pos]
		item, ok := arithmeticItemsBySymbol[sym]
		if !ok {
			return l.errorf("unexpected symbol: %s", sym)
		}
		l.emit(item)
	case r == '"', r == '\'':
		return stringLexer(r)
	case r == '=':
		l.emit(itemEquals)
	case r == eof:
		return l.errorf("unclosed tag")
	case r == '|':
		l.emit(itemPipe)
	case isLetterOrUnderscore(r):
		l.backup()
		return lexIdent
	case r == ',':
		l.emit(itemComma)
	default:
		return l.errorf("unrecognized character in action: %#U", r)
	}

	return lexInsideTag
}

func lexNegative(l *lexer) stateFn {
	// is it unary or binary op?
	// unary if it starts a group ('{' or '(') or an op came just before.
	var lastType = l.lastEmit.typ
	if lastType == itemInvalid ||
		lastType.isOp() ||
		lastType == itemLeftDelim ||
		lastType == itemCase ||
		lastType == itemComma ||
		lastType == itemLeftParen {
		// is it a negative number?
		if l.peek() >= '0' && l.peek() <= '9' {
			l.backup()
			return lexNumber
		}
		l.emit(itemNegate)
	} else {
		l.emit(itemSub)
	}
	return lexInsideTag
}

// lexSoyDoc emits:
// - the start and end tokens (/**, */)
// - the individual lines of the soydoc comment, trimmed.
// - the parameter tokens and identifiers
// '/**' has just been read.
func lexSoyDoc(l *lexer) stateFn {
	l.emit(itemSoyDocStart)
	var star = false
	var startOfLine = true // ignoring whitespace and asterisks.
	for {
		var ch = l.next()
		if ch == eof {
			return l.errorf("unexpected eof when scanning soydoc")
		}
		if star && ch == '/' {
			maybeEmitText(l, 2)
			l.emit(itemSoyDocEnd)
			return lexText
		}
		if startOfLine {
			// ignore any space or asterisks at the beginning of lines
			if isSpaceEOL(ch) {
				continue
			}
			if ch == '*' {
				star = true
				continue
			}
			l.pos--
			l.ignore()

			// start with @param?
			if strings.HasPrefix(l.input[l.pos:], "@param") {
				lexSoyDocParam(l)
			}
			startOfLine = false
		}

		if isEndOfLine(ch) {
			maybeEmitText(l, 1)
			startOfLine = true
		}
		star = ch == '*'
	}
}

func lexSoyDocParam(l *lexer) {
	l.pos += ast.Pos(len("@param"))
	switch ch := l.next(); {
	case ch == '?':
		if l.next() != ' ' {
			return
		}
		l.backup()
		l.emit(itemSoyDocOptionalParam)
	case ch == ' ':
		l.backup()
		l.emit(itemSoyDocParam)
	default:
		return // what a fakeout
	}

	// skip all spaces
	for {
		var r = l.next()
		if r == eof || !isSpace(r) {
			break
		}
	}
	l.backup()
	l.ignore()

	// extract the param
	for {
		var r = l.next()
		if isSpaceEOL(r) || r == eof {
			l.pos--
			l.emit(itemIdent)
			// don't skip newlines. the outer routine needs to know about it
			if isSpace(r) || r == eof {
				l.pos++
			}
			l.ignore()
			break
		}
	}
}

// "//" has just been read
func lexLineComment(l *lexer) stateFn {
	for {
		if r := l.next(); isEndOfLine(r) || r == eof {
			l.emit(itemComment)
			return lexText
		}
	}
}

// "/*" has just been read
func lexBlockComment(l *lexer) stateFn {
	var star = false
	for {
		switch l.next() {
		case eof:
			return l.errorf("unclosed block comment")
		case '*':
			star = true
			continue
		case '/':
			if star {
				l.emit(itemComment)
				return lexText
			}
		}
		star = false
	}
}

// stringLexer returns a stateFn that lexes strings surrounded by the given quote character.
func stringLexer(quoteChar rune) stateFn {
	// the quote char has already been read.
	return func(l *lexer) stateFn {
		for {
			switch l.next() {
			case eof:
				l.errorf("unexpected eof while scanning string")
			case '\\':
				l.next() // skip escape sequences
			case quoteChar:
				l.emit(itemString)
				return lexInsideTag
			}
		}
	}
}

// lexIdent recognizes the various kinds of identifiers
func lexIdent(l *lexer) stateFn {
	// the different idents start with different unique characters.
	// peel those off.
	var itemType = itemIdent
	switch l.next() {
	case '.':
		if isDigit(l.next()) {
			itemType = itemDotIndex
		} else {
			itemType = itemDotIdent
		}
		l.backup()
	case '$':
		itemType = itemDollarIdent
	case '/':
		itemType = itemCommandEnd
	case '\\':
		itemType = itemSpecialChar
	case '?':
		dot := l.next()
		if dot != '.' {
			l.errorf("unexpected beginning to ident: ?%v", dot)
		}
		if isDigit(l.next()) {
			itemType = itemQuestionDotIndex
		} else {
			itemType = itemQuestionDotIdent
		}
		l.backup()
	}

	// absorb the rest of the identifier
	for isAlphaNumeric(l.next()) {
	}
	l.backup()
	word := l.input[l.start:l.pos]

	// if it's a builtin, return that item type
	if itemType, ok := builtinIdents[word]; ok {
		l.emit(itemType)
		// {literal} and {css} have unusual lexing rules
		switch itemType {
		case itemLiteral:
			return lexLiteral
		case itemCss:
			return lexCss
		}
		return lexInsideTag
	}
	// if not a builtin, it shouldn't start with / or \
	if itemType == itemCommandEnd || itemType == itemSpecialChar {
		var str = l.input[l.start:l.pos]
		l.pos = l.start
		return l.errorf("unrecognized identifier %q", str)
	}

	// else, use the type determined at the beginning.
	l.emit(itemType)
	return lexInsideTag
}

// lexCss scans the body of the {css} command into an itemText.
// This is required because css classes are unquoted and may have hyphens (and
// thus are not recognized as idents).
// itemCss has already been emitted
func lexCss(l *lexer) stateFn {
	l.next()
	l.ignore()
	for l.next() != '}' {
	}
	l.backup()
	l.emit(itemText)
	l.next()
	if l.doubleDelim && l.next() != '}' {
		return l.errorf("expected double closing braces in tag")
	}
	l.emit(itemRightDelim)
	return lexText
}

// lexLiteral scans until a closing literal delimiter, "{\literal}".
// It emits the literal text and the closing tag.
//
// A literal section contains raw text and may include braces.
// itemLiteral has already been emitted.
func lexLiteral(l *lexer) stateFn {
	// emit the closing of the initial {literal} tag
	var ch = l.next()
	for isSpace(ch) {
		ch = l.next()
	}
	if ch != '}' {
		return l.errorf("expected closing tag after {literal..")
	}
	if l.doubleDelim && l.next() != '}' {
		return l.errorf("expected double closing braces in tag")
	}
	l.emit(itemRightDelim)

	// Fast forward through the literal section.
	var expectClose, delimLen = "{/literal}", 1
	if l.doubleDelim {
		expectClose, delimLen = "{{/literal}}", 2
	}
	var i = strings.Index(l.input[l.pos:], expectClose)
	if i == -1 {
		return l.errorf("unclosed literal")
	}
	l.pos += ast.Pos(i)

	// Accept everything as itemText until we see the {/literal}
	// Emit the other various tokens.
	if i > 0 {
		l.emit(itemText)
	}
	l.pos += ast.Pos(delimLen)
	l.emit(itemLeftDelim)
	l.pos += ast.Pos(len("/literal"))
	l.emit(itemLiteralEnd)
	l.pos += ast.Pos(delimLen)
	l.emit(itemRightDelim)
	return lexText
}

// lexNumber scans a number: a float or integer (which can be decimal or hex).
func lexNumber(l *lexer) stateFn {
	typ, ok := scanNumber(l)
	if !ok {
		return l.errorf("bad number syntax: %q", l.input[l.start:l.pos])
	}
	// Emits itemFloat or itemInteger.
	l.emit(typ)
	return lexInsideTag
}

// scanNumber scans a number according to Soy's specification.
//
// It returns the scanned itemType (itemFloat or itemInteger) and a flag
// indicating if an error was found.
//
// Floats must be in decimal and must either:
//
//     - Have digits both before and after the decimal point (both can be
//       a single 0), e.g. 0.5, -100.0, or
//     - Have a lower-case e that represents scientific notation,
//       e.g. -3e-3, 6.02e23.
//
// Integers can be:
//
//     - decimal (e.g. -827)
//     - hexadecimal (must begin with 0x and must use capital A-F,
//       e.g. 0x1A2B).
func scanNumber(l *lexer) (typ itemType, ok bool) {
	typ = itemInteger
	// Optional leading sign.
	hasSign := l.accept("+-")
	if ast.Pos(len(l.input)) >= l.pos+2 && l.input[l.pos:l.pos+2] == "0x" {
		// Hexadecimal.
		if hasSign {
			// No signs for hexadecimals.
			return
		}
		l.acceptRun("0x")
		if !l.acceptRun(hexDigits) {
			// Requires at least one digit.
			return
		}
		if l.accept(".") {
			// No dots for hexadecimals.
			return
		}
	} else {
		// Decimal.
		if !l.acceptRun(decDigits) {
			// Requires at least one digit.
			return
		}
		if l.accept(".") {
			// Float.
			if !l.acceptRun(decDigits) {
				// Requires a digit after the dot.
				return
			}
			typ = itemFloat
		} else {
			if (!hasSign && l.input[l.start] == '0' && l.pos > l.start+1) ||
				(hasSign && l.input[l.start+1] == '0' && l.pos > l.start+2) {
				// Integers can't start with 0.
				return
			}
		}
		if l.accept("e") {
			l.accept("+-")
			if !l.acceptRun(decDigits) {
				// A digit is required after the scientific notation.
				return
			}
			typ = itemFloat
		}
	}
	// Next thing must not be alphanumeric.
	if isAlphaNumeric(l.peek()) {
		l.next()
		return
	}
	ok = true
	return
}

// Helpers --------------------------------------------------------------------

// isAlphaNumeric reports whether r is an alphabetic, digit, or underscore.
func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

// isEndOfLine reports whether r is an end-of-line character.
func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

// isSpaceEOL returns true if r is space or end of line.
func isSpaceEOL(r rune) bool {
	return isSpace(r) || isEndOfLine(r)
}

func isLetterOrUnderscore(r rune) bool {
	return 'a' <= r && r <= 'z' ||
		'A' <= r && r <= 'Z' ||
		r == '_'
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// allSpaceWithNewline returns true if the entire string consists of whitespace,
// with at least one newline.
func allSpaceWithNewline(str string) bool {
	var seenNewline = false
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			return false
		}
		if isEndOfLine(ch) {
			seenNewline = true
		}
	}
	return seenNewline
}
