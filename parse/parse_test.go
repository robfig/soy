package parse

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
)

type parseTest struct {
	name  string
	input string
	tree  ast.Node
}

const (
	noError  = true
	hasError = false
)

func newList(pos ast.Pos) *ast.ListNode {
	return &ast.ListNode{Pos: pos}
}

func tList(nodes ...ast.Node) ast.Node {
	n := newList(0)
	n.Nodes = nodes
	return n
}

func tFile(nodes ...ast.Node) ast.Node {
	return &ast.SoyFileNode{Body: nodes}
}

func tTemplate(name string, nodes ...ast.Node) ast.Node {
	n := &ast.TemplateNode{0, name, nil, ast.AutoescapeOn, false}
	n.Body = newList(0)
	n.Body.Nodes = nodes
	return n
}

func newText(pos ast.Pos, text string) *ast.RawTextNode {
	return &ast.RawTextNode{Pos: pos, Text: []byte(text)}
}

func bin(n1, n2 ast.Node) ast.BinaryOpNode {
	return ast.BinaryOpNode{Arg1: n1, Arg2: n2}
}

func str(value string) *ast.StringNode {
	return &ast.StringNode{0, quoteString(value), value}
}

var parseTests = []parseTest{
	{"empty", "", tFile()},
	{"namespace", "{namespace soy.example}", tFile(&ast.NamespaceNode{0, "soy.example", 0})},
	{"empty template", "{template .name}{/template}", tFile(tTemplate(".name"))},
	{"text template", "{template .name}\nHello world!\n{/template}",
		tFile(tTemplate(".name", newText(0, "Hello world!")))},
	{"variable template", "{template .name}\nHello {$name}!\n{/template}",
		tFile(tTemplate(".name",
			newText(0, "Hello "),
			&ast.PrintNode{0, &ast.DataRefNode{0, "name", nil}, nil}, // implicit print
			newText(0, "!"),
		))},
	{"not", "{not $var}", tFile(&ast.PrintNode{0, &ast.NotNode{0, &ast.DataRefNode{0, "var", nil}}, nil})},
	{"negate", "{-$var}", tFile(&ast.PrintNode{0, &ast.NegateNode{0, &ast.DataRefNode{0, "var", nil}}, nil})},
	{"concat", `{'hello' + 'world'}`, tFile(&ast.PrintNode{0, &ast.AddNode{bin(
		str("hello"),
		str("world"))}, nil})},
	{"explicit print", `{print 'hello'}`, tFile(&ast.PrintNode{0, str("hello"), nil})},
	{"print directive", `{print 'hello'|id}`, tFile(&ast.PrintNode{0, str("hello"),
		[]*ast.PrintDirectiveNode{{0, "id", nil}}})},
	{"print directives", `{'hello'|noAutoescape|truncate:5,false}`, tFile(&ast.PrintNode{0, str("hello"),
		[]*ast.PrintDirectiveNode{
			{0, "noAutoescape", nil},
			{0, "truncate", []ast.Node{
				&ast.IntNode{0, 5},
				&ast.BoolNode{0, false}}}}})},

	{"soydoc", `/**
 * Text
 * @param boo scary description
 * @param? goo slimy
 */`, tFile(&ast.SoyDocNode{0, []*ast.SoyDocParamNode{
		{0, "boo", false},
		{0, "goo", true},
	}})},
	{"soydoc - one line", "/** @param name */", tFile(&ast.SoyDocNode{0, []*ast.SoyDocParamNode{
		{0, "name", false},
	}})},

	{"rawtext (linejoin)", "\n  a \n\tb\r\n  c  \n\n", tFile(newText(0, "a b c"))},
	{"rawtext+html", "\n  a <br>\n\tb\r\n\n  c\n\n<br> ", tFile(newText(0, "a <br>b c<br> "))},
	{"rawtext+comment", "a <br> // comment \n\tb\t// comment2\r\n  c\n\n", tFile(
		newText(0, "a <br>"),
		newText(0, "b"),
		newText(0, "c"),
	)},
	{"rawtext+tag", "a {$foo}\t {$baz}\n\t  b\r\n\n  {$bar} c", tFile(
		newText(0, "a "),
		&ast.PrintNode{0, &ast.DataRefNode{0, "foo", nil}, nil},
		newText(0, "\t "),
		&ast.PrintNode{0, &ast.DataRefNode{0, "baz", nil}, nil},
		newText(0, "b"),
		&ast.PrintNode{0, &ast.DataRefNode{0, "bar", nil}, nil},
		newText(0, " c"),
	)},
	{"rawtext+tag+html+comment", `
  {$italicHtml}<br>  // a {comment}
  {$italicHtml |noAutoescape}<br>  /* {a }comment */
  abc`, tFile(
		&ast.PrintNode{0, &ast.DataRefNode{0, "italicHtml", nil}, nil},
		newText(0, "<br>"),
		&ast.PrintNode{0, &ast.DataRefNode{0, "italicHtml", nil}, []*ast.PrintDirectiveNode{
			{0, "noAutoescape", nil}}},
		newText(0, "<br>"),
		newText(0, "abc"),
	)},

	{"specialchars", `{sp}{nil}{\r}{\n}{\t}{lb}{rb}`, tFile(
		newText(0, " "),
		newText(0, ""),
		newText(0, "\r"),
		newText(0, "\n"),
		newText(0, "\t"),
		newText(0, "{"),
		newText(0, "}"),
	)},
	{"specialchars inside string", `{'abc\ndef'}`, tFile(
		&ast.PrintNode{0, str("abc\ndef"), nil},
	)},

	{"literal", "{literal} {/call}\n {sp} // comment {/literal}", tFile(
		newText(0, " {/call}\n {sp} // comment "),
	)},

	{"css", `{css my-class} {css $component, myclass}`, tFile(
		&ast.CssNode{0, nil, "my-class"},
		newText(0, " "),
		&ast.CssNode{0, &ast.DataRefNode{0, "component", nil}, "myclass"},
	)},

	{"log", "{log}Hello {$name}{/log}", tFile(
		&ast.LogNode{0, tList(
			newText(0, "Hello "),
			&ast.PrintNode{0, &ast.DataRefNode{0, "name", nil}, nil},
		)},
	)},
	{"log+comment", "{log}Hello {$name} // comment\n{/log}", tFile(
		&ast.LogNode{0, tList(
			newText(0, "Hello "),
			&ast.PrintNode{0, &ast.DataRefNode{0, "name", nil}, nil},
		)},
	)},

	{"debugger", "{debugger}", tFile(&ast.DebuggerNode{0})},
	{"global", "{GLOBAL_STR}{app.GLOBAL}", tFile(
		&ast.PrintNode{0, &ast.GlobalNode{0, "GLOBAL_STR", data.String("a")}, nil},
		&ast.PrintNode{0, &ast.GlobalNode{0, "app.GLOBAL", data.String("b")}, nil},
	)},

	{"expression1", "{not false and (isFirst($foo) or (-$x - 5) > 3.1)}", tFile(&ast.PrintNode{0, &ast.AndNode{bin(
		&ast.NotNode{0, &ast.BoolNode{0, false}},
		&ast.OrNode{bin(
			&ast.FunctionNode{0, "isFirst", []ast.Node{&ast.DataRefNode{0, "foo", nil}}},
			&ast.GtNode{bin(
				&ast.SubNode{bin(
					&ast.NegateNode{0, &ast.DataRefNode{0, "x", nil}},
					&ast.IntNode{0, 5})},
				&ast.FloatNode{0, 3.1})})})}, nil})},

	{"expression2", `{null or ('foo' == 'f'+true ? -3 <= 5 : not $foo ?: bar(5))}`, tFile(&ast.PrintNode{0, &ast.OrNode{bin(
		&ast.NullNode{0},
		&ast.TernNode{0,
			&ast.EqNode{bin(
				str("foo"),
				&ast.AddNode{bin(
					str("f"),
					&ast.BoolNode{0, true})})},
			&ast.LteNode{bin(
				&ast.IntNode{0, -3},
				&ast.IntNode{0, 5})},
			&ast.ElvisNode{bin(
				&ast.NotNode{0, &ast.DataRefNode{0, "foo", nil}},
				&ast.FunctionNode{0, "bar", []ast.Node{&ast.IntNode{0, 5}}})}})}, nil})},

	{"expression3", `{'a'+'b' != 'ab' and (2 >= -5.0 or (null ?: true))}`, tFile(&ast.PrintNode{0, &ast.AndNode{bin(
		&ast.NotEqNode{bin(
			&ast.AddNode{bin(
				str("a"),
				str("b"))},
			str("ab"))},
		&ast.OrNode{bin(
			&ast.GteNode{bin(&ast.IntNode{0, 2}, &ast.FloatNode{0, -5.0})},
			&ast.ElvisNode{bin(
				&ast.NullNode{0},
				&ast.BoolNode{0, true})})})}, nil})},

	{"sub", `{1.0-0.5}`, tFile(&ast.PrintNode{0, &ast.SubNode{bin(
		&ast.FloatNode{0, 1.0},
		&ast.FloatNode{0, 0.5},
	)}, nil})},

	{"function", `{hasData()}`, tFile(&ast.PrintNode{0, &ast.FunctionNode{0, "hasData", nil}, nil})},

	{"empty list", `{[]}`, tFile(&ast.PrintNode{0, &ast.ListLiteralNode{0, nil}, nil})},

	{"list", `{[1, 'two', [3, false]]}`, tFile(&ast.PrintNode{0, &ast.ListLiteralNode{0, []ast.Node{
		&ast.IntNode{0, 1},
		str("two"),
		&ast.ListLiteralNode{0, []ast.Node{
			&ast.IntNode{0, 3},
			&ast.BoolNode{0, false},
		}},
	}}, nil})},

	{"empty map", `{[:]}`, tFile(&ast.PrintNode{0, &ast.MapLiteralNode{0, make(map[string]ast.Node)}, nil})},

	{"map", `{['aaa': 42, 'bbb': 'hello', 'ccc':[1]]}`, tFile(&ast.PrintNode{0, &ast.MapLiteralNode{0, map[string]ast.Node{
		"aaa": &ast.IntNode{0, 42},
		"bbb": str("hello"),
		"ccc": &ast.ListLiteralNode{0, []ast.Node{&ast.IntNode{0, 1}}},
	}}, nil})},

	{"if", `
{if $zoo}{$zoo}{/if}
{if $boo}
  Blah
{elseif $foo.goo > 2}
  {$boo}
{else}
  Blah {$moo}
{/if}`, tFile(
		&ast.IfNode{0, []*ast.IfCondNode{
			&ast.IfCondNode{0, &ast.DataRefNode{0, "zoo", nil}, tList(&ast.PrintNode{0, &ast.DataRefNode{0, "zoo", nil}, nil})},
		}},
		&ast.IfNode{0, []*ast.IfCondNode{
			&ast.IfCondNode{0, &ast.DataRefNode{0, "boo", nil}, tList(newText(0, "Blah"))},
			&ast.IfCondNode{0,
				&ast.GtNode{bin(
					&ast.DataRefNode{0, "foo", []ast.Node{&ast.DataRefKeyNode{0, false, "goo"}}},
					&ast.IntNode{0, 2})},
				tList(&ast.PrintNode{0, &ast.DataRefNode{0, "boo", nil}, nil})},
			&ast.IfCondNode{0,
				nil,
				tList(newText(0, "Blah "), &ast.PrintNode{0, &ast.DataRefNode{0, "moo", nil}, nil})},
		}},
	)},

	{"switch", `
{switch $boo} {case 0}Blah
  {case $foo.goo}
    Bleh
  {case -1, 1+1, $moo}
    Bluh
  {default}
    Bloh
{/switch}`, tFile(
		&ast.SwitchNode{0, &ast.DataRefNode{0, "boo", nil}, []*ast.SwitchCaseNode{
			&ast.SwitchCaseNode{0, []ast.Node{&ast.IntNode{0, 0}}, tList(newText(0, "Blah"))},
			&ast.SwitchCaseNode{0, []ast.Node{&ast.DataRefNode{0, "foo", []ast.Node{&ast.DataRefKeyNode{0, false, "goo"}}}},
				tList(newText(0, "Bleh"))},
			&ast.SwitchCaseNode{0, []ast.Node{
				&ast.IntNode{0, -1},
				&ast.AddNode{bin(&ast.IntNode{0, 1}, &ast.IntNode{0, 1})},
				&ast.DataRefNode{0, "moo", nil}}, tList(newText(0, "Bluh"))},
			&ast.SwitchCaseNode{0, nil, tList(newText(0, "Bloh"))},
		}},
	)},

	{"foreach", `
{foreach $goo in $goose}
  {$goose.numKids} goslings.{\n}
{/foreach}
{foreach $boo in $foo.booze}
  Scary drink {$boo.name}!
  {if not isLast($boo)}{\n}{/if}
{ifempty}
  Sorry, no booze.
{/foreach}`, tFile(
		&ast.ForNode{0, "goo", &ast.DataRefNode{0, "goose", nil}, tList(
			&ast.PrintNode{0, &ast.DataRefNode{0, "goose", []ast.Node{&ast.DataRefKeyNode{0, false, "numKids"}}}, nil},
			newText(0, " goslings."),
			newText(0, "\n"),
		), nil},
		&ast.ForNode{0, "boo", &ast.DataRefNode{0, "foo", []ast.Node{&ast.DataRefKeyNode{0, false, "booze"}}},
			tList(
				newText(0, "Scary drink "),
				&ast.PrintNode{0, &ast.DataRefNode{0, "boo", []ast.Node{&ast.DataRefKeyNode{0, false, "name"}}}, nil},
				newText(0, "!"),
				&ast.IfNode{0,
					[]*ast.IfCondNode{&ast.IfCondNode{0,
						&ast.NotNode{0, &ast.FunctionNode{0, "isLast", []ast.Node{&ast.DataRefNode{0, "boo", nil}}}},
						tList(newText(0, "\n"))}}}),
			tList(
				newText(0, "Sorry, no booze."))},
	)},

	{"for", `
{for $i in range(1, $items.length + 1)}
  {msg meaning="verb" desc="Numbered item."}
    {$i}: {$items[$i - 1]}{\n}
  {/msg}
{/for}`, tFile(
		&ast.ForNode{0, "i",
			&ast.FunctionNode{0, "range", []ast.Node{
				&ast.IntNode{0, 1},
				&ast.AddNode{bin(
					&ast.DataRefNode{0, "items", []ast.Node{&ast.DataRefKeyNode{0, false, "length"}}},
					&ast.IntNode{0, 1})}}},
			tList(
				&ast.MsgNode{0, "verb", "Numbered item.", []ast.Node{
					&ast.MsgPlaceholderNode{0,
						&ast.PrintNode{0, &ast.DataRefNode{0, "i", nil}, nil}},
					newText(0, ": "),
					&ast.MsgPlaceholderNode{0,
						&ast.PrintNode{0, &ast.DataRefNode{0, "items", []ast.Node{
							&ast.DataRefExprNode{0, false,
								&ast.SubNode{bin(
									&ast.DataRefNode{0, "i", nil},
									&ast.IntNode{0, 1})}}}}, nil}},
					newText(0, "\n"), // {\n}
				}}),
			nil},
	)},

	{"data ref", "{$boo.0['foo'+'bar'][5]?.goo}", tFile(&ast.PrintNode{0, &ast.DataRefNode{0, "boo", []ast.Node{
		&ast.DataRefIndexNode{0, false, 0},
		&ast.DataRefExprNode{0,
			false,
			&ast.AddNode{bin(
				str("foo"),
				str("bar"))}},
		&ast.DataRefExprNode{0, false, &ast.IntNode{0, 5}},
		&ast.DataRefKeyNode{0, true, "goo"}},
	}, nil})},

	{"call", `
{call name=".booTemplate_" /}
{call name="foo.goo.mooTemplate" data="all" /}
{call name=".zooTemplate" data="$animals"}
  // comments are allowed here

  {param key="yoo" value="round($too)" /}
  {param key="woo"}poo{/param}
  {param key="doo" kind="html"}doopoo{/param}
{/call}
{call a.long.template.booTemplate_ /}
{call .zooTemplate data="$animals"}
  {param yoo: round($too) /}
  {param woo}poo{/param}
  {param zoo: 0 /}
  {param doo kind="html"}doopoo{/param}
{/call}`, tFile(
		&ast.CallNode{0, ".booTemplate_", false, nil, nil},
		&ast.CallNode{0, "foo.goo.mooTemplate", true, nil, nil},
		&ast.CallNode{0, ".zooTemplate", false, &ast.DataRefNode{0, "animals", nil}, []ast.Node{
			&ast.CallParamValueNode{0, "yoo", &ast.FunctionNode{0, "round", []ast.Node{&ast.DataRefNode{0, "too", nil}}}},
			&ast.CallParamContentNode{0, "woo", tList(newText(0, "poo"))},
			&ast.CallParamContentNode{0, "doo", tList(newText(0, "doopoo"))}}},
		&ast.CallNode{0, "a.long.template.booTemplate_", false, nil, nil},
		&ast.CallNode{0, ".zooTemplate", false, &ast.DataRefNode{0, "animals", nil}, []ast.Node{
			&ast.CallParamValueNode{0, "yoo", &ast.FunctionNode{0, "round", []ast.Node{&ast.DataRefNode{0, "too", nil}}}},
			&ast.CallParamContentNode{0, "woo", tList(newText(0, "poo"))},
			&ast.CallParamValueNode{0, "zoo", &ast.IntNode{0, 0}},
			&ast.CallParamContentNode{0, "doo", tList(newText(0, "doopoo"))}}},
	)},

	{"let", `
{let $alpha: $boo.foo /}
{let $beta}Boo!{/let}
`, /*{let $delta kind="html"}Boo!{/let}*/ tFile(
		&ast.LetValueNode{0, "alpha", &ast.DataRefNode{0, "boo", []ast.Node{&ast.DataRefKeyNode{0, false, "foo"}}}},
		&ast.LetContentNode{0, "beta", tList(newText(0, "Boo!"))},
	)},

	{"comments", `
  {sp}  // {sp}
  /* {sp} {sp} */  // {sp}
  /* {sp} */{sp}/* {sp} */
  /* {sp}
  {sp} */{sp}
  // {sp} /* {sp} */
  http://www.google.com`, tFile(
		newText(0, " "),
		newText(0, " "),
		newText(0, " "),
		newText(0, "http://www.google.com"),
	)},

	{"alias", `{alias a.b.c}{call c.d/}`, tFile(
		&ast.CallNode{0, "a.b.c.d", false, nil, nil},
	)},
}

var globals = data.Map{
	"GLOBAL_STR": data.String("a"),
	"app.GLOBAL": data.String("b"),
}

func TestParse(t *testing.T) {
	for _, test := range parseTests {
		tmpl, err := SoyFile(test.name, test.input, globals)

		switch {
		// case err == nil && !test.ok:
		// 	t.Errorf("%q: expected error; got none", test.name)
		// 	continue
		case err != nil:
			t.Errorf("%q: unexpected error: %v", test.name, err)
			continue
			// case err != nil && !test.ok:
			// 	// expected error, got one
			// 	continue
		}
		if !eqTree(t, test.tree, tmpl) {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, tmpl, test.tree)
			t.Log("Expected:")
			printTree(t, test.tree, 0)
			t.Log("Actual:")
			if tmpl == nil {
				t.Log("<nil>")
			} else {
				printTree(t, tmpl, 0)
			}
		}
	}
}

func eqTree(t *testing.T, expected, actual ast.Node) bool {
	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("expected %T, got %T", expected, actual)
		return false
	}

	if actual == nil && expected == nil {
		return true
	}

	switch actual.(type) {
	case *ast.SoyFileNode:
		return eqNodes(t, expected.(*ast.SoyFileNode).Body, actual.(*ast.SoyFileNode).Body)
	case *ast.ListNode:
		return eqNodes(t, expected.(*ast.ListNode).Nodes, actual.(*ast.ListNode).Nodes)
	case *ast.NamespaceNode:
		return eqstr(t, "namespace", expected.(*ast.NamespaceNode).Name, actual.(*ast.NamespaceNode).Name)
	case *ast.TemplateNode:
		if expected.(*ast.TemplateNode).Name != actual.(*ast.TemplateNode).Name {
			return false
		}
		return eqTree(t, expected.(*ast.TemplateNode).Body, actual.(*ast.TemplateNode).Body)
	case *ast.RawTextNode:
		return eqstr(t, "text", string(expected.(*ast.RawTextNode).Text), string(actual.(*ast.RawTextNode).Text))
	case *ast.CssNode:
		return eqTree(t, expected.(*ast.CssNode).Expr, actual.(*ast.CssNode).Expr) &&
			eqstr(t, "css", expected.(*ast.CssNode).Suffix, actual.(*ast.CssNode).Suffix)
	case *ast.DebuggerNode:
		return true
	case *ast.LogNode:
		return eqTree(t, expected.(*ast.LogNode).Body, actual.(*ast.LogNode).Body)
	case *ast.LetValueNode:
		return eqstr(t, "let", expected.(*ast.LetValueNode).Name, actual.(*ast.LetValueNode).Name) &&
			eqTree(t, expected.(*ast.LetValueNode).Expr, actual.(*ast.LetValueNode).Expr)
	case *ast.LetContentNode:
		return eqstr(t, "let", expected.(*ast.LetContentNode).Name, actual.(*ast.LetContentNode).Name) &&
			eqTree(t, expected.(*ast.LetContentNode).Body, actual.(*ast.LetContentNode).Body)

	case *ast.NullNode:
		return true
	case *ast.BoolNode:
		return eqbool(t, "bool", expected.(*ast.BoolNode).True, actual.(*ast.BoolNode).True)
	case *ast.IntNode:
		return eqint(t, "int", expected.(*ast.IntNode).Value, actual.(*ast.IntNode).Value)
	case *ast.FloatNode:
		return eqfloat(t, "float", expected.(*ast.FloatNode).Value, actual.(*ast.FloatNode).Value)
	case *ast.StringNode:
		return eqstr(t, "stringnode",
			string(expected.(*ast.StringNode).Value), string(actual.(*ast.StringNode).Value))
	case *ast.GlobalNode:
		return eqstr(t, "global", expected.(*ast.GlobalNode).Name, actual.(*ast.GlobalNode).Name)
	case *ast.ListLiteralNode:
		return eqNodes(t, expected.(*ast.ListLiteralNode).Items, actual.(*ast.ListLiteralNode).Items)
	case *ast.MapLiteralNode:
		e, a := expected.(*ast.MapLiteralNode).Items, actual.(*ast.MapLiteralNode).Items
		if len(e) != len(a) {
			t.Errorf("map differed in size. expected %d, got %d", len(e), len(a))
			return false
		}
		for k, v := range e {
			av := a[k]
			if !eqTree(t, v, av) {
				return false
			}
		}
		return true

	case *ast.DataRefNode:
		return eqstr(t, "var", expected.(*ast.DataRefNode).Key, actual.(*ast.DataRefNode).Key) &&
			eqNodes(t, expected.(*ast.DataRefNode).Access, actual.(*ast.DataRefNode).Access)
	case *ast.DataRefExprNode:
		return eqbool(t, "datarefexpr", expected.(*ast.DataRefExprNode).NullSafe, actual.(*ast.DataRefExprNode).NullSafe) &&
			eqTree(t, expected.(*ast.DataRefExprNode).Arg, actual.(*ast.DataRefExprNode).Arg)
	case *ast.DataRefKeyNode:
		return eqbool(t, "datarefkey", expected.(*ast.DataRefKeyNode).NullSafe, actual.(*ast.DataRefKeyNode).NullSafe) &&
			eqstr(t, "datarefkey", expected.(*ast.DataRefKeyNode).Key, actual.(*ast.DataRefKeyNode).Key)
	case *ast.DataRefIndexNode:
		return eqbool(t, "datarefindex", expected.(*ast.DataRefIndexNode).NullSafe, actual.(*ast.DataRefIndexNode).NullSafe) &&
			eqint(t, "datarefindex", int64(expected.(*ast.DataRefIndexNode).Index), int64(actual.(*ast.DataRefIndexNode).Index))

	case *ast.NotNode:
		return eqTree(t, expected.(*ast.NotNode).Arg, actual.(*ast.NotNode).Arg)
	case *ast.NegateNode:
		return eqTree(t, expected.(*ast.NegateNode).Arg, actual.(*ast.NegateNode).Arg)
	case *ast.MulNode, *ast.DivNode, *ast.ModNode, *ast.AddNode, *ast.SubNode, *ast.EqNode, *ast.NotEqNode,
		*ast.GtNode, *ast.GteNode, *ast.LtNode, *ast.LteNode, *ast.OrNode, *ast.AndNode, *ast.ElvisNode:
		return eqBinOp(t, expected, actual)
	case *ast.TernNode:
		return eqTree(t, expected.(*ast.TernNode).Arg1, actual.(*ast.TernNode).Arg1) &&
			eqTree(t, expected.(*ast.TernNode).Arg2, actual.(*ast.TernNode).Arg2) &&
			eqTree(t, expected.(*ast.TernNode).Arg3, actual.(*ast.TernNode).Arg3)
	case *ast.FunctionNode:
		return eqstr(t, "function", expected.(*ast.FunctionNode).Name, actual.(*ast.FunctionNode).Name) &&
			eqNodes(t, expected.(*ast.FunctionNode).Args, actual.(*ast.FunctionNode).Args)

	case *ast.SoyDocNode:
		return eqNodes(t, expected.(*ast.SoyDocNode).Params, actual.(*ast.SoyDocNode).Params)
	case *ast.SoyDocParamNode:
		return eqstr(t, "soydocparam", expected.(*ast.SoyDocParamNode).Name, actual.(*ast.SoyDocParamNode).Name) &&
			eqbool(t, "soydocparam", expected.(*ast.SoyDocParamNode).Optional, actual.(*ast.SoyDocParamNode).Optional)
	case *ast.PrintNode:
		return eqTree(t, expected.(*ast.PrintNode).Arg, actual.(*ast.PrintNode).Arg)
	case *ast.MsgNode:
		return eqstr(t, "msg", expected.(*ast.MsgNode).Desc, actual.(*ast.MsgNode).Desc) &&
			eqstr(t, "msg", expected.(*ast.MsgNode).Meaning, actual.(*ast.MsgNode).Meaning) &&
			eqNodes(t, expected.(*ast.MsgNode).Body, actual.(*ast.MsgNode).Body)
	case *ast.MsgPlaceholderNode:
		return eqTree(t, expected.(*ast.MsgPlaceholderNode).Body, actual.(*ast.MsgPlaceholderNode).Body)
	case *ast.CallNode:
		return eqstr(t, "call", expected.(*ast.CallNode).Name, actual.(*ast.CallNode).Name) &&
			eqTree(t, expected.(*ast.CallNode).Data, actual.(*ast.CallNode).Data) &&
			eqNodes(t, expected.(*ast.CallNode).Params, actual.(*ast.CallNode).Params)
	case *ast.CallParamValueNode:
		return eqstr(t, "param", expected.(*ast.CallParamValueNode).Key, actual.(*ast.CallParamValueNode).Key) &&
			eqTree(t, expected.(*ast.CallParamValueNode).Value, actual.(*ast.CallParamValueNode).Value)
	case *ast.CallParamContentNode:
		return eqstr(t, "param", expected.(*ast.CallParamContentNode).Key, actual.(*ast.CallParamContentNode).Key) &&
			eqTree(t, expected.(*ast.CallParamContentNode).Content, actual.(*ast.CallParamContentNode).Content)

	case *ast.IfNode:
		return eqNodes(t, expected.(*ast.IfNode).Conds, actual.(*ast.IfNode).Conds)
	case *ast.IfCondNode:
		return eqTree(t, expected.(*ast.IfCondNode).Cond, actual.(*ast.IfCondNode).Cond) &&
			eqTree(t, expected.(*ast.IfCondNode).Body, actual.(*ast.IfCondNode).Body)
	case *ast.ForNode:
		return eqstr(t, "for", expected.(*ast.ForNode).Var, actual.(*ast.ForNode).Var) &&
			eqTree(t, expected.(*ast.ForNode).List, actual.(*ast.ForNode).List) &&
			eqTree(t, expected.(*ast.ForNode).Body, actual.(*ast.ForNode).Body) &&
			eqTree(t, expected.(*ast.ForNode).IfEmpty, actual.(*ast.ForNode).IfEmpty)
	case *ast.SwitchNode:
		return eqTree(t, expected.(*ast.SwitchNode).Value, actual.(*ast.SwitchNode).Value) &&
			eqNodes(t, expected.(*ast.SwitchNode).Cases, actual.(*ast.SwitchNode).Cases)
	case *ast.SwitchCaseNode:
		return eqTree(t, expected.(*ast.SwitchCaseNode).Body, actual.(*ast.SwitchCaseNode).Body) &&
			eqNodes(t, expected.(*ast.SwitchCaseNode).Values, actual.(*ast.SwitchCaseNode).Values)
	}
	panic(fmt.Sprintf("type not implemented: %T", actual))
}

func eqstr(t *testing.T, name, exp, act string) bool {
	if exp != act {
		t.Errorf("%s: expected %q got %q", name, exp, act)
	}
	return exp == act
}

func eqint(t *testing.T, name string, exp, act int64) bool {
	if exp != act {
		t.Errorf("%v: expected %v got %v", name, exp, act)
	}
	return exp == act
}

func eqbool(t *testing.T, name string, exp, act bool) bool {
	if exp != act {
		t.Errorf("%v: expected %v got %v", name, exp, act)
	}
	return exp == act
}

func eqfloat(t *testing.T, name string, exp, act float64) bool {
	if exp != act {
		t.Errorf("%v: expected %v got %v", name, exp, act)
	}
	return exp == act
}

// eqBinOp compares structs that embed binaryOpNode
func eqBinOp(t *testing.T, n1, n2 interface{}) bool {
	var (
		op1 = reflect.ValueOf(n1).Elem().Field(0).Interface().(ast.BinaryOpNode)
		op2 = reflect.ValueOf(n2).Elem().Field(0).Interface().(ast.BinaryOpNode)
	)
	return eqTree(t, op1.Arg1, op2.Arg1) && eqTree(t, op1.Arg2, op2.Arg2)
}

func eqNodes(t *testing.T, expected, actual interface{}) bool {
	a, e := reflect.ValueOf(actual), reflect.ValueOf(expected)
	if a.Kind() != reflect.Slice || e.Kind() != reflect.Slice {
		panic("whoops")
	}
	if a.Len() != e.Len() {
		t.Errorf("lengths not equal: expected %v got %v", e.Len(), a.Len())
		return false
	}
	for i := 0; i < a.Len(); i++ {
		if !eqTree(t, e.Index(i).Interface().(ast.Node), a.Index(i).Interface().(ast.Node)) {
			return false
		}
	}
	return true
}

var nodeType = reflect.TypeOf((*ast.Node)(nil)).Elem()

func printTree(t *testing.T, n ast.Node, depth int) {
	if reflect.TypeOf(n) != reflect.TypeOf((*ast.BinaryOpNode)(nil)) {
		t.Logf("%s--> %T", strings.Repeat("\t", depth), n)
	}
	val := reflect.ValueOf(n).Elem()
	for i := 0; i < val.NumField(); i++ {
		f := val.Field(i)
		ft := f.Type()
		if ft.Kind() == reflect.Interface && f.IsNil() {
			t.Logf("%s--> nil", strings.Repeat("\t", depth+1))
			continue
		}
		if ft.Kind() == reflect.Slice && ft.Elem().Implements(nodeType) {
			for i := 0; i < f.Len(); i++ {
				printTree(t, f.Index(i).Interface().(ast.Node), depth+1)
			}
		} else if f.Type().Implements(nodeType) {
			printTree(t, f.Interface().(ast.Node), depth+1)
		} else if f.Addr().Type().Implements(nodeType) {
			printTree(t, f.Addr().Interface().(ast.Node), depth)
		} else {
			//t.Logf("does not implement: %T", f.Interface())
		}
	}
}

// Parser tests imported from the official Soy project

func TestRecognizeSoyTag(t *testing.T) {
	works(t, "{sp}")
	works(t, "{ sp }")
	works(t, "{{sp}}")

	// Soy V1 syntax. will not fix.
	// works(t, "{space}")
	// works(t, "{{space}}")
	// works(t, "{{ {sp} }}")

	fails(t, "{}")
	fails(t, "{sp")
	fails(t, "{sp blah}")
	fails(t, "{print { }")
	fails(t, "{print } }")
	fails(t, "{print }}")
	fails(t, "{{}}")
	fails(t, "{{{blah: blah}}}")
	fails(t, "blah}blah")
	fails(t, "blah}}blah")
	fails(t, "{{print {{ }}")
	fails(t, "{{print {}}")
}

func TestRecognizeRawText(t *testing.T) {
	works(t, "blah>blah<blah<blah>blah>blah>blah>blah<blah")
	works(t, "{sp}{nil}{\\n}{{\\r}}{\\t}{lb}{{rb}}")
	works(t, "blah{literal}{ {{{ } }{ {}} { }}}}}}}\n"+
		"}}}}}}}}}{ { {{/literal}blah")

	fails(t, "{sp ace}")
	fails(t, "{/literal}")
	fails(t, "{literal attrib=\"value\"}")
}

func TestRecognizeCommands(t *testing.T) {
	works(t, ""+
		"{msg desc=\"blah\" hidden=\"true\"}\n"+
		"  {$boo} is a <a href=\"{$fooUrl}\">{$foo}</a>.\n"+
		"{/msg}")
	works(t, "{$aaa + 1}{print $bbb.ccc[$ddd] |noescape}")
	works(t, "{css selected-option}{css CSS_SELECTED_OPTION}{css $cssSelectedOption}")
	works(t, "{if $boo}foo{elseif $goo}moo{else}zoo{/if}")
	works(t, ""+
		"  {switch $boo}\n"+
		"    {case $foo} blah blah\n"+
		"    {case 2, $goo.moo, 'too'} bleh bleh\n"+
		"    {default} bluh bluh\n"+
		"  {/switch}\n")
	works(t, "{foreach $item in $items}{index($item)}. {$item.name}<br>{/foreach}")
	works(t, ""+
		"{for $i in range($boo + 1,\n"+
		"                 88, 11)}\n"+
		"Number {$i}.{{/for}}")
	works(t, "{call name=\"aaa.bbb.ccc\" data=\"all\" /}")
	works(t, ""+
		"{call name=\".aaa\"}\n"+
		"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
		"  {param key=\"foo\"}blah blah{/param}\n"+
		"  {param key=\"foo\" kind=\"html\"}blah blah{/param}\n"+
		"  {param foo kind=\"html\"}blah blah{/param}\n"+
		"{/call}")
	// Soy V1 syntax.  will not fix.
	// works(t,
	// 	"{call .aaa}\n"+
	// 		"  {param foo : bar \" baz/}\n"+
	// 		"{/call}\n")
	works(t, "{call aaa.bbb.ccc data=\"all\" /}")
	works(t, ""+
		"{call .aaa}\n"+
		"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
		"  {param key=\"foo\"}blah blah{/param}\n"+
		"{/call}")

	// TODO: implement delcall
	// works(t, "{delcall aaa.bbb.ccc data=\"all\" /}")
	// works(t, ""+
	// 	"{delcall name=\"ddd.eee\"}\n"+
	// 	"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
	// 	"  {param key=\"foo\"}blah blah{/param}\n"+
	// 	"{/delcall}")

	// TODO: implement phname
	// works(t, ""+
	// 	"{msg meaning=\"boo\" desc=\"blah\"}\n"+
	// 	"  {$boo phname=\"foo\"} is a \n"+
	// 	"  <a phname=\"begin_link\" href=\"{$fooUrl}\">\n"+
	// 	"    {$foo |noAutoescape phname=\"booFoo\" }\n"+
	// 	"  </a phname=\"END_LINK\" >.\n"+
	// 	"  {call .aaa data=\"all\"\nphname=\"AaaBbb\"/}\n"+
	// 	"  {call .aaa phname=\"AaaBbb\" data=\"all\"}{/call}\n"+
	// 	"{/msg}")

	works(t, "{log}Blah blah.{/log}")
	works(t, "{debugger}")
	works(t, "{let $foo : 1 + 2/}\n")
	works(t, "{let $foo : '\"'/}\n")
	works(t, "{let $foo}Hello{/let}\n")

	// TODO: implement kind
	// works(t, "{let $foo kind=\"html\"}Hello{/let}\n")

	fails(t, "{msg}blah{/msg}")
	fails(t, "{/msg}")

	// TODO: implement well-formed HTML checks.
	// fails(t, "{msg desc=\"\"}<a href=http://www.google.com{/msg}")

	// Within a message tag, some are not allowed: {msg}, {for}, {foreach}, {switch}, {if}
	fails(t, "{msg desc=\"\"}blah{msg desc=\"\"}bleh{/msg}bluh{/msg}")
	fails(t, "{msg desc=\"\"}{foreach $item in $items}{/foreach}{/msg}")
	fails(t, "{msg desc=\"\"}{for $i in range(5)}{/for}{/msg}")
	fails(t, "{msg desc=\"\"}{switch}{/switch}{/msg}")
	fails(t, "{msg desc=\"\"}{if $i}{/if}{/msg}")

	fails(t, "{msg desc=\"\"}blah{/msg blah}")
	fails(t, "{namespace}")
	fails(t, "{template}\n"+"blah\n"+"{/template}\n")
	fails(t, "{msg}<blah<blah>{/msg}")
	fails(t, "{msg}blah>blah{/msg}")
	fails(t, "{msg}<blah>blah>{/msg}")
	fails(t, "{print $boo /}")
	fails(t, "{if true}aaa{else/}bbb{/if}")
	fails(t, "{call .aaa.bbb /}")
	fails(t, "{delcall name=\"ddd.eee\"}{param foo: 0}{/call}")
	fails(t, "{delcall .dddEee /}")

	// TODO: implement phname
	// fails(t, "{msg desc=\"\"}{$boo phname=\"boo.foo\"}{/msg}")
	// fails(t, "{msg desc=\"\"}<br phname=\"boo-foo\" />{/msg}")
	// fails(t, "{msg desc=\"\"}{call .boo phname=\"boo\" phname=\"boo\" /}{/msg}")
	// fails(t, "{msg desc=\"\"}<br phname=\"break\" phname=\"break\" />{/msg}")

	fails(t, "{call name=\".aaa\"}{param boo kind=\"html\": 123 /}{/call}\n")
	fails(t, "{log}")
	fails(t, "{log 'Blah blah.'}")
	fails(t, "{let $foo kind=\"html\" : 1 + 1/}\n")
}

func TestRecognizeComments(t *testing.T) {
	works(t, "blah // }\n"+
		"{$boo}{msg desc=\"\"} //}\n"+
		"{/msg} // {/msg}\n"+
		"{foreach $item in $items}\t// }\n"+
		"{$item.name}{/foreach} //{{{{\n")
	works(t, "blah /* } */\n"+
		"{msg desc=\"\"} /*}*/{$boo}\n"+
		"/******************/ {/msg}\n"+
		"/* {}} { }* }* / }/ * { **}  //}{ { } {\n"+
		"\n  } {//*} {* /} { /* /}{} {}/ } **}}} */\n"+
		"{foreach $item in $items} /* }\n"+
		"{{{{{*/{$item.name}{/foreach}/*{{{{*/\n")
	works(t, " //}\n")
	works(t, "\n//}\n")
	works(t, "\n //}\n")

	fails(t, "{blah /* { */ blah}")
	fails(t, "{foreach $item // }\n"+
		"         in $items}\n"+
		"{$item}{/foreach}\n")
	fails(t, "aa////}\n")
	fails(t, "{nil}//}\n")
}

func works(t *testing.T, body string) {
	_, err := SoyFile("", body, nil)
	if err != nil {
		t.Error(err)
	}
}

func fails(t *testing.T, body string) {
	_, err := SoyFile("", body, nil)
	if err == nil {
		t.Errorf("should fail: %s", body)
	}
}
