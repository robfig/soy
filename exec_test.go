package soy

import (
	"bytes"
	"fmt"
	"log"
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
		exprtest("negate", `{-(1+1)}`, "-2"),
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
  {if isFirst($boo)}->{\n}{/if}
  {index($boo)}: Scary drink {$boo.name}!
  {if not isLast($boo)}{\n}{/if}
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
		}, "->\n0: Scary drink boo!"},
		{data{
			"goose": []interface{}{},
			"foo":   data{"booze": []interface{}{data{"name": "a"}, data{"name": "b"}}},
		}, "->\n0: Scary drink a!\n1: Scary drink b!"},
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

func TestFor(t *testing.T) {
	runExecTests(t, multidatatest("for", `
{for $i in range(1, length($items) + 1)}
  {msg desc="Numbered item."}
    {$i}: {$items[$i - 1]}{\n}
  {/msg}
{/for}`, []datatest{
		{data{"items": []interface{}{}}, ""},
		{data{"items": []interface{}{"car"}}, "1: car\n"},
		{data{"items": []interface{}{"car", "boat"}}, "1: car\n2: boat\n"},
	}, []errortest{
		{data{}},             // undefined is not a valid slice
		{data{"items": nil}}, // null is not a valid slice
		{data{"items": "a"}}, // string is not a valid slice
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

func TestCall(t *testing.T) {
	runExecTests(t, []execTest{
		{"call", "test.call", `{namespace test}

{template .call}
{call .boo_ /}{sp}
{call .boo_ data="all"/}{sp}
{call .zoo data="$animals"}
      // Note that the {param} blocks for the message and listItems parameter are declared to have
      // content of kind HTML. This instructs the contextual autoescaper to process the content of
      // these blocks as HTML, and to wrap the the value of the parameter as a soydata.SanitizedHtml
      // object.
  {param yoo: round($too) /}
  {param woo}poo{/param}
  {param zoo: 0 /}
  {param doo kind="html"}doopoo{/param}
{/call}
{/template}

{template .boo_}
  {if $animals}
    Yay!
  {else}
    Boo!
  {/if}
{/template}

{template .zoo}
 {$zoo} {$animal} {$woo}!
{/template}`,
			"Boo! Yay! 0 roos poo!",
			data{"animals": data{"animal": "roos"}, "too": 2.4},
			true,
		},
	})
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

// TODO: Can you add special chars within tags too?
func TestSpecialChars(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("special chars", `{sp}{nil}{\r}{\n}{\t}{lb}{rb}`, " \r\n\t{}"),
		exprtest("without nil there is space", "abc\ndef", "abc def"),
		exprtest("nil avoids space", "abc{nil}\ndef", "abcdef"),
		exprtest("without sp there is no space", "abc\n<a>", "abc<a>"),
		exprtest("sp adds space", "abc{sp}\n<a>", "abc <a>"),
	})
}

func TestLiteral(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("literal",
			`{literal} {/call}\n {sp} // comment {/literal}`,
			` {/call}\n {sp} // comment `),
	})
}

func TestCss(t *testing.T) {
	runExecTests(t, []execTest{
		exprtestwdata("css",
			`<div class="{css my-css-class}"></div> <a class="{css $component, a-class}">link</a>`,
			`<div class="my-css-class"></div> <a class="page-a-class">link</a>`,
			data{"component": "page"}),
	})
}

func TestLog(t *testing.T) {
	originalLogger := Logger
	defer func() { Logger = originalLogger }()

	var buf bytes.Buffer
	Logger = log.New(&buf, "", 0)
	runExecTests(t, []execTest{
		exprtestwdata("log", "{log} Hello {$name} // comment\n{/log}", ``, data{"name": "Rob"}),
	})
	if strings.TrimSpace(buf.String()) != "Hello Rob" {
		t.Errorf("logger didn't match: %q", buf.String())
	}
}

func TestDebugger(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("debugger", `{debugger}`, ``),
	})
}

func TestPrintDirectives(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("sanitized html", "{'<a>'}", "&lt;a&gt;"),
		exprtest("noAutoescape", "{'<a>'|noAutoescape}", "<a>"),
		exprtestwdata("sanitized var", "{$var}", "&lt;a&gt;", data{"var": "<a>"}),
		exprtestwdata("noAutoescape var", "{$var |noAutoescape}", "<a>", data{"var": "<a>"}),

		// |id == |noAutoescape (it's deprecated)
		exprtest("id", "{'<a>'|id}", "<a>"),

		// TODO: no way to disable html escaping yet.
		// exprtest("escapeHtml", "{'<a>'|escapeHtml}", "&lt;a&gt;"),

		exprtest("escapeUri1", "{''|escapeUri}", ""),
		exprtest("escapeUri2", "{'a%b > c'|escapeUri}", "a%25b+%3E+c"),
		// TODO: test it escapes kind=HTML content
		// TODO: test it does not escape kind=URI content

		exprtestwdata("ejs1", "{$var|escapeJsString}", ``, data{"var": ""}),
		exprtestwdata("ejs2", "{$var|escapeJsString}", `foo`, data{"var": "foo"}),
		exprtestwdata("ejs3", "{$var|escapeJsString}", `foo\\bar`, data{"var": "foo\\bar"}),
		// TODO: test it even escapes "kind=HTML" content
		// TODO: test it does not escape "kind=JS_STR" content
		exprtestwdata("ejs4", "{$var|escapeJsString}", `\\`, data{"var": "\\"}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\'\'`, data{"var": "''"}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\"foo\"`, data{"var": `"foo"`}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `42`, data{"var": 42}),

		exprtest("truncate", "{'Lorem Ipsum' |truncate:8}", "Lorem..."),
		exprtest("truncate w arg", "{'Lorem Ipsum' |truncate:8,false}", "Lorem Ip"),
		exprtest("truncate w expr", "{'Lorem Ipsum' |truncate:5+3,not true}", "Lorem Ip"),

		exprtest("insertWordBreaks", "{'1234567890'|insertWordBreaks:3}", "123<wbr>456<wbr>789<wbr>0"),
		exprtest("insertWordBreaks2", "{'123456789'|insertWordBreaks:3}", "123<wbr>456<wbr>789"),
		exprtest("insertWordBreaks3", "{'123456789'|insertWordBreaks:30}", "123456789"),
		exprtest("insertWordBreaks4", "{'12 345 6789'|insertWordBreaks:3}", "12 345 678<wbr>9"),
		exprtest("insertWordBreaks5", "{''|insertWordBreaks:3}", ""),

		exprtestwdata("nl2br", "{$var|changeNewlineToBr}", "<br>1<br>2<br>3<br><br>4<br><br>",
			data{"var": "\r1\n2\r3\r\n\n4\n\n"}),
	})
}

func TestGlobals(t *testing.T) {
	globals["app.global_str"] = "abc"
	globals["GLOBAL_INT"] = 5
	globals["global.nil"] = nil
	runExecTests(t, []execTest{
		exprtest("global", `{app.global_str} {GLOBAL_INT + 2} {global.nil?:'hi'}`, `abc 7 hi`),
	})
}

// helpers

var globals = make(map[string]interface{})

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
		tofu.globals = globals
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
