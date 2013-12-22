package soy

import (
	"bytes"
	"fmt"
	"testing"
)

type data map[string]interface{}

type execTest struct {
	name         string
	templateName string
	input        string
	output       string
	data         map[string]interface{}
	ok           bool
}

func exprtestwdata(name, expr, result string, data map[string]interface{}) execTest {
	return execTest{name, "test." + name,
		"{namespace test}{template ." + name + "}" + expr + "{/template}",
		result, data, true}
}
func exprtest(name, expr, result string) execTest {
	return exprtestwdata(name, expr, result, nil)
}

type datatest struct {
	data   map[string]interface{}
	result string
}

type errortest struct {
	data map[string]interface{}
}

func multidatatest(name, body string, successes []datatest, failures []errortest) []execTest {
	var execTests []execTest
	for i, t := range successes {
		execTests = append(execTests, execTest{
			fmt.Sprintf("%s (success %d) (%v)", name, i, t.data),
			"test." + name,
			"{namespace test}{template ." + name + "}" + body + "{/template}",
			t.result,
			t.data,
			true,
		})
	}
	for i, t := range failures {
		execTests = append(execTests, execTest{
			fmt.Sprintf("%s (fail %d) (%v)", name, i, t.data),
			"test." + name,
			"{namespace test}{template ." + name + "}" + body + "{/template}",
			"",
			t.data,
			false,
		})
	}
	return execTests
}

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

	// Line joining
	exprtest("helloLineJoin", "\n  Hello\n\n  world!\n", "Hello world!"),

	// Variables
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

// TODO: Test nullsafe dataref expression

// multidata tests execute a single, more complicated template multiple times
// with different data.
var multidataTests = [][]execTest{
	multidatatest("if", `
{if $zoo}{$zoo}{/if}
{if $boo}
  X
{elseif $foo.goo > 2}
  {$boo}
{else}
  Y {$moo}
{/if}`, []datatest{
		{data{"foo": data{"goo": 0}}, "Y null"},
		{data{"zoo": "abc", "foo": data{"goo": 0}}, "abcY null"},
		{data{"zoo": "", "boo": 1}, "X"},
		{data{"zoo": 0, "boo": true}, "X"},
		{data{"boo": "abc"}, "X"},
		{data{"boo": "", "foo": data{"goo": 2}}, "Y null"},
		{data{"boo": 0, "foo": data{"goo": 3}}, "0"},
		{data{"boo": 0, "foo": data{"goo": 3.0}}, "0"},
		{data{"zoo": "zoo", "foo": data{"goo": 0}, "moo": 3}, "zooY 3"},
	}, []errortest{
		{nil},
		{data{"foo": nil}},             // $foo.goo fails
		{data{"foo": "str"}},           // $foo.goo must be number
		{data{"foo": true}},            // $foo.goo must be number
		{data{"foo": data{}}},          // $foo.goo must be number
		{data{"foo": []interface{}{}}}, // $foo.goo must be number
	}),

	multidatatest("foreach", `
{foreach $goo in $goose}
  {$goo.numKids} goslings.{\n}
{/foreach}
{foreach $boo in $foo.booze}
  Scary drink {$boo.name}!
` /*  {if not isLast($boo)}{\n}{/if} */ +`
{ifempty}
  Sorry, no booze.
{/foreach}`, []datatest{
		{data{
			"goose": []interface{}{},
			"foo":   data{"booze": []interface{}{}},
		}, "Sorry, no booze."},
		{data{
			"goose": []interface{}{},
			"foo":   data{"booze": []interface{}{data{"name": "boo"}}},
		}, "Scary drink boo!"},
		{data{
			"goose": []interface{}{data{"numKids": 1}, data{"numKids": 2}},
			"foo":   data{"booze": []interface{}{}},
		}, "1 goslings.\n2 goslings.\nSorry, no booze."},
	}, []errortest{
		{nil},                                    // non-null-safe eval of $foo.booze fails
		{data{"foo": nil}},                       // ditto
		{data{"foo": data{}}},                    // $foo.booze must be a list
		{data{"foo": data{"booze": "str"}}},      // $foo.booze must be list
		{data{"foo": data{"booze": 5}}},          // $foo.booze must be list
		{data{"foo": data{"booze": data{}}}},     // $foo.booze must be list
		{data{"foo": data{"booze": true}}},       // $foo.booze must be list
		{data{"foo": data{"booze": []data{{}}}}}, // $boo.name fails
	}),
}

func init() {
	for _, mdt := range multidataTests {
		execTests = append(execTests, mdt...)
	}
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
