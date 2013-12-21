package parse

import (
	"fmt"

	"testing"
)

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
	{"variable", `{$name.bar}`, []item{
		tLeft,
		{itemVariable, 0, "$name"},
		{itemDot, 0, "."},
		{itemIdent, 0, "bar"},
		tRight,
		tEOF,
	}},
	{"soydoc", `/** this is a comment */`, []item{
		{itemSoyDocStart, 0, ""},
		{itemText, 0, `/** this is a comment */`},
		{itemSoyDocEnd, 0, ""},
		tEOF,
	}},
	{"if", `{if $var}{$var} is true{/if}`, []item{
		tLeft,
		{itemIf, 0, "if"},
		{itemVariable, 0, "$var"},
		tRight,
		tLeft,
		{itemVariable, 0, "$var"},
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
		{itemVariable, 0, "$var"},
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

	{"special characters", `{sp}{nil}{\r}{\n}{\t}{lb}{rb}`, []item{
		tLeft, {itemSpace, 0, "sp"}, // {sp}
		tRight, tLeft, {itemEmptyString, 0, "nil"}, // {nil}
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
		{itemVariable, 0, "$foo"},
		{itemIdent, 0, "in"},
		{itemVariable, 0, "$bars"},
		tRight,
		tLeft, {itemVariable, 0, "$foo"}, tRight,
		tLeft, {itemIfempty, 0, "ifempty"}, tRight,
		{itemText, 0, "No bars"},
		tLeft, {itemForeachEnd, 0, "/foreach"}, tRight,
		tEOF,
	}},

	{"switch", `{switch $boo} {case 0}Blah{case $foo.goo}Bleh{case -1, 1, $moo} {default}{/switch}`, []item{
		tLeft,
		{itemSwitch, 0, "switch"},
		{itemVariable, 0, "$boo"},
		tRight,
		tLeft,
		{itemCase, 0, "case"},
		{itemInteger, 0, "0"},
		tRight,
		{itemText, 0, "Blah"},
		tLeft,
		{itemCase, 0, "case"},
		{itemVariable, 0, "$foo"},
		{itemDot, 0, "."},
		{itemIdent, 0, "goo"},
		tRight,
		{itemText, 0, "Bleh"},
		tLeft,
		{itemCase, 0, "case"},
		{itemInteger, 0, "-1"},
		{itemComma, 0, ","},
		{itemInteger, 0, "1"},
		{itemComma, 0, ","},
		{itemVariable, 0, "$moo"},
		tRight,
		tLeft,
		{itemDefault, 0, "default"},
		tRight,
		tLeft,
		{itemSwitchEnd, 0, "/switch"},
		tRight,
		tEOF,
	}},

	{"call", `{call .other data="all"/}`, []item{
		tLeft,
		{itemCall, 0, "call"},
		{itemDot, 0, "."},
		{itemIdent, 0, "other"},
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
		{itemVariable, 0, "$var"},
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
		{itemVariable, 0, "$foo"},
		{itemRightParen, 0, ")"},
		tRight,
		tEOF,
	}},

	{"msg", `{msg desc="msg description"}Hello{/msg}`, []item{
		tLeft,
		{itemMsg, 0, "msg"},
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

	{"data ref", "{$boo.0['foo'+'bar'][5]?.goo}", []item{
		tLeft,
		{itemVariable, 0, "$boo"},
		{itemDot, 0, "."},
		{itemInteger, 0, "0"},
		{itemLeftBracket, 0, "["},
		{itemString, 0, "'foo'"},
		{itemAdd, 0, "+"},
		{itemString, 0, "'bar'"},
		{itemRightBracket, 0, "]"},
		{itemLeftBracket, 0, "["},
		{itemInteger, 0, "5"},
		{itemRightBracket, 0, "]"},
		{itemQuestionDot, 0, "?."},
		{itemIdent, 0, "goo"},
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
		{itemDot, 0, "."},
		{itemIdent, 0, "templateName"},
		tRight,
		{itemText, 0, "\nHello world.\n"},
		tLeft,
		{itemTemplateEnd, 0, "/template"},
		tRight,
		tEOF,
	}},
}

var itemName = map[itemType]string{
	itemError:     "error",
	itemTemplate:  "template",
	itemNamespace: "namespace",
}

func (i itemType) String() string {
	s := itemName[i]
	if s == "" {
		return fmt.Sprintf("item%d", int(i))
	}
	return s
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
		l := lex("", v)
		typ, ok := scanNumber(l)
		res := l.input[l.start:l.pos]
		if !ok || typ != itemInteger {
			t.Fatalf("Expected a valid integer for %q", v)
		}
		if res != v {
			t.Fatalf("Expected %q, got %q", v, res)
		}
	}
	for _, v := range invalidIntegers {
		l := lex("", v)
		_, ok := scanNumber(l)
		if ok {
			t.Fatalf("Expected an invalid integer for %q", v)
		}
	}
	for _, v := range validFloats {
		l := lex("", v)
		typ, ok := scanNumber(l)
		res := l.input[l.start:l.pos]
		if !ok || typ != itemFloat {
			t.Fatalf("Expected a valid float for %q", v)
		}
		if res != v {
			t.Fatalf("Expected %q, got %q", v, res)
		}
	}
	for _, v := range invalidFloats {
		l := lex("", v)
		_, ok := scanNumber(l)
		if ok {
			t.Fatalf("Expected an invalid float for %q", v)
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
