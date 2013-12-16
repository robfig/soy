package parse

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lexer design from text/template

// Tokens ---------------------------------------------------------------------

// item represents a token or text string returned from the scanner.
type item struct {
	typ itemType // The type of this item.
	pos Pos      // The starting position, in bytes, of this item in the input string.
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
	itemNil        itemType = iota // not used
	itemEOF                        // EOF
	itemError                      // error occurred; value is text of error
	itemLeftDelim                  // tag left delimiter: {
	itemRightDelim                 // tag right delimiter: }
	itemText                       // plain text
	itemIdent                      // identifier
	itemVariable                   // $variable
	itemEquals                     // =
	// Primitive literals.
	itemBool
	itemFloat
	itemInteger
	itemList
	itemMap
	itemString
	itemSoyDocStart
	itemSoyDocParam
	itemSoyDocEnd
	// Print directives - |<directive>[:arg1[,arg2..]]
	itemDirective
	itemDirectiveArg
	// Commands.
	itemCommand     // used only to delimit the commands
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
	itemLiteral     // {literal}
	itemMsg         // {msg ...}
	itemNamespace   // {namespace}
	itemParam       // {param ...}
	itemPrint       // {print ...}
	itemSwitch      // {switch ...}
	itemTemplate    // {template ...}
	// Character commands.
	itemCarriageReturn // {\r}
	itemEmptyString    // {nil}
	itemLeftBrace      // {lb}
	itemNewline        // {\n}
	itemRightBrace     // {rb}
	itemSpace          // {sp}
	itemTab            // {\t}
	// Close commands.
	itemCommandEnd     // used only to delimit the commend ends.
	itemCallEnd        // {/call}
	itemDelcallEnd     // {/delcall}
	itemDeltemplateEnd // {/deltemplate}
	itemForEnd         // {/for}
	itemForeachEnd     // {/foreach}
	itemIfEnd          // {/if}
	itemLiteralEnd     // {/literal}
	itemMsgEnd         // {/msg}
	itemParamEnd       // {/param}
	itemSwitchEnd      // {/switch}
	itemTemplateEnd    // {/template}

	// These commands are defined in TemplateParser.jj but not in the docs.
	// Apparently they are not available in the open source version of Soy.
	// See http://goo.gl/V0wsd
	// itemLet                  // {let}{/let}
	// itemPlural               // {plural}{/plural}
	// itemSelect               // {select}{/select}
)

var commands = map[string]itemType{
	"namespace": itemNamespace,
	"template":  itemTemplate,
	"call":      itemCall,
	"case":      itemCase,
	"css":       itemCss,
	"default":   itemDefault,
	"else":      itemElse,
	"elseif":    itemElseif,
	"for":       itemFor,
	"foreach":   itemForeach,
	"if":        itemIf,
	"ifempty":   itemIfempty,
	"literal":   itemLiteral,
	"msg":       itemMsg,
	"param":     itemParam,
	"print":     itemPrint,
	"switch":    itemSwitch,

	"/call":        itemCallEnd,
	"/delcall":     itemDelcallEnd,
	"/deltemplate": itemDeltemplateEnd,
	"/for":         itemForEnd,
	"/foreach":     itemForeachEnd,
	"/if":          itemIfEnd,
	"/literal":     itemLiteralEnd,
	"/msg":         itemMsgEnd,
	"/param":       itemParamEnd,
	"/switch":      itemSwitchEnd,
	"/template":    itemTemplateEnd,
}

// isCommandEnd returns true if this is a command closing tag.
func (t itemType) isCommandEnd() bool {
	return t > itemCommandEnd
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
	pos         Pos       // current position in the input.
	start       Pos       // start position of this item.
	width       int       // width of last rune read from input.
	items       chan item // channel of scanned items.
	doubleDelim bool      // flag for tags starting with double braces.
	lastPos     Pos       // position of most recent item returned by nextItem
}

// nextItem returns the next item from the input.
func (l *lexer) nextItem() item {
	item := <-l.items
	l.lastPos = item.pos
	return item
}

// lex creates a new scanner for the input string.
func lex(name, input string) *lexer {
	l := &lexer{
		name:  name,
		input: input,
		items: make(chan item),
	}
	go l.run()
	return l
}

// run runs the state machine for the lexer.
func (l *lexer) run() {
	for l.state = lexText; l.state != nil; {
		l.state = l.state(l)
	}
}

// next returns the next rune in the input.
func (l *lexer) next() (r rune) {
	if l.pos >= Pos(len(l.input)) {
		l.width = 0
		return eof
	}
	r, l.width = utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += Pos(l.width)
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
	l.pos -= Pos(l.width)
}

// emit passes an item back to the client.
func (l *lexer) emit(t itemType) {
	if l.pos > Pos(len(l.input)) {
		l.pos = Pos(len(l.input))
	}
	l.items <- item{t, l.pos, l.input[l.start:l.pos]}
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
func (l *lexer) lineNumber() int {
	return 1 + strings.Count(l.input[:l.pos], "\n")
}

// columnNumber reports which column in the current line we're on.
func (l *lexer) columnNumber() int {
	n := strings.LastIndex(l.input[:l.pos], "\n")
	if n == -1 {
		n = 0
	}
	return int(l.pos) - n
}

// error returns an error item and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- item{itemError, l.pos, fmt.Sprintf(format, args...)}
	return nil
}

// State functions ------------------------------------------------------------

func backupAndMaybeEmitText(l *lexer) {
	l.backup()
	if l.pos > l.start {
		if allSpace(l.input[l.start:l.pos]) {
			l.ignore()
		} else {
			l.emit(itemText)
		}
	}
}

// lexText scans until an opening command delimiter, "{".
func lexText(l *lexer) stateFn {
	for {
		switch l.next() {
		case '{':
			backupAndMaybeEmitText(l)
			return lexLeftDelim
		case '/':
			if l.peek() == '*' {
				backupAndMaybeEmitText(l)
				return lexSoyDoc
			}
		case eof:
			backupAndMaybeEmitText(l)
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
	return lexInsideTag
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

// lexInsideTag scans the elements inside a template tag.
//
// The first item within a tag is the command name (the print command is
// implied if no command is specified), and the rest of the text within the
// tag (if any) is referred to as the command text.
//
// Soy tag format:
//
//     - Can be delimited by single braces "{...}" or double braces "{{...}}".
//     - Soy tags delimited by double braces are allowed to contain single
//       braces within.
//     - Some Soy tags are allowed to end in "/}" or "/}}" to denote immediate
//       ending of a block.
//     - It is an error to use "/}" or "/}}" when it's not applicable to the
//       command.
//     - If there is a command name, it must come immediately after the
//       opening delimiter.
//     - The command name must be followed by either the closing delimiter
//       (if the command does not take any command text) or a whitespace (if
//       the command takes command text).
//     - It is an error to provide command text when it's not applicable,
//       and vice versa.
//
// Commands without closing tag (can't end in "/}" or "/}}"):
//
//     - {delpackage ...}
//     - {namespace ...}
//     - {print ...}
//     - {...} (implicit print)
//     - {\r}
//     - {nil}
//     - {lb}
//     - {\n}
//     - {rb}
//     - {sp}
//     - {\t}
//     - {elseif ...}
//     - {else ...}
//     - {case ...}
//     - {default}
//     - {ifempty}
//     - {css ...}
//
// Commands with optional closing tag:
//
//     - {call ... /} or {call ...}...{/call}
//     - {delcall ... /} or {delcall ...}...{/delcall}
//     - {param ... /} or {param ...}...{/param}
//
// Commands with required closing tag:
//
//     - {deltemplate ...}...{/deltemplate}
//     - {for ...}...{/for}
//     - {foreach ...}...{/foreach}
//     - {if ...}...{/if}
//     - {literal}...{/literal}
//     - {msg ...}...{/msg}
//     - {switch ...}...{/switch}
//     - {template ...}...{/template}
func lexInsideTag(l *lexer) stateFn {
	switch r := l.next(); {
	case isSpace(r):
		l.ignore()
		return lexInsideTag
	case r == '$':
		return lexVariable
	case r == '}':
		return lexRightDelim
	case r == eof || isEndOfLine(r):
		return l.errorf("unclosed tag")
	case r == '|':
		return lexDirective
	case r == '.':
		return lexIdent
	case isLetterOrUnderscore(r), r == '/':
		l.backup()
		return lexIdent
	default:
		return l.errorf("unrecognized character in action: %#U", r)
	}

	return lexInsideTag
}

// lexVariable scans a variable: $Alphanumeric
// $ has already been read.
func lexVariable(l *lexer) stateFn {
	for r := l.next(); isAlphaNumeric(r); r = l.next() {
	}
	l.backup()
	l.emit(itemVariable)
	return lexInsideTag
}

// TODO: extract soydoc params here?
func lexSoyDoc(l *lexer) stateFn {
	l.emit(itemSoyDocStart)
	var star = false
	for {
		var ch = l.next()
		if ch == eof {
			l.errorf("unexpected eof when scanning soydoc")
		}
		if star && ch == '/' {
			l.emit(itemText)
			l.emit(itemSoyDocEnd)
			return lexText
		}
		star = ch == '*'
	}
}

// lexDirective scans a print directive: |Alphanumeric[:arg1[,arg2..]]
// | has already been read.
func lexDirective(l *lexer) stateFn {
	// TODO
	return lexInsideTag
}

func lexIdent(l *lexer) stateFn {
Loop:
	for {
		switch r := l.next(); {
		case isAlphaNumeric(r), r == '/':
			// absorb.
		default:
			l.backup()
			word := l.input[l.start:l.pos]
			if !l.atTerminator() {
				return l.errorf("bad character %#U", r)
			}
			switch {
			case commands[word] > itemNil:
				l.emit(commands[word])
			case word == "true", word == "false":
				l.emit(itemBool)
			default:
				if word[0] == '/' {
					// TODO: this doesn't leave the position in the correct spot.
					return l.errorf("bad character %#U", word[0])
				}
				l.emit(itemIdent)
			}
			break Loop
		}
	}
	return lexInsideTag
}

// lexLiteral scans until a closing literal delimiter, "{\literal}".
// It emits the literal text and the closing tag.
//
// A literal section contains raw text and may include braces.
func lexLiteral(l *lexer) stateFn {
	var end bool
	var pos Pos
	for {
		if strings.HasPrefix(l.input[l.pos:], "{/literal}") {
			end, pos = true, 10
		} else if strings.HasPrefix(l.input[l.pos:], "{{/literal}}") {
			end, pos = true, 12
		}
		if end {
			if l.pos > l.start {
				l.emit(itemText)
			}
			l.pos += pos
			l.emit(itemLiteralEnd)
		}
		if l.next() == eof {
			return l.errorf("unclosed literal")
		}
	}
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
	if l.input[l.pos:l.pos+2] == "0x" {
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
			if (!hasSign && l.input[l.start] == '0') ||
				(hasSign && l.input[l.start+1] == '0') {
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

// // lexSpace scans a run of space characters.
// // One space has already been seen.
// func lexSpace(l *lexer) stateFn {
// 	for isSpace(l.peek()) {
// 		l.next()
// 	}
// 	l.emit(itemSpace)
// 	return lexInsideTag
// }

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

// atTerminator reports whether the input is at valid termination character to
// appear after an identifier.
func (l *lexer) atTerminator() bool {
	r := l.peek()
	if isSpace(r) || isEndOfLine(r) {
		return true
	}
	switch r {
	case eof, '|', '=', '}':
		return true
	}
	return false
}

func isLetterOrUnderscore(r rune) bool {
	return 'a' <= r && r <= 'z' ||
		'A' <= r && r <= 'Z' ||
		r == '_'
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

// allSpace returns true if the entire string consists of whitespace
func allSpace(str string) bool {
	for _, ch := range str {
		if !unicode.IsSpace(ch) {
			return false
		}
	}
	return true
}
