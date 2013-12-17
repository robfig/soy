package parse

import (
	"fmt"

	"testing"
)

// func TestScanNumber(t *testing.T) {
// 	validIntegers := []string{
// 		// Decimal.
// 		"42",
// 		"-827",
// 		// Hexadecimal.
// 		"0x1A2B",
// 	}
// 	invalidIntegers := []string{
// 		// Decimal.
// 		"042",
// 		"-0827",
// 		// Hexadecimal.
// 		"-0x1A2B",
// 		"0X1A2B",
// 		"0x1a2b",
// 		"0x1A2B.2B",
// 	}
// 	validFloats := []string{
// 		"0.5",
// 		"-100.0",
// 		"-3e-3",
// 		"6.02e23",
// 		"5.1e-9",
// 	}
// 	invalidFloats := []string{
// 		".5",
// 		"-.5",
// 		"100.",
// 		"-100.",
// 		"-3E-3",
// 		"6.02E23",
// 		"5.1E-9",
// 		"-3e",
// 		"6.02e",
// 	}

// 	for _, v := range validIntegers {
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
// 	for _, v := range invalidIntegers {
// 		l := newLexer("", v)
// 		_, ok := scanNumber(l)
// 		if ok {
// 			t.Fatalf("Expected an invalid integer for %q", v)
// 		}
// 	}
// 	for _, v := range validFloats {
// 		l := newLexer("", v)
// 		typ, ok := scanNumber(l)
// 		res := l.input[l.start:l.pos]
// 		if !ok || typ != itemFloat {
// 			t.Fatalf("Expected a valid float for %q", v)
// 		}
// 		if res != v {
// 			t.Fatalf("Expected %q, got %q", v, res)
// 		}
// 	}
// 	for _, v := range invalidFloats {
// 		l := newLexer("", v)
// 		_, ok := scanNumber(l)
// 		if ok {
// 			t.Fatalf("Expected an invalid float for %q", v)
// 		}
// 	}
// }

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
	{"variable", `{$name}`, []item{
		tLeft,
		{itemVariable, 0, "$name"},
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
	{"not", `{not $var}`, []item{
		tLeft,
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
		{itemIdent, 0, ".templateName"},
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
