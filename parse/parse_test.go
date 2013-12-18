package parse

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
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

func newString(val string) *StringNode {
	s, err := strconv.Unquote(val)
	if err != nil {
		panic(err)
	}
	return &StringNode{0, val, s}
}

var parseTests = []parseTest{
	{"empty", "", tList()},
	{"namespace", "{namespace example}", tList(newNamespace(0, "example"))},
	{"empty template", "{template .name}{/template}", tList(tTemplate(".name"))},
	{"text template", "{template .name}\nHello world!\n{/template}",
		tList(tTemplate(".name", newText(0, "\nHello world!\n")))},
	{"variable template", "{template .name}\nHello {$name}!\n{/template}",
		tList(tTemplate(".name",
			newText(0, "\nHello "),
			newPrint(0, &VariableNode{0, "$name"}), // implicit print
			newText(0, "!\n"),
		))},
	{"soydoc", "/** Text\n*/", tList(newSoyDoc(0, "/** Text\n*/"))},
	{"negate", "{not $var}", tList(&PrintNode{0, &NotNode{0, &VariableNode{0, "$var"}}})},
	{"concat", `{"hello" + "world"}`, tList(&PrintNode{0,
		&AddNode{bin(
			newString(`"hello"`),
			newString(`"world"`))},
	})},

	{"expression1", "{not false and (isFirst($foo) or ($x - 5) > 3.1)}", tList(&PrintNode{0,
		&AndNode{bin(
			&NotNode{0, &BoolNode{0, false}},
			&OrNode{bin(
				&FunctionNode{0, "isFirst", []Node{&VariableNode{0, "$foo"}}},
				&GtNode{bin(
					&SubNode{bin(
						&VariableNode{0, "$x"},
						&IntNode{0, 5})},
					&FloatNode{0, 3.1})})})},
	})},

	{"expression2", `{null or ("foo" == "f"+true ? 3 <= 5 : not $foo ?: bar(5))}`, tList(&PrintNode{0,
		&OrNode{bin(
			&NullNode{0},
			&TernNode{0,
				&EqNode{bin(
					newString(`"foo"`),
					&AddNode{bin(
						newString(`"f"`),
						&BoolNode{0, true})})},
				&LteNode{bin(
					&IntNode{0, 3},
					&IntNode{0, 5})},
				&ElvisNode{bin(
					&NotNode{0, &VariableNode{0, "$foo"}},
					&FunctionNode{0, "bar", []Node{&IntNode{0, 5}}})}})},
	})},

	{"expression3", `{"a"+"b" != "ab" and (2 >= 5.0 or (null ?: true))}`, tList(&PrintNode{0,
		&AndNode{bin(
			&NotEqNode{bin(
				&AddNode{bin(
					newString(`"a"`),
					newString(`"b"`))},
				newString(`"ab"`))},
			&OrNode{bin(
				&GteNode{bin(&IntNode{0, 2}, &FloatNode{0, 5.0})},
				&ElvisNode{bin(
					&NullNode{0},
					&BoolNode{0, true})})})},
	})},

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

		t.Log("Expected:")
		printTree(t, test.tree, 0)
		t.Log("Actual:")
		if tmpl == nil {
			t.Log("<nil>")
		} else {
			printTree(t, tmpl.Root, 0)
		}

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
		if !eqTree(t, tmpl.Root, test.tree) {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, tmpl.Root, test.tree)
		}
	}
}

func eqTree(t *testing.T, actual, expected Node) bool {
	if reflect.TypeOf(actual) != reflect.TypeOf(expected) {
		t.Errorf("expected %T, got %T", expected, actual)
		return false
	}

	switch actual.(type) {
	case *ListNode:
		return eqNodes(t, expected.(*ListNode).Nodes, actual.(*ListNode).Nodes)
	case *NamespaceNode:
		return expected.(*NamespaceNode).Name == actual.(*NamespaceNode).Name
	case *TemplateNode:
		if expected.(*TemplateNode).Name != actual.(*TemplateNode).Name {
			return false
		}
		return eqTree(t, expected.(*TemplateNode).Body, actual.(*TemplateNode).Body)
	case *TextNode:
		return bytes.Equal(expected.(*TextNode).Text, actual.(*TextNode).Text)
	case *VariableNode:
		return expected.(*VariableNode).Name == actual.(*VariableNode).Name
	case *NullNode:
		return true
	case *BoolNode:
		return expected.(*BoolNode).True == actual.(*BoolNode).True
	case *IntNode:
		return expected.(*IntNode).Value == actual.(*IntNode).Value
	case *FloatNode:
		return expected.(*FloatNode).Value == actual.(*FloatNode).Value
	case *StringNode:
		return expected.(*StringNode).Quoted == actual.(*StringNode).Quoted
	case *TernNode:
		return eqTree(t, expected.(*TernNode).Arg1, actual.(*TernNode).Arg1) &&
			eqTree(t, expected.(*TernNode).Arg2, actual.(*TernNode).Arg2) &&
			eqTree(t, expected.(*TernNode).Arg3, actual.(*TernNode).Arg3)
	case *FunctionNode:
		return expected.(*FunctionNode).Name == actual.(*FunctionNode).Name &&
			eqNodes(t, expected.(*FunctionNode).Args, actual.(*FunctionNode).Args)
	case *SoyDocNode:
		return expected.(*SoyDocNode).Comment == actual.(*SoyDocNode).Comment
	case *NotNode:
		return eqTree(t, expected.(*NotNode).Arg, actual.(*NotNode).Arg)
	case *PrintNode:
		return eqTree(t, expected.(*PrintNode).Arg, actual.(*PrintNode).Arg)
	case *MulNode, *DivNode, *ModNode, *AddNode, *SubNode, *EqNode, *NotEqNode,
		*GtNode, *GteNode, *LtNode, *LteNode, *OrNode, *AndNode, *ElvisNode:
		return eqBinOp(t, expected, actual)
	}

	panic(fmt.Sprintf("type not implemented: %T", actual))
}

// eqBinOp compares structs that embed binaryOpNode
func eqBinOp(t *testing.T, n1, n2 interface{}) bool {
	var (
		op1 = reflect.ValueOf(n1).Elem().Field(0).Interface().(binaryOpNode)
		op2 = reflect.ValueOf(n2).Elem().Field(0).Interface().(binaryOpNode)
	)
	return eqTree(t, op1.Arg1, op2.Arg1) && eqTree(t, op1.Arg2, op2.Arg2)
}

func eqNodes(t *testing.T, actual, expected []Node) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range actual {
		if !eqTree(t, actual[i], expected[i]) {
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
