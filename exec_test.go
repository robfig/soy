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

func exprtestwdata(name, expr, result string, data interface{}) execTest {
	return execTest{name, "test." + name,
		"{namespace test}{template ." + name + "}" + expr + "{/template}",
		result, data, true}
}
func exprtest(name, expr, result string) execTest {
	return exprtestwdata(name, expr, result, nil)
}

var ifExpr = `{if $zoo}{$zoo}{/if}
{if $boo}
  Blah
{elseif $foo.goo > 2}
  {$boo}
{else}
  Blah {$moo}
{/if}`

var execTests = []execTest{
	// Namespace + static template
	exprtest("empty", "", ""),
	exprtest("sayHello", "Hello world!", "Hello world!"),
	{"hello world w/ soydoc", "test.sayHello",
		"{namespace test}\n/** Says hello */\n{template .sayHello}Hello world!{/template}",
		"Hello world!",
		nil, true},

	// Expression
	exprtest("arithmetic", "{2*(1+1)/(2%4)}", "2"),
	exprtest("bools", "{not false and (2 > 5.0 or (null ?: true))}", "true"),
	exprtest("comparisons", `{0.5<=1 ? null?:'hello' : (1!=1)}`, "hello"),
	exprtest("stringconcat", `{'hello' + 'world'}`, "helloworld"),
	exprtest("mixedconcat", `{5 + 'world'}`, "5world"),
	exprtest("elvis", `{null?:'hello'}`, "hello"), // elvis does isNonnull check on first arg
	// exprtest("elvis2", `{0?:'hello'}`, "0"),

	// Control flow
	//exprtestdata("if", ifExpr, "Blah", nil),

	// Line joining
	exprtest("helloLineJoin", "\n  Hello\n\n  world!\n", "Hello world!"),

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

	// {"call w/ line join", "test.callLine",
	// 	`{namespace test}

	// {template .callLine}
	// Hello <a>{call .guy/}</a>!
	// {/template}

	// {template .guy}
	//   Rob
	// {/template}
	// `,
	// 	" Hello <a>Rob</a>! ",
	// 	nil, true},

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
