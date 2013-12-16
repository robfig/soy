package soy

import (
	"bytes"
	"fmt"
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

var parseTests = []parseTest{
	{"empty", "", tList()},
	{"namespace", "{namespace example}", tList(newNamespace(0, "example"))},
	{"empty template", "{template .name}{/template}", tList(tTemplate(".name"))},
	{"text template", "{template .name}\nHello world!\n{/template}",
		tList(tTemplate(".name", newText(0, "\nHello world!\n")))},
	{"variable template", "{template .name}\nHello {$name}!\n{/template}",
		tList(tTemplate(".name",
			newText(0, "\nHello "),
			newVariable(0, "$name"),
			newText(0, "!\n"),
		))},
	{"soydoc", "/** Text\n*/", tList(newSoyDoc(0, "/** Text\n*/"))},

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
		if !eqTree(tmpl.Root, test.tree) {
			t.Errorf("%s=(%q): got\n\t%v\nexpected\n\t%v", test.name, test.input, tmpl.Root, test.tree)
		}
	}
}

func eqTree(actual, expected Node) bool {
	if actual.Type() != expected.Type() {
		return false
	}

	switch actual.Type() {
	case NodeList:
		return eqNodes(expected.(*ListNode).Nodes, actual.(*ListNode).Nodes)
	case NodeNamespace:
		return expected.(*NamespaceNode).Name == actual.(*NamespaceNode).Name
	case NodeTemplate:
		if expected.(*TemplateNode).Name != actual.(*TemplateNode).Name {
			return false
		}
		return eqTree(expected.(*TemplateNode).Body, actual.(*TemplateNode).Body)
	case NodeText:
		return bytes.Equal(expected.(*TextNode).Text, actual.(*TextNode).Text)
	case NodeVariable:
		return expected.(*VariableNode).Name == actual.(*VariableNode).Name
	case NodeSoyDoc:
		return expected.(*SoyDocNode).Comment == actual.(*SoyDocNode).Comment
	}

	panic("type not implemented:" + actual.String())
}

func eqNodes(actual, expected []Node) bool {
	if len(expected) != len(actual) {
		return false
	}
	for i := range actual {
		if !eqTree(actual[i], expected[i]) {
			return false
		}
	}
	return true
}
