package soymsg

import (
	"testing"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
)

func TestSetPlaceholders(t *testing.T) {
	type test struct {
		node  *ast.MsgNode
		phstr string
	}

	var tests = []test{
		{newMsg("Hello world"), "Hello world"},
		{newMsg("Hello {$name}"), "Hello {NAME}"},
		{newMsg("{$a}, {$b}, and {$c}"), "{A}, {B}, and {C}"},
		{newMsg("{$a} {$a}"), "{A} {A}"},
		{newMsg("{$a} {$b.a}"), "{A_1} {A_2}"},
		{newMsg("{$a.a}{$a.b.a}"), "{A_1}{A_2}"},

		// Command sequences
		{newMsg("hello{sp}world"), "hello world"},

		// TODO: investigate globals
		// {newMsg("{GLOBAL}"), "{GLOBAL}"},
		// {newMsg("{sub.global}"), "{GLOBAL}"},
	}

	for _, test := range tests {
		var actual = test.node.PlaceholderString()
		if actual != test.phstr {
			t.Errorf("(actual) %v != %v (expected)", actual, test.phstr)
		}
	}
}

func newMsg(msg string) *ast.MsgNode {
	var sf, err = parse.SoyFile("", `{msg desc=""}`+msg+`{/msg}`,
		data.Map{"GLOBAL": data.Int(1), "sub.global": data.Int(2)})
	if err != nil {
		panic(err)
	}
	var msgnode = sf.Body[0].(*ast.MsgNode)
	SetPlaceholdersAndID(msgnode)
	return msgnode
}

func TestBaseName(t *testing.T) {
	type test struct {
		expr string
		ph   string
	}
	var tests = []test{
		{"$foo", "FOO"},
		{"$foo.boo", "BOO"},
		{"$foo.boo[0].zoo", "ZOO"},
		{"$foo.boo.0.zoo", "ZOO"},

		// parse.Expr doesn't accept undefined globals.
		// {"GLOBAL", "GLOBAL"},
		// {"sub.GLOBAL", "GLOBAL"},

		{"$foo[0]", "XXX"},
		{"$foo.boo[0]", "XXX"},
		{"$foo.boo.0", "XXX"},
		{"$foo + 1", "XXX"},
		{"'text'", "XXX"},
		{"max(1, 3)", "XXX"},
	}

	for _, test := range tests {
		var n, err = parse.Expr(test.expr)
		if err != nil {
			t.Error(err)
			return
		}

		var actual = genBasePlaceholderName(&ast.PrintNode{0, n, nil})
		if actual != test.ph {
			t.Errorf("(actual) %v != %v (expected)", actual, test.ph)
		}
	}
}

func TestToUpperUnderscore(t *testing.T) {
	var tests = []struct{ in, out string }{
		{"booFoo", "BOO_FOO"},
		{"_booFoo", "BOO_FOO"},
		{"booFoo_", "BOO_FOO"},
		{"BooFoo", "BOO_FOO"},
		{"boo_foo", "BOO_FOO"},
		{"BOO_FOO", "BOO_FOO"},
		{"__BOO__FOO__", "BOO_FOO"},
		{"Boo_Foo", "BOO_FOO"},
		{"boo8Foo", "BOO_8_FOO"},
		{"booFoo88", "BOO_FOO_88"},
		{"boo88_foo", "BOO_88_FOO"},
		{"_boo_8foo", "BOO_8_FOO"},
		{"boo_foo8", "BOO_FOO_8"},
		{"_BOO__8_FOO_", "BOO_8_FOO"},
	}
	for _, test := range tests {
		var actual = toUpperUnderscore(test.in)
		if actual != test.out {
			t.Errorf("(actual) %v != %v (expected)", actual, test.out)
		}
	}
}
