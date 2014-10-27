package parse

import "testing"

type lexTest struct {
	name  string
	input string
	items []item
}

var (
	tEOF   = item{itemEOF, 0, ""}
	tLeft  = item{itemLeftDelim, 0, "{"}
	tRight = item{itemRightDelim, 0, "}"}
)

var lexTests = []lexTest{
	{"empty", "", []item{tEOF}},
	{"eaten spaces", " \t\n", []item{tEOF}},
	{"text", `now is the time`, []item{{itemText, 0, "now is the time"}, tEOF}},
	{"keywords", "{namespace template}", []item{
		tLeft,
		{itemNamespace, 0, "namespace"},
		{itemTemplate, 0, "template"},
		tRight,
		tEOF,
	}},
	{"double delimiters", "{{namespace/}}", []item{
		{itemLeftDelim, 0, "{{"},
		{itemNamespace, 0, "namespace"},
		{itemRightDelimEnd, 0, "/}}"},
		tEOF,
	}},
	{"dottted ident", "{namespace a.namespace.name}", []item{
		tLeft,
		{itemNamespace, 0, "namespace"},
		{itemIdent, 0, "a"},
		{itemDotIdent, 0, ".namespace"},
		{itemDotIdent, 0, ".name"},
		tRight,
		tEOF,
	}},
	{"leadingdotident", "{template .name}", []item{
		tLeft,
		{itemTemplate, 0, "template"},
		{itemDotIdent, 0, ".name"},
		tRight,
		tEOF,
	}},
	{"variable", `{$name.bar}`, []item{
		tLeft,
		{itemDollarIdent, 0, "$name"},
		{itemDotIdent, 0, ".bar"},
		tRight,
		tEOF,
	}},

	{"if", `{if $var}{$var} is true{/if}`, []item{
		tLeft,
		{itemIf, 0, "if"},
		{itemDollarIdent, 0, "$var"},
		tRight,
		tLeft,
		{itemDollarIdent, 0, "$var"},
		tRight,
		{itemText, 0, ` is true`},
		tLeft,
		{itemIfEnd, 0, "/if"},
		tRight,
		tEOF,
	}},
	{"if-else", `{if $var}true{else}false{/if}`, []item{
		tLeft,
		{itemIf, 0, "if"},
		{itemDollarIdent, 0, "$var"},
		tRight,
		{itemText, 0, `true`},
		tLeft,
		{itemElse, 0, "else"},
		tRight,
		{itemText, 0, `false`},
		tLeft,
		{itemIfEnd, 0, "/if"},
		tRight,
		tEOF,
	}},
	{"if expr", `{if hasData() and $name}{/if}`, []item{
		tLeft,
		{itemIf, 0, "if"},
		{itemIdent, 0, "hasData"},
		{itemLeftParen, 0, "("},
		{itemRightParen, 0, ")"},
		{itemAnd, 0, "and"},
		{itemDollarIdent, 0, "$name"},
		tRight,
		tLeft,
		{itemIfEnd, 0, "/if"},
		tRight,
		tEOF,
	}},

	{"special characters", `{sp}{nil}{\r}{\n}{\t}{lb}{rb}`, []item{
		tLeft, {itemSpace, 0, "sp"}, // {sp}
		tRight, tLeft, {itemNil, 0, "nil"}, // {nil}
		tRight, tLeft, {itemCarriageReturn, 0, "\\r"}, // {\r}
		tRight, tLeft, {itemNewline, 0, "\\n"}, // {\n}
		tRight, tLeft, {itemTab, 0, "\\t"}, // {\t}
		tRight, tLeft, {itemLeftBrace, 0, "lb"}, // {lb}
		tRight, tLeft, {itemRightBrace, 0, "rb"}, // {rb}
		tRight, tEOF,
	}},

	{"foreach", `{foreach $foo in $bars}{$foo}{ifempty}No bars{/foreach}`, []item{
		tLeft,
		{itemForeach, 0, "foreach"},
		{itemDollarIdent, 0, "$foo"},
		{itemIdent, 0, "in"},
		{itemDollarIdent, 0, "$bars"},
		tRight,
		tLeft, {itemDollarIdent, 0, "$foo"}, tRight,
		tLeft, {itemIfempty, 0, "ifempty"}, tRight,
		{itemText, 0, "No bars"},
		tLeft, {itemForeachEnd, 0, "/foreach"}, tRight,
		tEOF,
	}},

	{"switch", `{switch $boo} {case 0}Blah{case $foo.goo}Bleh{case -1, 1, $moo} {default}{/switch}`, []item{
		tLeft,
		{itemSwitch, 0, "switch"},
		{itemDollarIdent, 0, "$boo"},
		tRight,
		{itemText, 0, " "},
		tLeft,
		{itemCase, 0, "case"},
		{itemInteger, 0, "0"},
		tRight,
		{itemText, 0, "Blah"},
		tLeft,
		{itemCase, 0, "case"},
		{itemDollarIdent, 0, "$foo"},
		{itemDotIdent, 0, ".goo"},
		tRight,
		{itemText, 0, "Bleh"},
		tLeft,
		{itemCase, 0, "case"},
		{itemInteger, 0, "-1"},
		{itemComma, 0, ","},
		{itemInteger, 0, "1"},
		{itemComma, 0, ","},
		{itemDollarIdent, 0, "$moo"},
		tRight,
		{itemText, 0, " "},
		tLeft,
		{itemDefault, 0, "default"},
		tRight,
		tLeft,
		{itemSwitchEnd, 0, "/switch"},
		tRight,
		tEOF,
	}},

	{"plural", `{plural $boo}
{case 1}One boo
{default}Multiple boo
{/plural}`, []item{
		tLeft,
		{itemPlural, 0, "plural"},
		{itemDollarIdent, 0, "$boo"},
		tRight,
		tLeft,
		{itemCase, 0, "case"},
		{itemInteger, 0, "1"},
		tRight,
		{itemText, 0, "One boo\n"},
		tLeft,
		{itemDefault, 0, "default"},
		tRight,
		{itemText, 0, "Multiple boo\n"},
		tLeft,
		{itemPluralEnd, 0, "/plural"},
		tRight,
		tEOF,
	}},

	{"call", `{call .other data="all"/}`, []item{
		tLeft,
		{itemCall, 0, "call"},
		{itemDotIdent, 0, ".other"},
		{itemIdent, 0, "data"},
		{itemEquals, 0, "="},
		{itemString, 0, `"all"`},
		{itemRightDelimEnd, 0, "/}"},
		tEOF,
	}},

	{"not expr", `{(not $var)}`, []item{
		tLeft,
		{itemLeftParen, 0, "("},
		{itemNot, 0, "not"},
		{itemDollarIdent, 0, "$var"},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"arithmetic", `{2+2.0 == null and ? not : true <= "10" ?:/}`, []item{
		tLeft,
		{itemInteger, 0, "2"},
		{itemAdd, 0, "+"},
		{itemFloat, 0, "2.0"},
		{itemEq, 0, "=="},
		{itemNull, 0, "null"},
		{itemAnd, 0, "and"},
		{itemTernIf, 0, "?"},
		{itemNot, 0, "not"},
		{itemColon, 0, ":"},
		{itemBool, 0, "true"},
		{itemLte, 0, "<="},
		{itemString, 0, `"10"`},
		{itemElvis, 0, "?:"},
		{itemRightDelimEnd, 0, "/}"},
		tEOF,
	}},

	{"expression", `{"a"+"b" != "ab" and (2 >= 5.0 or (null ?: true))}`, []item{
		tLeft,
		{itemString, 0, `"a"`},
		{itemAdd, 0, "+"},
		{itemString, 0, `"b"`},
		{itemNotEq, 0, "!="},
		{itemString, 0, `"ab"`},
		{itemAnd, 0, "and"},
		{itemLeftParen, 0, "("},
		{itemInteger, 0, "2"},
		{itemGte, 0, ">="},
		{itemFloat, 0, "5.0"},
		{itemOr, 0, "or"},
		{itemLeftParen, 0, "("},
		{itemNull, 0, "null"},
		{itemElvis, 0, "?:"},
		{itemBool, 0, "true"},
		{itemRightParen, 0, ")"},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"expression2", `{0.5<=1 ? null?:"hello" : (1!=1)}`, []item{
		tLeft,
		{itemFloat, 0, "0.5"},
		{itemLte, 0, "<="},
		{itemInteger, 0, "1"},
		{itemTernIf, 0, "?"},
		{itemNull, 0, "null"},
		{itemElvis, 0, "?:"},
		{itemString, 0, `"hello"`},
		{itemColon, 0, ":"},
		{itemLeftParen, 0, "("},
		{itemInteger, 0, "1"},
		{itemNotEq, 0, "!="},
		{itemInteger, 0, "1"},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"empty list", `{[]}`, []item{
		tLeft,
		{itemLeftBracket, 0, "["},
		{itemRightBracket, 0, "]"},
		tRight,
		tEOF,
	}},

	{"list", `{[1, 'two', [3, false]]}`, []item{
		tLeft,
		{itemLeftBracket, 0, "["},
		{itemInteger, 0, "1"},
		{itemComma, 0, ","},
		{itemString, 0, "'two'"},
		{itemComma, 0, ","},
		{itemLeftBracket, 0, "["},
		{itemInteger, 0, "3"},
		{itemComma, 0, ","},
		{itemBool, 0, "false"},
		{itemRightBracket, 0, "]"},
		{itemRightBracket, 0, "]"},
		tRight,
		tEOF,
	}},

	{"empty map", `{[:]}`, []item{
		tLeft,
		{itemLeftBracket, 0, "["},
		{itemColon, 0, ":"},
		{itemRightBracket, 0, "]"},
		tRight,
		tEOF,
	}},

	{"map", `{['aaa': 42, 'bbb': 'hello', 'ccc':[1]]}`, []item{
		tLeft,
		{itemLeftBracket, 0, "["},
		{itemString, 0, "'aaa'"},
		{itemColon, 0, ":"},
		{itemInteger, 0, "42"},
		{itemComma, 0, ","},
		{itemString, 0, "'bbb'"},
		{itemColon, 0, ":"},
		{itemString, 0, "'hello'"},
		{itemComma, 0, ","},
		{itemString, 0, "'ccc'"},
		{itemColon, 0, ":"},
		{itemLeftBracket, 0, "["},
		{itemInteger, 0, "1"},
		{itemRightBracket, 0, "]"},
		{itemRightBracket, 0, "]"},
		tRight,
		tEOF,
	}},

	{"functions", `{foo(5, $foo)}`, []item{
		tLeft,
		{itemIdent, 0, "foo"},
		{itemLeftParen, 0, "("},
		{itemInteger, 0, "5"},
		{itemComma, 0, ","},
		{itemDollarIdent, 0, "$foo"},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"functions w/ newline", "{foo(\n)}", []item{
		tLeft,
		{itemIdent, 0, "foo"},
		{itemLeftParen, 0, "("},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"msg", `{msg meaning="verb" desc="msg description"}Hello{/msg}`, []item{
		tLeft,
		{itemMsg, 0, "msg"},
		{itemIdent, 0, "meaning"},
		{itemEquals, 0, "="},
		{itemString, 0, `"verb"`},
		{itemIdent, 0, "desc"},
		{itemEquals, 0, "="},
		{itemString, 0, `"msg description"`},
		tRight,
		{itemText, 0, "Hello"},
		tLeft,
		{itemMsgEnd, 0, "/msg"},
		tRight,
		tEOF,
	}},

	{"data ref", "{$boo.0?.50['foo'+'bar'].baz[5]?.goo}", []item{
		tLeft,
		{itemDollarIdent, 0, "$boo"},
		{itemDotIndex, 0, ".0"},
		{itemQuestionDotIndex, 0, "?.50"},
		{itemLeftBracket, 0, "["},
		{itemString, 0, "'foo'"},
		{itemAdd, 0, "+"},
		{itemString, 0, "'bar'"},
		{itemRightBracket, 0, "]"},
		{itemDotIdent, 0, ".baz"},
		{itemLeftBracket, 0, "["},
		{itemInteger, 0, "5"},
		{itemRightBracket, 0, "]"},
		{itemQuestionDotIdent, 0, "?.goo"},
		tRight,
		tEOF,
	}},

	{"literal", `{literal}{$var}{nil} // comment{/literal}`, []item{
		tLeft,
		{itemLiteral, 0, "literal"},
		tRight,
		{itemText, 0, "{$var}{nil} // comment"},
		tLeft,
		{itemLiteralEnd, 0, "/literal"},
		tRight,
		tEOF,
	}},

	{"css", `{css my-class} {css $component, myclass}`, []item{
		tLeft,
		{itemCss, 0, "css"},
		{itemText, 0, "my-class"},
		tRight,
		{itemText, 0, " "},
		tLeft,
		{itemCss, 0, "css"},
		{itemText, 0, "$component, myclass"},
		tRight,
		tEOF,
	}},

	{"log", `{log} some expr {$expr}{/log}`, []item{
		tLeft,
		{itemLog, 0, "log"},
		tRight,
		{itemText, 0, " some expr "},
		tLeft,
		{itemDollarIdent, 0, "$expr"},
		tRight,
		tLeft,
		{itemLogEnd, 0, "/log"},
		tRight,
		tEOF,
	}},

	{"debugger", `{debugger}`, []item{
		tLeft,
		{itemDebugger, 0, "debugger"},
		tRight,
		tEOF,
	}},

	{"print directives", `{$var|a}{'a'|a|b}{|a |b}{|a:23 |b:'a',5}`, []item{
		tLeft,
		{itemDollarIdent, 0, "$var"},
		{itemPipe, 0, "|"},
		{itemIdent, 0, "a"},
		tRight,
		tLeft,
		{itemString, 0, "'a'"},
		{itemPipe, 0, "|"},
		{itemIdent, 0, "a"},
		{itemPipe, 0, "|"},
		{itemIdent, 0, "b"},
		tRight,
		tLeft,
		{itemPipe, 0, "|"},
		{itemIdent, 0, "a"},
		{itemPipe, 0, "|"},
		{itemIdent, 0, "b"},
		tRight,
		tLeft,
		{itemPipe, 0, "|"},
		{itemIdent, 0, "a"},
		{itemColon, 0, ":"},
		{itemInteger, 0, "23"},
		{itemPipe, 0, "|"},
		{itemIdent, 0, "b"},
		{itemColon, 0, ":"},
		{itemString, 0, "'a'"},
		{itemComma, 0, ","},
		{itemInteger, 0, "5"},
		tRight,
		tEOF,
	}},

	{"global", `{GLOBAL_STR}{app.GLOBAL}`, []item{
		tLeft,
		{itemIdent, 0, "GLOBAL_STR"},
		tRight,
		tLeft,
		{itemIdent, 0, "app"},
		{itemDotIdent, 0, ".GLOBAL"},
		tRight,
		tEOF,
	}},

	{"line comment", ` // this is a {comment} `, []item{
		{itemComment, 0, "// this is a {comment} "},
		tEOF,
	}},
	{"line comment2", " // this is a {comment}\n//\n{/log}", []item{
		{itemComment, 0, "// this is a {comment}\n"},
		{itemComment, 0, "//\n"},
		tLeft,
		{itemLogEnd, 0, "/log"},
		tRight,
		tEOF,
	}},
	{"line comment3", "a // this is a {comment} \n", []item{
		{itemText, 0, "a"},
		{itemComment, 0, "// this is a {comment} \n"},
		tEOF,
	}},
	{"line comment4", "{a}//not a comment", []item{
		tLeft,
		{itemIdent, 0, "a"},
		tRight,
		{itemText, 0, "//not a comment"},
		tEOF,
	}},

	{"block comment", "/* this is a {comment} \n * multi line \n */", []item{
		{itemComment, 0, "/* this is a {comment} \n * multi line \n */"},
		tEOF,
	}},
	{"soydoc", `/** this is a soydoc comment */`, []item{
		{itemSoyDocStart, 0, "/**"},
		{itemText, 0, `this is a soydoc comment `},
		{itemSoyDocEnd, 0, "*/"},
		tEOF,
	}},
	{"soydoc", `/** @param name */`, []item{
		{itemSoyDocStart, 0, "/**"},
		{itemSoyDocParam, 0, "@param"},
		{itemIdent, 0, `name`},
		{itemSoyDocEnd, 0, "*/"},
		tEOF,
	}},
	{"soydoc", `
/**
 * This is a soydoc comment
 * @param boo scary
 * @param? goo slimy (optional)
 */`, []item{
		{itemSoyDocStart, 0, "/**"},
		{itemText, 0, "This is a soydoc comment"},
		{itemSoyDocParam, 0, "@param"},
		{itemIdent, 0, "boo"},
		{itemText, 0, "scary"},
		{itemSoyDocOptionalParam, 0, "@param?"},
		{itemIdent, 0, "goo"},
		{itemText, 0, "slimy (optional)"},
		{itemSoyDocEnd, 0, "*/"},
		tEOF,
	}},

	{"soydoc2", `
/**
 * @param boo
 * @param? goo
 */`, []item{
		{itemSoyDocStart, 0, "/**"},
		{itemSoyDocParam, 0, "@param"},
		{itemIdent, 0, "boo"},
		{itemSoyDocOptionalParam, 0, "@param?"},
		{itemIdent, 0, "goo"},
		{itemSoyDocEnd, 0, "*/"},
		tEOF,
	}},

	{"let", `{let $ident: 1+1/}{let $ident}content{/let}`, []item{
		tLeft,
		{itemLet, 0, "let"},
		{itemDollarIdent, 0, "$ident"},
		{itemColon, 0, ":"},
		{itemInteger, 0, "1"},
		{itemAdd, 0, "+"},
		{itemInteger, 0, "1"},
		{itemRightDelimEnd, 0, "/}"},
		tLeft,
		{itemLet, 0, "let"},
		{itemDollarIdent, 0, "$ident"},
		tRight,
		{itemText, 0, "content"},
		tLeft,
		{itemLetEnd, 0, "/let"},
		tRight,
		tEOF,
	}},

	{"alias", `{alias a.b.c}`, []item{
		tLeft,
		{itemAlias, 0, "alias"},
		{itemIdent, 0, "a"},
		{itemDotIdent, 0, ".b"},
		{itemDotIdent, 0, ".c"},
		tRight,
		tEOF,
	}},

	{"namespace and template", `{namespace example}

{template .templateName}
Hello world.
{/template}
`, []item{
		tLeft,
		{itemNamespace, 0, "namespace"},
		{itemIdent, 0, "example"},
		tRight,
		tLeft,
		{itemTemplate, 0, "template"},
		{itemDotIdent, 0, ".templateName"},
		tRight,
		{itemText, 0, "\nHello world.\n"},
		tLeft,
		{itemTemplateEnd, 0, "/template"},
		tRight,
		tEOF,
	}},
}

// collect gathers the emitted items into a slice.
func collect(t *lexTest) (items []item) {
	l := lex(t.name, t.input)
	for {
		item := l.nextItem()
		items = append(items, item)
		if item.typ == itemEOF || item.typ == itemError {
			break
		}
	}
	return
}

func equal(i1, i2 []item, checkPos bool) bool {
	if len(i1) != len(i2) {
		return false
	}
	for k := range i1 {
		if i1[k].typ != i2[k].typ {
			return false
		}
		if i1[k].val != i2[k].val {
			return false
		}
		if checkPos && i1[k].pos != i2[k].pos {
			return false
		}
	}
	return true
}

func TestLex(t *testing.T) {
	for _, test := range lexTests {
		items := collect(&test)
		if !equal(items, test.items, false) {
			t.Errorf("%s: got\n\t%+v\nexpected\n\t%v", test.name, items, test.items)
		}
	}
}

func TestScanNumber(t *testing.T) {
	validIntegers := []string{
		// Decimal.
		"0",
		"-1",
		"42",
		"-827",
		// Hexadecimal.
		"0x1A2B",
	}
	invalidIntegers := []string{
		// Decimal.
		"042",
		"-0827",
		// Hexadecimal.
		"-0x1A2B",
		"0X1A2B",
		"0x1a2b",
		"0x1A2B.2B",
	}
	validFloats := []string{
		"0.5",
		"-100.0",
		"-3e-3",
		"6.02e23",
		"5.1e-9",
	}
	invalidFloats := []string{
		".5",
		"-.5",
		"100.",
		"-100.",
		"-3E-3",
		"6.02E23",
		"5.1E-9",
		"-3e",
		"6.02e",
	}

	for _, v := range validIntegers {
		l := lexExpr("", v)
		item := <-l.items
		if item.typ != itemInteger {
			t.Fatalf("Expected a valid integer for %q, got %v", v, item.val)
		}
		if item.val != v {
			t.Fatalf("Expected %q, got %q", v, item.val)
		}
		if err := <-l.items; err.typ != itemError {
			t.Fatalf("Expected EOF, got %v", err)
		}
	}
	for _, v := range invalidIntegers {
		l := lexExpr("", v)
		item := <-l.items
		if item.typ != itemError {
			t.Fatalf("Expected an invalid integer for %q, got %v", v, item)
		}
	}
	for _, v := range validFloats {
		l := lexExpr("", v)
		item := <-l.items
		if item.typ != itemFloat {
			t.Fatalf("Expected a valid float for %q", v)
		}
		if item.val != v {
			t.Fatalf("Expected %q, got %q", v, item.val)
		}
		if err := <-l.items; err.typ != itemError {
			t.Fatalf("Expected EOF, got %v", err)
		}
	}
	for _, v := range invalidFloats {
		l := lexExpr("", v)
		item := <-l.items
		if item.typ == itemFloat {
			t.Fatalf("Expected an invalid float for %q, got %v", v, item.typ)
		}
	}
}

// 	validNamespaces := []string{
// 		"{namespace example}",
// 		"{namespace examples.simple}",
// 		"{namespace examples.simple.three}",
// 		`{namespace examples.simple.three autoescape="contextual"}`,
// 		`{namespace  extraspace  autoescape="off" }`,
// 	}
// 	invalidNamespaces := []string{
// 		"{namespaceexample}",
// 		"{namespace no spaces}",
// 		"{namespace\n}",
// 	}
// 	for _, v := range validNamespaces {
// 		l := newLexer("", v)
// 		typ, ok := scanNumber(l)
// 		res := l.input[l.start:l.pos]
// 		if !ok || typ != itemInteger {
// 			t.Fatalf("Expected a valid integer for %q", v)
// 		}
// 		if res != v {
// 			t.Fatalf("Expected %q, got %q", v, res)
// 		}
// 	}

// }
