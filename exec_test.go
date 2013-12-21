package soy

import (
	"bytes"
	"testing"
)

type data map[string]interface{}

type execTest struct {
	name         string
	templateName string
	input        string
	output       string
	data         interface{}
	ok           bool
}

var execTests = []execTest{
	// Namespace + static template
	{"empty", "test.empty",
		"{namespace test}\n{template .empty}{/template}",
		"",
		nil, true},
	{"hello world", "test.sayHello",
		"{namespace test}\n{template .sayHello}Hello world!{/template}",
		"Hello world!",
		nil, true},
	{"hello world w/ soydoc", "test.sayHello",
		"{namespace test}\n/** Says hello */\n{template .sayHello}Hello world!{/template}",
		"Hello world!",
		nil, true},

	// Expression
	{"arithmetic", "test.arith",
		"{namespace test}{template .arith}{2*(1+1)/(2%4)}{/template}",
		"2",
		nil, true},
	{"bools", "test.bools",
		"{namespace test}{template .bools}{not false and (2 > 5.0 or (null ?: true))}{/template}",
		"true",
		nil, true},
	{"comparisons", "test.compare",
		`{namespace test}{template .compare}{0.5<=1 ? null?:'hello' : (1!=1)}{/template}`,
		"hello",
		nil, true},
	{"string concat", "test.concat",
		`{namespace test}{template .concat}{'hello' + 'world'}{/template}`,
		"helloworld",
		nil, true},

	// TODO: Elvis only checks if cond is defined, not truthy.

	// Line joining
	{"linejoin hello", "test.sayHello",
		"{namespace test}\n{template .sayHello}\n  Hello\n\n  world!\n{/template}",
		"Hello world!",
		nil, true},

	// Variables
	// TODO: "undefined data keys are falsy"
	{"hello world w/ variable", "test.sayHello",
		`{namespace test}

/** @param name */
{template .sayHello}
Hello {$name}!
{/template}`,
		"Hello Rob!",
		data{"name": "Rob"}, true},

	// 	{"call w/ line join", "test.callLine",
	// 		`{namespace test}

	// {template .callLine}
	// Hello <a>{call .guy}</a>!
	// {/template}

	// {template .guy}
	//   Rob
	// {/template}
	// `,
	// 		" Hello <a>Rob</a>! ",
	// 		nil, true},

	// // Invalid
	// {"missing namespace", ".sayHello",
	// 	"{template .sayHello}Hello world!{/template}",
	// 	"",
	// 	nil, false},
}

func TestExec(t *testing.T) {
	b := new(bytes.Buffer)
	for _, test := range execTests {
		var tofu = New()
		var err = tofu.Parse(test.input)
		if err != nil {
			t.Errorf("%s: parse error: %s", test.name, err)
			continue
		}
		b.Reset()
		tmpl, _ := tofu.Template(test.templateName)
		err = tmpl.Execute(b, test.data)
		switch {
		case !test.ok && err == nil:
			t.Errorf("%s: expected error; got none", test.name)
			continue
		case test.ok && err != nil:
			t.Errorf("%s: unexpected execute error: %s", test.name, err)
			continue
		case !test.ok && err != nil:
			// expected error, got one
		}
		result := b.String()
		if result != test.output {
			t.Errorf("%s: expected\n\t%q\ngot\n\t%q", test.name, test.output, result)
		}
	}
}
