package soy

import (
	"bytes"
	"fmt"
	"strings"
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

func TestBasicExec(t *testing.T) {
	runExecTests(t, []execTest{
		// Namespace + static template
		exprtest("empty", "", ""),
		exprtest("sayHello", "Hello world!", "Hello world!"),
		{"hello world w/ soydoc", "test.sayHello",
			"{namespace test}\n/** Says hello */\n{template .sayHello}Hello world!{/template}",
			"Hello world!",
			nil, true},

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

	})
}

func TestExpressions(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("arithmetic", "{2*(1+1)/(2%4)}", "2"),
		exprtest("bools", "{not false and (2 > 5.0 or (null ?: true))}", "true"),
		exprtest("bools2", "{2*(1.5+1) < 3 ? 'nope' : (2 >= 2) == (5.5<6) != true }", "false"),
		exprtest("bools3", "{null or 0.0 or ([:] and [])}", "true"), // map/list is truthy
		exprtest("bools4", "{'a' == 'a'}", "true"),
		exprtest("bools5", "{null == $foo}", "false"),
		exprtest("bools6", "{null == null}", "true"),
		exprtest("bools7", "{$foo == $foo}", "true"),
		exprtest("comparisons", `{0.5<=1 ? null?:'hello' : (1!=1)}`, "hello"),
		exprtest("stringconcat", `{'hello' + 'world'}`, "helloworld"),
		exprtest("mixedconcat", `{5 + 'world'}`, "5world"),
		exprtest("elvis", `{null?:'hello'}`, "hello"),   // elvis does isNonnull check on first arg
		exprtest("elvis2", `{$foo?:'hello'}`, "hello"),  // elvis does isNonnull check on first arg
		exprtest("elvis3", `{0?:'hello'}`, "0"),         // 0 is non-null
		exprtest("elvis4", `{false?:'hello'}`, "false"), // false is non-null
	})
}

func TestIf(t *testing.T) {
	runExecTests(t, multidatatest("if", `
{if $zoo}{$zoo}{/if}
{if $boo}
  X
{elseif $foo.goo > 2}
  {$boo}
{else}
  Y {$moo?:''}
{/if}`, []datatest{
		{data{"foo": data{"goo": 0}}, "Y "},
		{data{"zoo": "abc", "foo": data{"goo": 0}}, "abcY "},
		{data{"zoo": "", "boo": 1}, "X"},
		{data{"zoo": 0, "boo": true}, "X"},
		{data{"boo": "abc"}, "X"},
		{data{"boo": "", "foo": data{"goo": 2}}, "Y "},
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
	}))
}

func TestForeach(t *testing.T) {
	runExecTests(t, multidatatest("foreach", `
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
	}))
}

func TestSwitch(t *testing.T) {
	runExecTests(t, multidatatest("switch", `
{switch $boo} {case 0}A
  {case $foo.goo}
    B
  {case -1, 1, $moo}
    C
  {default}
    D
{/switch}`, []datatest{
		{data{
			"boo": 0,
		}, "A"},
		{data{
			"boo": 1,
			"foo": data{"goo": 1},
		}, "B"},
		{data{
			"boo": -1,
			"foo": data{"goo": 5},
		}, "C"},
		{data{
			"boo": 1,
			"foo": data{"goo": 5},
		}, "C"},
		{data{
			"boo": 2,
			"foo": data{"goo": 5},
		}, "D"},
		{data{
			"boo": 2,
			"foo": data{"goo": 5},
			"moo": 2,
		}, "C"},
	}, []errortest{}),
	)
}

func TestDataRefs(t *testing.T) {
	runExecTests(t, []execTest{
		// single key
		exprtestwdata("undefined", "{$foo}", "", nil).fails(),        // undefined = error
		exprtestwdata("null", "{$foo}", "null", data{"foo": nil}),    // null prints
		exprtestwdata("string", "{$foo}", "foo", data{"foo": "foo"}), // string print
		exprtestwdata("list", "{$foo}", "[a, 5, [2.5], [null]]",
			data{"foo": []interface{}{"a", 5, []interface{}{2.5}, []interface{}{nil}}}), // list print
		exprtestwdata("map", "{$foo}", "{a: 5, b: [true], c: {}}",
			data{"foo": data{"a": 5, "b": []interface{}{true}, "c": data{}}}), // map print

		// index lookups
		exprtestwdata("basic", "{$foo.2}", "result",
			data{"foo": []interface{}{"a", 5, "result"}}),
		exprtestwdata("out of bounds", "{$foo.7}", "",
			data{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("undefined slice", "{$foo.2}", "", data{}).fails(),
		exprtestwdata("null slice", "{$foo.2}", "", data{"foo": nil}).fails(),
		exprtestwdata("nullsafe on undefined slice", "{$foo?.2}", "null", data{}),
		exprtestwdata("nullsafe on null slice", "{$foo?.2}", "null", data{"foo": nil}),
		exprtestwdata("nullsafe does not save out of bounds", "{$foo?.2}", "",
			data{"foo": []interface{}{"a"}}).fails(),
		exprtestwdata("lookup on nonslice", "{$foo?.2}", "", data{"foo": "hello"}).fails(),

		// key lookups
		exprtestwdata("basic", "{$foo.bar}", "result", data{"foo": data{"bar": "result"}}),
		exprtestwdata("undefined map", "{$foo.bar}", "", data{}).fails(),
		exprtestwdata("null map", "{$foo.bar}", "", data{"foo": nil}).fails(),
		exprtestwdata("null value is ok", "{$foo.bar}", "null", data{"foo": data{"bar": nil}}),
		exprtestwdata("nullsafe on undefined map", "{$foo?.bar}", "null", data{}),
		exprtestwdata("nullsafe on null map", "{$foo?.bar}", "null", data{"foo": nil}),
		exprtestwdata("lookup on nonmap", "{$foo?.bar}", "", data{"foo": "hello"}).fails(),

		// expr lookups (index)
		exprtestwdata("exprbasic", "{$foo[2]}", "result", data{"foo": []interface{}{"a", 5, "result"}}),
		exprtestwdata("exprout of bounds", "{$foo[7]}", "", data{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("exprundefined slice", "{$foo[2]}", "", data{}).fails(),
		exprtestwdata("exprnull slice", "{$foo[2]}", "", data{"foo": nil}).fails(),
		exprtestwdata("exprnullsafe on undefined slice", "{$foo?[2]}", "null", data{}),
		exprtestwdata("exprnullsafe on null slice", "{$foo?[2]}", "null", data{"foo": nil}),
		exprtestwdata("exprnullsafe does not save out of bounds", "{$foo?[2]}", "",
			data{"foo": []interface{}{"a"}}).fails(),
		exprtestwdata("exprarith", "{$foo[1+1>3.0?8:1]}", "5", data{"foo": []interface{}{"a", 5, "z"}}),

		// expr lookups (key)
		exprtestwdata("exprkeybasic", "{$foo['bar']}", "result", data{"foo": data{"bar": "result"}}),
		exprtestwdata("exprkeyundefined map", "{$foo['bar']}", "", data{}).fails(),
		exprtestwdata("exprkeynull map", "{$foo['bar']}", "", data{"foo": nil}).fails(),
		exprtestwdata("exprkeynull value is ok", "{$foo['bar']}", "null", data{"foo": data{"bar": nil}}),
		exprtestwdata("exprkeynullsafe on undefined map", "{$foo?['bar']}", "null", data{}),
		exprtestwdata("exprkeynullsafe on null map", "{$foo?['bar']}", "null", data{"foo": nil}),
		exprtestwdata("exprkeyarith", "{$foo['b'+('a'+'r')]}", "result", data{"foo": data{"bar": "result"}}),
	})
}

// helpers

func (t execTest) fails() execTest {
	t.ok = false
	return t
}

func exprtestwdata(name, expr, result string, data map[string]interface{}) execTest {
	return execTest{name, "test." + strings.Replace(name, " ", "_", -1),
		"{namespace test}{template ." + strings.Replace(name, " ", "_", -1) + "}" + expr + "{/template}",
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

// multidata tests execute a single, more complicated template multiple times
// with different data.
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

func runExecTests(t *testing.T, tests []execTest) {
	b := new(bytes.Buffer)
	for _, test := range tests {
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
