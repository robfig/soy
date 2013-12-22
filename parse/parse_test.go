package parse

import (
	"fmt"
	"reflect"

	"strings"

	"testing"
)

type parseTest struct {
	name  string
	input string
	tree  Node
}

const (
	noError  = true
	hasError = false
)

func tList(nodes ...Node) Node {
	n := newList(0)
	n.Nodes = nodes
	return n
}

func tTemplate(name string, nodes ...Node) Node {
	n := newTemplate(0, name)
	n.Body = newList(0)
	n.Body.Nodes = nodes
	return n
}

func bin(n1, n2 Node) binaryOpNode {
	return binaryOpNode{Arg1: n1, Arg2: n2}
}

var parseTests = []parseTest{
	{"empty", "", tList()},
	{"namespace", "{namespace example}", tList(newNamespace(0, "example"))},
	{"empty template", "{template .name}{/template}", tList(tTemplate(".name"))},
	{"text template", "{template .name}\nHello world!\n{/template}",
		tList(tTemplate(".name", newText(0, "Hello world!")))},
	{"variable template", "{template .name}\nHello {$name}!\n{/template}",
		tList(tTemplate(".name",
			newText(0, "Hello "),
			newPrint(0, &DataRefNode{0, "name", nil}), // implicit print
			newText(0, "!"),
		))},
	{"soydoc", "/** Text\n*/", tList(newSoyDoc(0, "/** Text\n*/"))},
	{"negate", "{not $var}", tList(&PrintNode{0, &NotNode{0, &DataRefNode{0, "var", nil}}})},
	{"concat", `{'hello' + 'world'}`, tList(&PrintNode{0,
		&AddNode{bin(
			&StringNode{0, "hello"},
			&StringNode{0, "world"})},
	})},

	{"rawtext (linejoin)", "\n  a \n\tb\r\n  c  \n\n", tList(newText(0, "a b c"))},
	{"rawtext+html", "\n  a <br>\n\tb\r\n\n  c\n\n<br> ", tList(newText(0, "a <br>b c<br> "))},
	{"rawtext+comment", "a <br> // comment \n\tb\t// comment2\r\n  c\n\n", tList(newText(0, "a <br>b c"))},
	{"rawtext+tag", "a {$foo}\n\t  b\r\n\n  {$bar} c", tList(
		newText(0, "a "),
		&PrintNode{0, &DataRefNode{0, "foo", nil}},
		newText(0, "b"),
		&PrintNode{0, &DataRefNode{0, "bar", nil}},
		newText(0, " c"),
	)},

	{"expression1", "{not false and (isFirst($foo) or (-$x - 5) > 3.1)}", tList(&PrintNode{0,
		&AndNode{bin(
			&NotNode{0, &BoolNode{0, false}},
			&OrNode{bin(
				&FunctionNode{0, "isFirst", []Node{&DataRefNode{0, "foo", nil}}},
				&GtNode{bin(
					&SubNode{bin(
						&NegateNode{0, &DataRefNode{0, "x", nil}},
						&IntNode{0, 5})},
					&FloatNode{0, 3.1})})})},
	})},

	{"expression2", `{null or ('foo' == 'f'+true ? -3 <= 5 : not $foo ?: bar(5))}`, tList(&PrintNode{0,
		&OrNode{bin(
			&NullNode{0},
			&TernNode{0,
				&EqNode{bin(
					&StringNode{0, "foo"},
					&AddNode{bin(
						&StringNode{0, "f"},
						&BoolNode{0, true})})},
				&LteNode{bin(
					&IntNode{0, -3},
					&IntNode{0, 5})},
				&ElvisNode{bin(
					&NotNode{0, &DataRefNode{0, "foo", nil}},
					&FunctionNode{0, "bar", []Node{&IntNode{0, 5}}})}})},
	})},

	{"expression3", `{'a'+'b' != 'ab' and (2 >= -5.0 or (null ?: true))}`, tList(&PrintNode{0,
		&AndNode{bin(
			&NotEqNode{bin(
				&AddNode{bin(
					&StringNode{0, "a"},
					&StringNode{0, "b"})},
				&StringNode{0, "ab"})},
			&OrNode{bin(
				&GteNode{bin(&IntNode{0, 2}, &FloatNode{0, -5.0})},
				&ElvisNode{bin(
					&NullNode{0},
					&BoolNode{0, true})})})},
	})},

	{"empty list", `{[]}`, tList(&PrintNode{0,
		&ListLiteralNode{0, nil},
	})},

	{"list", `{[1, 'two', [3, false]]}`, tList(&PrintNode{0,
		&ListLiteralNode{0, []Node{
			&IntNode{0, 1},
			&StringNode{0, "two"},
			&ListLiteralNode{0, []Node{
				&IntNode{0, 3},
				&BoolNode{0, false},
			}},
		}},
	})},

	{"empty map", `{[:]}`, tList(&PrintNode{0,
		&MapLiteralNode{0, make(map[string]Node)},
	})},

	{"map", `{['aaa': 42, 'bbb': 'hello', 'ccc':[1]]}`, tList(&PrintNode{0,
		&MapLiteralNode{0, map[string]Node{
			"aaa": &IntNode{0, 42},
			"bbb": &StringNode{0, "hello"},
			"ccc": &ListLiteralNode{0, []Node{&IntNode{0, 1}}},
		}},
	})},

	{"if", `
{if $zoo}{$zoo}{/if}
{if $boo}
  Blah
{elseif $foo.goo > 2}
  {$boo}
{else}
  Blah {$moo}
{/if}`, tList(
		&IfNode{0, []*IfCondNode{
			&IfCondNode{0, &DataRefNode{0, "zoo", nil}, tList(&PrintNode{0, &DataRefNode{0, "zoo", nil}})},
		}},
		&IfNode{0, []*IfCondNode{
			&IfCondNode{0, &DataRefNode{0, "boo", nil}, tList(newText(0, "Blah"))},
			&IfCondNode{0,
				&GtNode{bin(
					&DataRefNode{0, "foo", []Node{&DataRefKeyNode{0, false, "goo"}}},
					&IntNode{0, 2})},
				tList(&PrintNode{0, &DataRefNode{0, "boo", nil}})},
			&IfCondNode{0,
				nil,
				tList(newText(0, "Blah "), &PrintNode{0, &DataRefNode{0, "moo", nil}})},
		}},
	)},

	{"switch", `
{switch $boo} {case 0}Blah
  {case $foo.goo}
    Bleh
  {case -1, 1, $moo}
    Bluh
  {default}
    Bloh
{/switch}`, tList(
		&SwitchNode{0, &DataRefNode{0, "boo", nil}, []*SwitchCaseNode{
			&SwitchCaseNode{0, []Node{&IntNode{0, 0}}, tList(newText(0, "Blah"))},
			&SwitchCaseNode{0, []Node{&DataRefNode{0, "foo", []Node{&DataRefKeyNode{0, false, "goo"}}}},
				tList(newText(0, "Bleh"))},
			&SwitchCaseNode{0, []Node{
				&IntNode{0, -1},
				&IntNode{0, 1},
				&DataRefNode{0, "moo", nil}}, tList(newText(0, "Bluh"))},
			&SwitchCaseNode{0, nil, tList(newText(0, "Bloh"))},
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
{/foreach}`, tList(
		&ForNode{0, "goo", &DataRefNode{0, "goose", nil}, tList(
			&PrintNode{0, &DataRefNode{0, "goose", []Node{&DataRefKeyNode{0, false, "numKids"}}}},
			newText(0, " goslings."),
			newText(0, "\n"),
		), nil},
		&ForNode{0, "boo", &DataRefNode{0, "foo", []Node{&DataRefKeyNode{0, false, "booze"}}},
			tList(
				newText(0, "Scary drink "),
				&PrintNode{0, &DataRefNode{0, "boo", []Node{&DataRefKeyNode{0, false, "name"}}}},
				newText(0, "!"),
				&IfNode{0,
					[]*IfCondNode{&IfCondNode{0,
						&NotNode{0, &FunctionNode{0, "isLast", []Node{&DataRefNode{0, "boo", nil}}}},
						tList(newText(0, "\n"))}}}),
			tList(
				newText(0, "Sorry, no booze."))},
	)},

	{"for", `
{for $i in range(1, $items.length + 1)}
  {msg desc="Numbered item."}
    {$i}: {$items[$i - 1]}{\n}
  {/msg}
{/for}`, tList(
		&ForNode{0, "i",
			&FunctionNode{0, "range", []Node{
				&IntNode{0, 1},
				&AddNode{bin(
					&DataRefNode{0, "items", []Node{&DataRefKeyNode{0, false, "length"}}},
					&IntNode{0, 1})}}},
			tList(
				&MsgNode{0, "Numbered item.", tList(
					&PrintNode{0, &DataRefNode{0, "i", nil}},
					newText(0, ": "),
					&PrintNode{0, &DataRefNode{0, "items", []Node{
						&DataRefExprNode{0, false,
							&SubNode{bin(
								&DataRefNode{0, "i", nil},
								&IntNode{0, 1})}}}}},
					newText(0, "\n"), // {\n}
				)}),
			nil},
	)},

	{"data ref", "{$boo.0['foo'+'bar'][5]?.goo}", tList(&PrintNode{0,
		&DataRefNode{0, "boo", []Node{
			&DataRefIndexNode{0, false, 0},
			&DataRefExprNode{0,
				false,
				&AddNode{bin(
					&StringNode{0, "foo"},
					&StringNode{0, "bar"})}},
			&DataRefExprNode{0, false, &IntNode{0, 5}},
			&DataRefKeyNode{0, true, "goo"}},
		}})},

	{"call", `
{call name=".booTemplate_" /}
{call function="foo.goo.mooTemplate" data="all" /}
{call name=".zooTemplate" data="$animals"}
  {param key="yoo" value="round($too)" /}
  {param key="woo"}poo{/param}
  {param key="doo" kind="html"}doopoo{/param}
{/call}
{call .booTemplate_ /}
{call .zooTemplate data="$animals"}
  {param yoo: round($too) /}
  {param woo}poo{/param}
  {param zoo: 0 /}
  {param doo kind="html"}doopoo{/param}
{/call}`, tList(
		&CallNode{0, ".booTemplate_", false, nil, nil},
		&CallNode{0, "foo.goo.mooTemplate", true, nil, nil},
		&CallNode{0, ".zooTemplate", false, &DataRefNode{0, "animals", nil}, []*CallParamNode{
			{0, "yoo", &FunctionNode{0, "round", []Node{&DataRefNode{0, "too", nil}}}},
			{0, "woo", tList(newText(0, "poo"))},
			{0, "doo", tList(newText(0, "doopoo"))}}},
		&CallNode{0, ".booTemplate_", false, nil, nil},
		&CallNode{0, ".zooTemplate", false, &DataRefNode{0, "animals", nil}, []*CallParamNode{
			{0, "yoo", &FunctionNode{0, "round", []Node{&DataRefNode{0, "too", nil}}}},
			{0, "woo", tList(newText(0, "poo"))},
			{0, "zoo", &IntNode{0, 0}},
			{0, "doo", tList(newText(0, "doopoo"))}}},
	)},

	// "  {let $alpha: $boo.foo /}\n" +
	// "  {let $beta}Boo!{/let}\n" +
	// "  {let $gamma}\n" +
	// "    {for $i in range($alpha)}\n" +
	// "      {$i}{$beta}\n" +
	// "    {/for}\n" +
	// "  {/let}\n" +
	// "  {let $delta kind=\"html\"}Boo!{/let}\n";

	// {"spaces", " \t\n", noError, `" \t\n"`},
	// {"text", "some text", noError, `"some text"`},
	// {"emptyAction", "{{}}", hasError, `{{}}`},
	// {"simple command", "{template .templateName}", noError, `{{printf}}`},
	// {"$ invocation", "{{$varname}}", noError, "{{$varname}}"},
}

var builtins = map[string]interface{}{
	"printf": fmt.Sprintf,
}

func TestParse(t *testing.T) {
	textFormat = "%q"
	defer func() { textFormat = "%s" }()
	for _, test := range parseTests {
		tmpl, err := New(test.name).Parse(test.input, builtins)

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
		if !eqTree(t, test.tree, tmpl.Root) {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, tmpl.Root, test.tree)
			t.Log("Expected:")
			printTree(t, test.tree, 0)
			t.Log("Actual:")
			if tmpl == nil {
				t.Log("<nil>")
			} else {
				printTree(t, tmpl.Root, 0)
			}
		}
	}
}

func eqTree(t *testing.T, expected, actual Node) bool {
	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("expected %T, got %T", expected, actual)
		return false
	}

	if actual == nil && expected == nil {
		return true
	}

	switch actual.(type) {
	case *ListNode:
		return eqNodes(t, expected.(*ListNode).Nodes, actual.(*ListNode).Nodes)
	case *NamespaceNode:
		return eqstr(t, "namespace", expected.(*NamespaceNode).Name, actual.(*NamespaceNode).Name)
	case *TemplateNode:
		if expected.(*TemplateNode).Name != actual.(*TemplateNode).Name {
			return false
		}
		return eqTree(t, expected.(*TemplateNode).Body, actual.(*TemplateNode).Body)
	case *RawTextNode:
		return eqstr(t, "text", string(expected.(*RawTextNode).Text), string(actual.(*RawTextNode).Text))

	case *NullNode:
		return true
	case *BoolNode:
		return eqbool(t, "bool", expected.(*BoolNode).True, actual.(*BoolNode).True)
	case *IntNode:
		return eqint(t, "int", expected.(*IntNode).Value, actual.(*IntNode).Value)
	case *FloatNode:
		return eqfloat(t, "float", expected.(*FloatNode).Value, actual.(*FloatNode).Value)
	case *StringNode:
		return eqstr(t, "stringnode", expected.(*StringNode).Value, actual.(*StringNode).Value)
	case *ListLiteralNode:
		return eqNodes(t, expected.(*ListLiteralNode).Items, actual.(*ListLiteralNode).Items)
	case *MapLiteralNode:
		e, a := expected.(*MapLiteralNode).Items, actual.(*MapLiteralNode).Items
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

	case *DataRefNode:
		return eqstr(t, "var", expected.(*DataRefNode).Key, actual.(*DataRefNode).Key) &&
			eqNodes(t, expected.(*DataRefNode).Access, actual.(*DataRefNode).Access)
	case *DataRefExprNode:
		return eqbool(t, "datarefexpr", expected.(*DataRefExprNode).NullSafe, actual.(*DataRefExprNode).NullSafe) &&
			eqTree(t, expected.(*DataRefExprNode).Arg, actual.(*DataRefExprNode).Arg)
	case *DataRefKeyNode:
		return eqbool(t, "datarefkey", expected.(*DataRefKeyNode).NullSafe, actual.(*DataRefKeyNode).NullSafe) &&
			eqstr(t, "datarefkey", expected.(*DataRefKeyNode).Key, actual.(*DataRefKeyNode).Key)
	case *DataRefIndexNode:
		return eqbool(t, "datarefindex", expected.(*DataRefIndexNode).NullSafe, actual.(*DataRefIndexNode).NullSafe) &&
			eqint(t, "datarefindex", int64(expected.(*DataRefIndexNode).Index), int64(actual.(*DataRefIndexNode).Index))

	case *NotNode:
		return eqTree(t, expected.(*NotNode).Arg, actual.(*NotNode).Arg)
	case *NegateNode:
		return eqTree(t, expected.(*NegateNode).Arg, actual.(*NegateNode).Arg)
	case *MulNode, *DivNode, *ModNode, *AddNode, *SubNode, *EqNode, *NotEqNode,
		*GtNode, *GteNode, *LtNode, *LteNode, *OrNode, *AndNode, *ElvisNode:
		return eqBinOp(t, expected, actual)
	case *TernNode:
		return eqTree(t, expected.(*TernNode).Arg1, actual.(*TernNode).Arg1) &&
			eqTree(t, expected.(*TernNode).Arg2, actual.(*TernNode).Arg2) &&
			eqTree(t, expected.(*TernNode).Arg3, actual.(*TernNode).Arg3)
	case *FunctionNode:
		return eqstr(t, "function", expected.(*FunctionNode).Name, actual.(*FunctionNode).Name) &&
			eqNodes(t, expected.(*FunctionNode).Args, actual.(*FunctionNode).Args)

	case *SoyDocNode:
		return expected.(*SoyDocNode).Comment == actual.(*SoyDocNode).Comment
	case *PrintNode:
		return eqTree(t, expected.(*PrintNode).Arg, actual.(*PrintNode).Arg)
	case *MsgNode:
		return eqstr(t, "msg", expected.(*MsgNode).Desc, actual.(*MsgNode).Desc) &&
			eqTree(t, expected.(*MsgNode).Body, actual.(*MsgNode).Body)
	case *CallNode:
		return eqstr(t, "call", expected.(*CallNode).Name, actual.(*CallNode).Name) &&
			eqTree(t, expected.(*CallNode).Data, actual.(*CallNode).Data) &&
			eqNodes(t, expected.(*CallNode).Params, actual.(*CallNode).Params)
	case *CallParamNode:
		return eqstr(t, "param", expected.(*CallParamNode).Key, actual.(*CallParamNode).Key) &&
			eqTree(t, expected.(*CallParamNode).Value, actual.(*CallParamNode).Value)

	case *IfNode:
		return eqNodes(t, expected.(*IfNode).Conds, actual.(*IfNode).Conds)
	case *IfCondNode:
		return eqTree(t, expected.(*IfCondNode).Cond, actual.(*IfCondNode).Cond) &&
			eqTree(t, expected.(*IfCondNode).Body, actual.(*IfCondNode).Body)
	case *ForNode:
		return eqstr(t, "for", expected.(*ForNode).Var, actual.(*ForNode).Var) &&
			eqTree(t, expected.(*ForNode).List, actual.(*ForNode).List) &&
			eqTree(t, expected.(*ForNode).Body, actual.(*ForNode).Body) &&
			eqTree(t, expected.(*ForNode).IfEmpty, actual.(*ForNode).IfEmpty)
	case *SwitchNode:
		return eqTree(t, expected.(*SwitchNode).Value, actual.(*SwitchNode).Value) &&
			eqNodes(t, expected.(*SwitchNode).Cases, actual.(*SwitchNode).Cases)
	case *SwitchCaseNode:
		return eqTree(t, expected.(*SwitchCaseNode).Body, actual.(*SwitchCaseNode).Body) &&
			eqNodes(t, expected.(*SwitchCaseNode).Values, actual.(*SwitchCaseNode).Values)
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
		op1 = reflect.ValueOf(n1).Elem().Field(0).Interface().(binaryOpNode)
		op2 = reflect.ValueOf(n2).Elem().Field(0).Interface().(binaryOpNode)
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
		if !eqTree(t, e.Index(i).Interface().(Node), a.Index(i).Interface().(Node)) {
			return false
		}
	}
	return true
}

var nodeType = reflect.TypeOf((*Node)(nil)).Elem()

func printTree(t *testing.T, n Node, depth int) {
	if reflect.TypeOf(n) != reflect.TypeOf((*binaryOpNode)(nil)) {
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
				printTree(t, f.Index(i).Interface().(Node), depth+1)
			}
		} else if f.Type().Implements(nodeType) {
			printTree(t, f.Interface().(Node), depth+1)
		} else if f.Addr().Type().Implements(nodeType) {
			printTree(t, f.Addr().Interface().(Node), depth)
		} else {
			//t.Logf("does not implement: %T", f.Interface())
		}
	}
}

// func TestRecognizeSoyTag(t *testing.T) {
// 	works(t, "{sp}")
// 	works(t, "{space}")
// 	works(t, "{ sp }")
// 	works(t, "{{sp}}")
// 	works(t, "{{space}}")
// 	works(t, "{{ {sp} }}")

// 	fails(t, "{}")
// 	fails(t, "{sp")
// 	fails(t, "{sp blah}")
// 	fails(t, "{print { }")
// 	fails(t, "{print } }")
// 	fails(t, "{print }}")
// 	fails(t, "{{}}")
// 	fails(t, "{{{blah: blah}}}")
// 	fails(t, "blah}blah")
// 	fails(t, "blah}}blah")
// 	fails(t, "{{print {{ }}")
// 	fails(t, "{{print {}}")
// }

// func TestRecognizeRawText(t *testing.T) {
// 	works(t, "blah>blah<blah<blah>blah>blah>blah>blah<blah")
// 	works(t, "{sp}{nil}{\\n}{{\\r}}{\\t}{lb}{{rb}}")
// 	works(t, "blah{literal}{ {{{ } }{ {}} { }}}}}}}\n"+
// 		"}}}}}}}}}{ { {{/literal}blah")

// 	fails(t, "{sp ace}")
// 	fails(t, "{/literal}")
// 	fails(t, "{literal attrib=\"value\"}")
// 	fails(t, "{literal}{literal}{/literal}")
// }

// func TestRecognizeCommands(t *testing.T) {
// 	works(t, ""+
// 		"{msg desc=\"blah\" hidden=\"true\"}\n"+
// 		"  {$boo} is a <a href=\"{$fooUrl}\">{$foo}</a>.\n"+
// 		"{/msg}")
// 	works(t, "{$aaa + 1}{print $bbb.ccc[$ddd] |noescape}")
// 	works(t, "{css selected-option}{css CSS_SELECTED_OPTION}{css $cssSelectedOption}")
// 	works(t, "{if $boo}foo{elseif $goo}moo{else}zoo{/if}")
// 	works(t, ""+
// 		"  {switch $boo}\n"+
// 		"    {case $foo} blah blah\n"+
// 		"    {case 2, $goo.moo, 'too'} bleh bleh\n"+
// 		"    {default} bluh bluh\n"+
// 		"  {/switch}\n")
// 	works(t, "{foreach $item in $items}{index($item)}. {$item.name}<br>{/foreach}")
// 	works(t, ""+
// 		"{for $i in range($boo + 1,\n"+
// 		"                 88, 11)}\n"+
// 		"Number {$i}.{{/for}}")
// 	works(t, "{call function=\"aaa.bbb.ccc\" data=\"all\" /}")
// 	works(t, ""+
// 		"{call name=\".aaa\"}\n"+
// 		"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
// 		"  {param key=\"foo\"}blah blah{/param}\n"+
// 		"  {param key=\"foo\" kind=\"html\"}blah blah{/param}\n"+
// 		"  {param foo kind=\"html\"}blah blah{/param}\n"+
// 		"{/call}")
// 	works(t,
// 		"{call .aaa}\n"+
// 			"  {param foo : bar \" baz/}\n"+
// 			"{/call}\n")
// 	works(t, "{call aaa.bbb.ccc data=\"all\" /}")
// 	works(t, ""+
// 		"{call .aaa}\n"+
// 		"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
// 		"  {param key=\"foo\"}blah blah{/param}\n"+
// 		"{/call}")
// 	works(t, "{delcall aaa.bbb.ccc data=\"all\" /}")
// 	works(t, ""+
// 		"{delcall name=\"ddd.eee\"}\n"+
// 		"  {{param key=\"boo\" value=\"$boo\" /}}\n"+
// 		"  {param key=\"foo\"}blah blah{/param}\n"+
// 		"{/delcall}")
// 	works(t, ""+
// 		"{msg meaning=\"boo\" desc=\"blah\"}\n"+
// 		"  {$boo phname=\"foo\"} is a \n"+
// 		"  <a phname=\"begin_link\" href=\"{$fooUrl}\">\n"+
// 		"    {$foo |noAutoescape phname=\"booFoo\" }\n"+
// 		"  </a phname=\"END_LINK\" >.\n"+
// 		"  {call .aaa data=\"all\"\nphname=\"AaaBbb\"/}\n"+
// 		"  {call .aaa phname=\"AaaBbb\" data=\"all\"}{/call}\n"+
// 		"{/msg}")
// 	works(t, "{log}Blah blah.{/log}")
// 	works(t, "{debugger}")
// 	works(t, "{let $foo : 1 + 2/}\n")
// 	works(t, "{let $foo : '\"'/}\n")
// 	works(t, "{let $foo}Hello{/let}\n")
// 	works(t, "{let $foo kind=\"html\"}Hello{/let}\n")

// 	fails(t, "{msg}blah{/msg}")
// 	fails(t, "{/msg}")
// 	fails(t, "{msg desc=\"\"}<a href=http://www.google.com{/msg}")
// 	fails(t, "{msg desc=\"\"}blah{msg desc=\"\"}bleh{/msg}bluh{/msg}")
// 	fails(t, "{msg desc=\"\"}blah{/msg blah}")
// 	fails(t, "{namespace}")
// 	fails(t, "{template}\n"+"blah\n"+"{/template}\n")
// 	fails(t, "{msg}<blah<blah>{/msg}")
// 	fails(t, "{msg}blah>blah{/msg}")
// 	fails(t, "{msg}<blah>blah>{/msg}")
// 	fails(t, "{print $boo /}")
// 	fails(t, "{if true}aaa{else/}bbb{/if}")
// 	fails(t, "{call .aaa.bbb /}")
// 	fails(t, "{delcall name=\"ddd.eee\"}{param foo: 0}{/call}")
// 	fails(t, "{delcall .dddEee /}")
// 	fails(t, "{msg desc=\"\"}{$boo phname=\"boo.foo\"}{/msg}")
// 	fails(t, "{msg desc=\"\"}<br phname=\"boo-foo\" />{/msg}")
// 	fails(t, "{msg desc=\"\"}{call .boo phname=\"boo\" phname=\"boo\" /}{/msg}")
// 	fails(t, "{msg desc=\"\"}<br phname=\"break\" phname=\"break\" />{/msg}")
// 	fails(t, "{call name=\".aaa\"}{param boo kind=\"html\": 123 /}{/call}\n")
// 	fails(t, "{log}")
// 	fails(t, "{log 'Blah blah.'}")
// 	fails(t, "{let $foo kind=\"html\" : 1 + 1/}\n")
// }

// func TestRecognizeComments(t *testing.T) {
// 	works(t, "blah // }\n"+
// 		"{$boo}{msg desc=\"\"} //}\n"+
// 		"{/msg} // {/msg}\n"+
// 		"{foreach $item in $items}\t// }\n"+
// 		"{$item.name}{/foreach} //{{{{\n")
// 	works(t, "blah /* } */\n"+
// 		"{msg desc=\"\"} /*}*/{$boo}\n"+
// 		"/******************/ {/msg}\n"+
// 		"/* {}} { }* }* / }/ * { **}  //}{ { } {\n"+
// 		"\n  } {//*} {* /} { /* /}{} {}/ } **}}} */\n"+
// 		"{foreach $item in $items} /* }\n"+
// 		"{{{{{*/{$item.name}{/foreach}/*{{{{*/\n")
// 	works(t, "//}\n")
// 	works(t, " //}\n")
// 	works(t, "\n//}\n")
// 	works(t, "\n //}\n")

// 	fails(t, "{blah /* { */ blah}")
// 	fails(t, "{foreach $item // }\n"+
// 		"         in $items}\n"+
// 		"{$item}{/foreach}\n")
// 	fails(t, "aa////}\n")
// 	fails(t, "{nil}//}\n")
// }

// // func TestParseComments(t *testing.T) {

// // 	templateBody :=
// //         "  {sp}  // {sp}\n" +  // first {sp} outside of comments
// //         "  /* {sp} {sp} */  // {sp}\n" +
// //         "  /* {sp} */{sp}/* {sp} */\n" +  // middle {sp} outside of comments
// //         "  /* {sp}\n" +
// //         "  {sp} */{sp}\n" +  // last {sp} outside of comments
// //         "  // {sp} /* {sp} */\n" +
// //         "  http://www.google.com\n";  // not a comment if "//" preceded by a non-space such as ":"

// //     List<StandaloneNode> nodes = parseTemplateBody(templateBody);
// //     assertEquals(1, nodes.size());
// //     assertEquals("   http://www.google.com", ((RawRawTextNode) nodes.get(0)).getRawText());
// //   }

// // public void testParseRawText() throws Exception {

// //   String templateBody =
// //       "  {sp} aaa bbb  \n" +
// //       "  ccc {lb}{rb} ddd {\\n}\n" +
// //       "  eee <br>\n" +
// //       "  fff\n" +
// //       "  {literal}ggg\n" +
// //       "hhh }{  {/literal}  \n" +
// //       "  \u2222\uEEEE\u9EC4\u607A\n";

// //   List<StandaloneNode> nodes = parseTemplateBody(templateBody);
// //   assertEquals(1, nodes.size());
// //   RawRawTextNode rtn = (RawRawTextNode) nodes.get(0);
// //   assertEquals(
// //       "  aaa bbb ccc {} ddd \neee <br>fffggg\nhhh }{  \u2222\uEEEE\u9EC4\u607A",
// //       rtn.getRawText());
// //   assertEquals(
// //       "  aaa bbb ccc {lb}{rb} ddd {\\n}eee <br>fffggg{\\n}hhh {rb}{lb}  \u2222\uEEEE\u9EC4\u607A",
// //       rtn.toSourceString());
// // }

// func works(t *testing.T, body string) {
// 	_, err := New(body).Parse(body, nil)
// 	if err != nil {
// 		t.Error(err)
// 	}
// }

// func fails(t *testing.T, body string) {
// 	_, err := New(body).Parse(body, nil)
// 	if err == nil {
// 		t.Errorf("should fail: %s", body)
// 	}
// }
