package soyjs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/andreyvit/diff"
	"github.com/robertkrimen/otto"
	"github.com/robfig/soy"
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/soymsg"
	"github.com/robfig/soy/template"
)

// TODO: test all types of globals
// TODO: test all functions

// This is the same test suite as tofu/exec_test, verifying that the JS versions
// get the same result.

/** BEGIN COPIED TESTS (minus lines marked with DIFFERENCE) */

type d map[string]interface{}

type execTest struct {
	name         string
	templateName string
	input        string
	output       string
	data         interface{}
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
			d{"name": "Rob"}, true},

		{"call w/ line join", "test.callLine",
			`{namespace test}

		{template .callLine}
		Hello <a>{call .guy/}</a>!
		{/template}

		{template .guy}
		  Rob
		{/template}
		`,
			"Hello <a>Rob</a>!",
			nil, true},

		// Invalid
		{"missing namespace", ".sayHello",
			"{template .sayHello}Hello world!{/template}",
			"",
			nil, false},
	})
}

// DIFFERENCE: Boolean expressions print true/false on server but return a value on client.
// (This happens in official Soy)
func TestExpressions(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("arithmetic", "{2*(1+1)/(2%4)}", "2"),
		exprtest("arithmetic2", "{2.0-1.5}", "0.5"),
		exprtest("bools", "{not false and (2 > 5.0 or (null ?: true))}", "true"),
		exprtest("bools2", "{2*(1.5+1) < 3 ? 'nope' : (2 >= 2) == (5.5<6) != true }", "false"),
		// DIFFERENCE: official soy returns true but prints nothing.  (weird!)
		// ({true} prints true. {[:]} is truthy but prints nothing.)
		// exprtest("bools3", "{null or 0.0 or ([:] and [])}", "true"), // map/list is truthy
		exprtest("bools4", "{'a' == 'a'}", "true"),
		// exprtest("bools5", "{null == $foo}", "false"),  // DIFFERENCE
		exprtest("bools6", "{null == null}", "true"),
		// exprtest("bools7", "{$foo == $foo}", "true"),  // DIFFERENCE
		exprtest("comparisons", `{0.5<=1 ? null?:'hello' : (1!=1)}`, "hello"),
		exprtest("stringconcat", `{'hello' + 'world'}`, "helloworld"),
		exprtest("mixedconcat", `{5 + 'world'}`, "5world"),
		exprtest("elvis", `{null?:'hello'}`, "hello"), // elvis does isNonnull check on first arg
		//exprtest("elvis2", `{$foo?:'hello'}`, "hello"),  // elvis does isNonnull check on first arg
		exprtest("elvis3", `{0?:'hello'}`, "0"),         // 0 is non-null
		exprtest("elvis4", `{false?:'hello'}`, "false"), // false is non-null
		exprtest("negate", `{-(1+1)}`, "-2"),
		exprtest("negate float", `{-(1+1.5)}`, "-2.5"),

		// short-circuiting
		exprtest("shortcircuit precondition undef key fails", "{$undef.key}", "").fails(),
		// exprtest("shortcircuit and", "{$undef and $undef.key}", "undefined"),
		exprtest("shortcircuit or", "{'yay' or $undef.key}", "yay"), // DIFFERENCE
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
		{d{"foo": d{"goo": 0}}, "Y "},
		{d{"zoo": "abc", "foo": d{"goo": 0}}, "abcY "},
		{d{"zoo": "", "boo": 1}, "X"},
		{d{"zoo": 0, "boo": true}, "X"},
		{d{"boo": "abc"}, "X"},
		{d{"boo": "", "foo": d{"goo": 2}}, "Y "},
		{d{"boo": 0, "foo": d{"goo": 3}}, "0"},
		{d{"boo": 0, "foo": d{"goo": 3.0}}, "0"},
		{d{"zoo": "zoo", "foo": d{"goo": 0}, "moo": 3}, "zooY 3"},
	}, []errortest{
		{nil},
		{d{"foo": nil}}, // $foo.goo fails

		// DIFFERENCE: JS templates don't error on these.
		// {d{"foo": "str"}},           // $foo.goo must be number
		// {d{"foo": true}},            // $foo.goo must be number
		// {d{"foo": d{}}},             // $foo.goo must be number
		// {d{"foo": []interface{}{}}}, // $foo.goo must be number
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
		{d{
			"goose": []interface{}{},
			"foo":   d{"booze": []interface{}{}},
		}, "Sorry, no booze."},
		{d{
			"goose": []interface{}{},
			"foo":   d{"booze": []interface{}{d{"name": "boo"}}},
		}, "->\n0: Scary drink boo!"},
		{d{
			"goose": []interface{}{},
			"foo":   d{"booze": []interface{}{d{"name": "a"}, d{"name": "b"}}},
		}, "->\n0: Scary drink a!\n1: Scary drink b!"},
		{d{
			"goose": []interface{}{d{"numKids": 1}, d{"numKids": 2}},
			"foo":   d{"booze": []interface{}{}},
		}, "1 goslings.\n2 goslings.\nSorry, no booze."},
	}, []errortest{
		{nil},                           // non-null-safe eval of $foo.booze fails
		{d{"foo": nil}},                 // ditto
		{d{"foo": d{}}},                 // $foo.booze must be a list
		{d{"foo": d{"booze": "str"}}},   // $foo.booze must be list
		{d{"foo": d{"booze": 5}}},       // $foo.booze must be list
		{d{"foo": d{"booze": d{}}}},     // $foo.booze must be list
		{d{"foo": d{"booze": true}}},    // $foo.booze must be list
		{d{"foo": d{"booze": []d{{}}}}}, // $boo.name fails
	}))
}

func TestFor(t *testing.T) {
	runExecTests(t, multidatatest("for", `
{for $i in range(1, length($items) + 1)}
  {msg desc="Numbered item."}
    {$i}: {$items[$i - 1]}{\n}
  {/msg}
{/for}`, []datatest{
		{d{"items": []interface{}{}}, ""},
		{d{"items": []interface{}{"car"}}, "1: car\n"},
		{d{"items": []interface{}{"car", "boat"}}, "1: car\n2: boat\n"},
	}, []errortest{
		{d{}},             // undefined is not a valid slice
		{d{"items": nil}}, // null is not a valid slice
		// DIFFERENCE: JS function iterates through the string "a" instead of throwing an error.
		// {d{"items": "a"}}, // string is not a valid slice
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
		{d{
			"boo": 0,
		}, "A"},
		{d{
			"boo": 1,
			"foo": d{"goo": 1},
		}, "B"},
		{d{
			"boo": -1,
			"foo": d{"goo": 5},
		}, "C"},
		{d{
			"boo": 1,
			"foo": d{"goo": 5},
		}, "C"},
		{d{
			"boo": 2,
			"foo": d{"goo": 5},
		}, "D"},
		{d{
			"boo": 2,
			"foo": d{"goo": 5},
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
			d{"animals": d{"animal": "roos"}, "too": 2.4},
			true,
		},
	})
}

func TestDataRefs(t *testing.T) {
	runExecTests(t, []execTest{
		// single key
		exprtestwdata("undefined", "{$foo}", "", nil).fails(),     // undefined = error
		exprtestwdata("null", "{$foo}", "null", d{"foo": nil}),    // null prints
		exprtestwdata("string", "{$foo}", "foo", d{"foo": "foo"}), // string print
		// DIFFERENCE: JS prints lists as "a,5,2.5", without brackets
		// exprtestwdata("list", "{$foo}", "[a, 5, [2.5], [null]]",
		// 	d{"foo": []interface{}{"a", 5, []interface{}{2.5}, []interface{}{nil}}}), // list print
		// DIFFERENCE JS prints maps as [Object object]
		// exprtestwdata("map", "{$foo}", "{a: 5, b: [true], c: {}}",
		// 	d{"foo": d{"a": 5, "b": []interface{}{true}, "c": d{}}}), // map print

		// index lookups
		exprtestwdata("basic", "{$foo.2}", "result",
			d{"foo": []interface{}{"a", 5, "result"}}),
		// DIFFERENCE: JS just returns undefined
		// exprtestwdata("out of bounds", "{$foo.7}", "",
		// 	d{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("undefined slice", "{$foo.2}", "", d{}).fails(),
		exprtestwdata("null slice", "{$foo.2}", "", d{"foo": nil}).fails(),
		exprtestwdata("nullsafe on undefined slice", "{$foo?.2}", "null", d{}),
		exprtestwdata("nullsafe on null slice", "{$foo?.2}", "null", d{"foo": nil}),
		// DIFFERENCE: JS does not throw error on out-of-bounds
		// exprtestwdata("nullsafe does not save out of bounds", "{$foo?.2}", "",
		// 	d{"foo": []interface{}{"a"}}).fails(),
		// DIFFERENCE: JS allows lookups on everything.
		// exprtestwdata("lookup on nonslice", "{$foo?.2}", "", d{"foo": "hello"}).fails(),

		// key lookups
		exprtestwdata("basic", "{$foo.bar}", "result", d{"foo": d{"bar": "result"}}),
		exprtestwdata("undefined map", "{$foo.bar}", "", d{}).fails(),
		exprtestwdata("null map", "{$foo.bar}", "", d{"foo": nil}).fails(),
		exprtestwdata("null value is ok", "{$foo.bar}", "null", d{"foo": d{"bar": nil}}),
		exprtestwdata("nullsafe on undefined map", "{$foo?.bar}", "null", d{}),
		exprtestwdata("nullsafe on null map", "{$foo?.bar}", "null", d{"foo": nil}),
		// DIFFERENCE: In JS everything's a map.
		// exprtestwdata("lookup on nonmap", "{$foo?.bar}", "", d{"foo": "hello"}).fails(),

		// expr lookups (index)
		exprtestwdata("exprbasic", "{$foo[2]}", "result", d{"foo": []interface{}{"a", 5, "result"}}),
		// DIFFERENCE: No error
		// exprtestwdata("exprout of bounds", "{$foo[7]}", "", d{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("exprundefined slice", "{$foo[2]}", "", d{}).fails(),
		exprtestwdata("exprnull slice", "{$foo[2]}", "", d{"foo": nil}).fails(),
		exprtestwdata("exprnullsafe on undefined slice", "{$foo?[2]}", "null", d{}),
		exprtestwdata("exprnullsafe on null slice", "{$foo?[2]}", "null", d{"foo": nil}),
		// DIFFERENCE: No error
		// exprtestwdata("exprnullsafe does not save out of bounds", "{$foo?[2]}", "",
		// 	d{"foo": []interface{}{"a"}}).fails(),
		exprtestwdata("exprarith", "{$foo[1+1>3.0?8:1]}", "5", d{"foo": []interface{}{"a", 5, "z"}}),

		// expr lookups (key)
		exprtestwdata("exprkeybasic", "{$foo['bar']}", "result", d{"foo": d{"bar": "result"}}),
		exprtestwdata("exprkeyundefined map", "{$foo['bar']}", "", d{}).fails(),
		exprtestwdata("exprkeynull map", "{$foo['bar']}", "", d{"foo": nil}).fails(),
		exprtestwdata("exprkeynull value is ok", "{$foo['bar']}", "null", d{"foo": d{"bar": nil}}),
		exprtestwdata("exprkeynullsafe on undefined map", "{$foo?['bar']}", "null", d{}),
		exprtestwdata("exprkeynullsafe on null map", "{$foo?['bar']}", "null", d{"foo": nil}),
		exprtestwdata("exprkeyarith", "{$foo['b'+('a'+'r')]}", "result", d{"foo": d{"bar": "result"}}),

		// nullsafe + elvis
		exprtestwdata("elvis and nullsafe key", "{$foo?.bar ?: 'hello'}", "hello", d{}),
		exprtestwdata("elvis and nullsafe key success", "{$foo?.bar ?: 'hello'}", "hi", d{"foo": d{"bar": "hi"}}),
		exprtestwdata("elvis and nullsafe key half success", "{$foo?.bar ?: 'hello'}", "hello", d{"foo": d{}}),
		exprtestwdata("elvis and nullsafe index ", "{$foo?[0] ?: 'hello'}", "hello", d{}),
		exprtestwdata("elvis and nullsafe index success", "{$foo?[0] ?: 'hello'}", "a", d{"foo": []interface{}{"a", "b", "c", "d"}}),
		exprtestwdata("elvis and nullsafe index half success", "{$foo?[0] ?: 'hello'}", "hello", d{"foo": []interface{}{}}),
		exprtestwdata("elvis and nullsafe expr", "{$foo?['bar'] ?: 'hello'}", "hello", d{}),
		exprtestwdata("elvis and nullsafe expr success", "{$foo?['bar'] ?: 'hello'}", "hi", d{"foo": d{"bar": "hi"}}),
		exprtestwdata("elvis and nullsafe expr half success", "{$foo?['bar'] ?: 'hello'}", "hello", d{"foo": d{}}),
		exprtestwdata("elvis and nullsafe chain", "{$foo?.bar?.baz ?: 'hello'}", "hello", d{"foo": d{"bar": d{}}}),

		// nullsafe + binary op
		exprtestwdata("elvis and nullsafe key add", "{let $x: $foo + $bar?.baz /}{$x}", "5", d{"foo": 2, "bar": d{"baz": 3}}), // incorrect would give null
		exprtestwdata("elvis and nullsafe key add rev", "{let $x: $bar?.baz + $foo /}{$x}", "5", d{"foo": 2, "bar": d{"baz": 3}}),
		exprtestwdata("elvis and nullsafe key or", "{let $x: $foo or $bar?.baz /}{$x}", "hello", d{"foo": "hello", "bar": d{}}), // incorrect would give null
		exprtestwdata("elvis and nullsafe key or rev", "{let $x: $bar?.baz or $foo /}{$x}", "hello", d{"foo": "hello", "bar": d{}}),
		exprtestwdata("elvis and nullsafe key comp", "{let $x: $foo == $bar?.baz /}{$x}", "false", d{"foo": "hello", "bar": d{}}), // incorrect would give undefined
		exprtestwdata("elvis and nullsafe key comp rev", "{let $x: $bar?.baz == $foo /}{$x}", "false", d{"foo": "hello", "bar": d{}}),

		// DIFFERENCE: More tests on nullsafe navigation.
		exprtestwdata("nullsafe battle royale",
			"{$foo[2].bar?.baz?['bar']?[3].boo[3]}", "null", d{
				"foo": []interface{}{
					d{},
					d{},
					d{}, // foo[2].bar is null
				}}),
		exprtestwdata("nullsafe battle royale2",
			"{$foo[2].bar?.baz?['bar']?[3].boo[3]}", "null", d{
				"foo": []interface{}{
					d{},
					d{},
					d{"bar": d{}}, // foo[2].bar.baz is null
				}}),
		exprtestwdata("nullsafe battle royale3",
			"{$foo[2].bar?.baz?['bar']?[3].boo[3]}", "null", d{
				"foo": []interface{}{
					d{},
					d{},
					d{"bar": d{"baz": d{}}}, // foo[2].bar.baz['bar'] is null
				}}),
		exprtestwdata("nullsafe battle royale4",
			"{$foo[2].bar?.baz?['bar']?[3].boo[3]}", "d", d{
				"foo": []interface{}{
					d{},
					d{},
					d{"bar": d{"baz": d{"bar": []interface{}{
						d{},
						d{},
						d{},
						// foo[2].bar.baz['bar'][3].boo[3] is NOT null
						d{"boo": []interface{}{"a", "b", "c", "d"}}}}}},
				}}),
	})
}

func TestSpecialChars(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("special chars", `{sp}{nil}{\r}{\n}{\t}{lb}{rb}`, " \r\n\t{}"),
		exprtest("without nil there is space", "abc\ndef", "abc def"),
		exprtest("nil avoids space", "abc{nil}\ndef", "abcdef"),
		exprtest("without sp there is no space", "abc\n<a>", "abc<a>"),
		exprtest("sp adds space", "abc{sp}\n<a>", "abc <a>"),
	})
}

func TestForeachElvis(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("foreachelvislet", `{template .foo}
			{let $list: null ?: [] /}
			{foreach $l in $list}{/foreach}
			{/template}`, ""),
		exprtest("foreachelvisinline", `{template .foo}
			{foreach $l in null ?: []}{/foreach}
			{/template}`, ""),
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
			d{"component": "page"}),
	})
}

/** TestLog */
/** TestDebugger */

func TestPrintDirectives(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("sanitized html", "{'<a>'}", "&lt;a&gt;"),
		exprtest("noAutoescape", "{'<a>'|noAutoescape}", "<a>"),
		exprtestwdata("sanitized var", "{$var}", "&lt;a&gt;", d{"var": "<a>"}),
		exprtestwdata("noAutoescape var", "{$var |noAutoescape}", "<a>", d{"var": "<a>"}),

		// |id == |noAutoescape (it's deprecated)
		exprtest("id", "{'<a>'|id}", "<a>"),

		exprtest("escapeHtml", "{'<a>'|escapeHtml}", "&lt;a&gt;"),

		exprtest("escapeUri1", "{''|escapeUri}", ""),
		// DIFFERENCE: Go results in +, JS in %20
		exprtest("escapeUri2", "{'a%b > c'|escapeUri}", "a%25b%20%3E%20c"),
		// TODO: test it escapes kind=HTML content
		// TODO: test it does not escape kind=URI content

		exprtestwdata("ejs1", "{$var|escapeJsString}", ``, d{"var": ""}),
		exprtestwdata("ejs2", "{$var|escapeJsString}", `foo`, d{"var": "foo"}),
		exprtestwdata("ejs3", "{$var|escapeJsString}", `foo\\bar`, d{"var": "foo\\bar"}),
		// TODO: test it even escapes "kind=HTML" content
		// TODO: test it does not escape "kind=JS_STR" content
		exprtestwdata("ejs4", "{$var|escapeJsString}", `\\`, d{"var": "\\"}),
		// DIFFERENCE: Go results in \'\', JS in \x27\x27
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\x27\x27`, d{"var": "''"}),
		// DIFFERENCE: Go results in \"\", JS in \x22\x22
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\x22foo\x22`, d{"var": `"foo"`}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `42`, d{"var": 42}),

		exprtest("truncate", "{'Lorem Ipsum' |truncate:8}", "Lorem..."),
		exprtest("truncate w arg", "{'Lorem Ipsum' |truncate:8,false}", "Lorem Ip"),
		exprtest("truncate w expr", "{'Lorem Ipsum' |truncate:5+3,not true}", "Lorem Ip"),

		exprtest("insertWordBreaks", "{'1234567890'|insertWordBreaks:3}", "123<wbr>456<wbr>789<wbr>0"),
		exprtest("insertWordBreaks2", "{'123456789'|insertWordBreaks:3}", "123<wbr>456<wbr>789"),
		exprtest("insertWordBreaks3", "{'123456789'|insertWordBreaks:30}", "123456789"),
		exprtest("insertWordBreaks4", "{'12 345 6789'|insertWordBreaks:3}", "12 345 678<wbr>9"),
		exprtest("insertWordBreaks5", "{''|insertWordBreaks:3}", ""),

		exprtestwdata("nl2br", "{$var|changeNewlineToBr}", "<br>1<br>2<br>3<br><br>4<br><br>",
			d{"var": "\r1\n2\r3\r\n\n4\n\n"}),
	})
}

func TestGlobals(t *testing.T) {
	globals["app.global_str"] = data.New("abc")
	globals["GLOBAL_INT"] = data.New(5)
	globals["global.nil"] = data.New(nil)
	runExecTests(t, []execTest{
		exprtest("global", `{app.global_str} {GLOBAL_INT + 2} {global.nil?:'hi'}`, `abc 7 hi`),
	})
}

func TestInjectedData(t *testing.T) {
	ij["foo"] = data.String("abc")
	runExecTests(t, []execTest{
		exprtest("ij", `{$ij.foo}`, `abc`),
	})
}

func TestAutoescapeModes(t *testing.T) {
	runExecTests(t, []execTest{
		{"template autoescape=false", "test.autoescapeoff", `{namespace test}

{template .autoescapeoff autoescape="false"}
  {$foo} {$foo|escapeHtml}
{/template}`,
			"<b>hello</b> &lt;b&gt;hello&lt;/b&gt;",
			d{"foo": "<b>hello</b>"},
			true,
		},

		{"autoescape mode w/ call", "test.autowcall", `{namespace test}

{template .autowcall}
  {call .called data="all"/}
{/template}

{template .called autoescape="false"}
  {$foo}
{/template}
`,
			"<b>hello</b>",
			d{"foo": "<b>hello</b>"},
			true,
		},

		{"autoescape mode w/ call2", "test.autowcall", `{namespace test}

{template .autowcall autoescape="false"}
  {call .called data="all"/}
{/template}

{template .called}
  {$foo}
{/template}
`,
			"&lt;b&gt;hello&lt;/b&gt;",
			d{"foo": "<b>hello</b>"},
			true,
		},

		{"namespace sets default", "test.name", `
{namespace test autoescape="false"}

{template .name}
  {$foo}
{/template}`,
			"<b>hello</b>",
			d{"foo": "<b>hello</b>"},
			true,
		},

		{"template overrides namespace", "test.name", `
{namespace test autoescape="false"}

{template .name autoescape="true"}
  {$foo}
{/template}`,
			"&lt;b&gt;hello&lt;/b&gt;",
			d{"foo": "<b>hello</b>"},
			true,
		},
	})
}

var helloWorldTemplate = `
{namespace examples.simple}

/**
 * Says hello to the world.
 */
{template .helloWorld}
  Hello world!
{/template}

/**
 * Greets a person using "Hello" by default.
 * @param name The name of the person.
 * @param? greetingWord Optional greeting word to use instead of "Hello".
 */
{template .helloName}
  {if not $greetingWord}
    Hello {$name}!
  {else}
    {$greetingWord} {$name}!
  {/if}
{/template}

/**
 * Greets a person and optionally a list of other people.
 * @param name The name of the person.
 * @param additionalNames The additional names to greet. May be an empty list.
 */
{template .helloNames}
  // Greet the person.
  {call .helloName data="all" /}<br>
  // Greet the additional people.
  {foreach $additionalName in $additionalNames}
    {call .helloName}
      {param name: $additionalName /}
    {/call}
    {if not isLast($additionalName)}
      <br>  // break after every line except the last
    {/if}
  {ifempty}
    No additional people to greet.
  {/foreach}
{/template}
`

// TestHelloWorld executes the Hello World tutorial on the Soy Templates site.
func TestHelloWorld(t *testing.T) {
	runExecTests(t, []execTest{
		{"no data", "examples.simple.helloWorld", helloWorldTemplate,
			"Hello world!",
			d{},
			true,
		},

		{"1 name", "examples.simple.helloName", helloWorldTemplate,
			"Hello Ana!",
			d{"name": "Ana"},
			true,
		},

		{"additional names", "examples.simple.helloNames", helloWorldTemplate,
			"Hello Ana!<br>Hello Bob!<br>Hello Cid!<br>Hello Dee!",
			d{"name": "Ana", "additionalNames": []string{"Bob", "Cid", "Dee"}},
			true,
		},
	})
}

var identicalParamNameTemplate = `
{namespace test}

/**
 * A wrapper template to call .helloNameIdentical.
 * Uses a let variable with same name as param passed to .helloNameIdentical.
 * @param param
 */
{template .helloWrapperIdentical}
	{let $name: $param ?: 'world' /}
	{call .helloNameIdentical data="all"}
		{param name: $name /}
	{/call}
{/template}

/**
 * @param name
 */
{template .helloNameIdentical}
  Hello {$name}!
{/template}
`

// TestIdenticalParamName checks that proper JS compilation when using params and vars of the same name within templates.
func TestIdenticalParamName(t *testing.T) {
	runExecTests(t, []execTest{
		{"normal wrapper call without param", "test.helloWrapperIdentical", identicalParamNameTemplate,
			"Hello world!",
			d{},
			true,
		},

		{"normal wrapper call with param", "test.helloWrapperIdentical", identicalParamNameTemplate,
			"Hello Ana!",
			d{"param": "Ana"},
			true,
		},

		{"normal call with param", "test.helloNameIdentical", identicalParamNameTemplate,
			"Hello Ana!",
			d{"name": "Ana"},
			true,
		},
	})
}

/** TestStructData */

func TestLet(t *testing.T) {
	runExecTests(t, []execTest{
		exprtestwdata("let", `
{let $alpha: $boo.foo /}
{let $beta}Boo!{/let}
{let $gamma}
  {for $i in range($alpha)}
    {$i}{$beta}
  {/for}
{/let}
{$gamma}`,
			"0Boo!1Boo!2Boo!",
			d{"boo": d{"foo": 3}}),
	})
}

// Tests that a map with string keys with spaces is escaped correctly
func TestLetMap(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("let", `{let $foo:['FooBar': 1234] /}{$foo['FooBar']}`, "1234"),
		exprtest("let", `{let $foo:['Foo Bar': 1234] /}{$foo['Foo Bar']}`, "1234"),
	})
}

// testing cross namespace stuff requires multiple file bodies
type nsExecTest struct {
	name         string
	templateName string
	input        []string
	output       string
	data         interface{}
	ok           bool
	msgs         *fakeBundle
}

func TestAlias(t *testing.T) {
	runNsExecTests(t, []nsExecTest{
		{"alias", "test.alias",
			[]string{`
{namespace test}
{alias foo.bar.baz}

{template .alias}
{call baz.hello/}
{/template}
`, `
{namespace foo.bar.baz}

{template .hello}
Hello world
{/template}`},
			"Hello world", nil, true, nil},
	})
}

// Helpers

var globals = make(data.Map)
var ij = make(data.Map)

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

/** END COPY PASTA */

type fakeBundle struct {
	msgs   map[uint64]*soymsg.Message
	locale string
}

func (fb *fakeBundle) Message(id uint64) *soymsg.Message {
	if fb == nil || fb.msgs == nil {
		return nil
	}
	return fb.msgs[id]
}

func (fb *fakeBundle) Locale() string {
	return fb.locale
}

func (fb *fakeBundle) PluralCase(n int) int {
	return -1
}

func pluralEnglish(n int) int {
	if n == 1 {
		return 0
	}
	return 1
}

func pluralCzech(n int) int {
	switch {
	case n == 1:
		return 0
	case n >= 2 && n <= 4:
		return 1
	default:
		return 2
	}
}

func newFakeBundle(msg, tran, locale string) *fakeBundle {
	var sf, err = parse.SoyFile("", `{msg desc=""}`+msg+`{/msg}`)
	if err != nil {
		panic(err)
	}
	var msgnode = sf.Body[0].(*ast.MsgNode)
	soymsg.SetPlaceholdersAndID(msgnode)
	var m = soymsg.NewMessage(msgnode.ID, tran)
	return &fakeBundle{map[uint64]*soymsg.Message{msgnode.ID: m}, locale}
}

func newFakePluralBundle(pluralVar, msg1, msg2, locale string, msgstr []string) *fakeBundle {
	var sf, err = parse.SoyFile("", `{msg desc=""}
{plural `+pluralVar+`}
  {case 1}`+msg1+`
  {default}`+msg2+`
{/plural}
{/msg}`)
	if err != nil {
		panic(err)
	}
	var msgnode = sf.Body[0].(*ast.MsgNode)
	soymsg.SetPlaceholdersAndID(msgnode)
	var msg = newMessage(msgnode, msgstr)
	return &fakeBundle{map[uint64]*soymsg.Message{msgnode.ID: &msg}, locale}
}

func newMessage(node *ast.MsgNode, msgstrs []string) soymsg.Message {
	var cases []soymsg.PluralCase
	for _, msgstr := range msgstrs {
		// TODO: Ideally this would convert from PO plural form to CLDR plural class.
		// Instead, just use PluralCase() to select one of these.
		cases = append(cases, soymsg.PluralCase{
			Spec:  soymsg.PluralSpec{soymsg.PluralSpecOther, -1}, // not used
			Parts: soymsg.Parts(msgstr),
		})
	}
	return soymsg.Message{node.ID, []soymsg.Part{soymsg.PluralPart{
		VarName: node.Body.Children()[0].(*ast.MsgPluralNode).VarName,
		Cases:   cases,
	}}}
}

func TestMessages(t *testing.T) {
	runNsExecTests(t, []nsExecTest{
		{"no bundle", "test.main", []string{`{namespace test}
{template .main}
  {msg desc=""}
    Hello world
  {/msg}
{/template}`}, "Hello world", nil, true, nil},

		{"bundle lacks", "test.main", []string{`{namespace test}
{template .main}
  {msg desc=""}
    Hello world
  {/msg}
{/template}`}, "Hello world", nil, true, newFakeBundle("foo", "bar", "")},

		{"bundle has", "test.main", []string{`{namespace test}
{template .main}
  {msg desc=""}
    Hello world
  {/msg}
{/template}`}, "Sup", nil, true, newFakeBundle("Hello world", "Sup", "")},

		{"msg with variable & translation", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    a: {$a}
  {/msg}
{/template}`}, "a is 1", d{"a": 1}, true, newFakeBundle("a: {$a}", "a is {A}", "")},

		{"msg w variables", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    {$a}{$a} xx {$a}{sp}
  {/msg}
{/template}`}, "11xxx1", d{"a": 1}, true, newFakeBundle("{$a}{$a} xx {$a}{sp}", "{A}{A}xxx{A}", "")},

		{"msg w numbered placeholders", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    {$a.a}{$a.b.a}
  {/msg}
{/template}`}, "21", d{"a": d{"a": 1, "b": d{"a": 2}}},
			true, newFakeBundle("{$a.a}{$a.b.a}", "{A_2}{A_1}", "")},

		{"msg w html", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    Click <a>here</a>
  {/msg}
{/template}`}, "<a>Click here</a>", nil,
			true, newFakeBundle("Click <a>here</a>", "{START_LINK}Click here{END_LINK}", "")},

		{"plural, not found, singular", "test.main", []string{`{namespace test}
/** @param n */
{template .main}
  {msg desc=""}
    {plural $n}
    {case 1}
      one user
    {default}
      {$n} users
    {/plural}
  {/msg}
{/template}`}, "one user", d{"n": 1},
			true, nil},

		{"plural, not found, plural", "test.main", []string{`{namespace test}
/** @param n */
{template .main}
  {msg desc=""}
    {plural $n}
    {case 1}
      one user
    {default}
      {$n} users
    {/plural}
  {/msg}
{/template}`}, "11 users", d{"n": 11},
			true, nil},

		{"plural, singular", "test.main", []string{`{namespace test}
/** @param n */
{template .main}
  {msg desc=""}
    {plural $n}
    {case 1}
      one user
    {default}
      {$n} users
    {/plural}
  {/msg}
{/template}`}, "|one user|", d{"n": 1}, true,
			newFakePluralBundle("$n", "one user", "{$n} users",
				"en", []string{"|one user|", "|({N_2}) users|"})},

		{"plural, plural", "test.main", []string{`{namespace test}
/** @param n */
{template .main}
  {msg desc=""}
    {plural $n}
    {case 1}
      one user
    {default}
      {$n} users
    {/plural}
  {/msg}
{/template}`}, "|(10) users|", d{"n": 10}, true,
			newFakePluralBundle("$n", "one user", "{$n} users",
				"en", []string{"|one user|", "|({N_2}) users|"})},

		{"plural, few, czech", "test.main", []string{`{namespace test}
/** @param n */
{template .main}
  {msg desc=""}
    {plural $n}
    {case 1}
      one user
    {default}
      {$n} users
    {/plural}
  {/msg}
{/template}`}, "|few (3) users|", d{"n": 3}, true,
			newFakePluralBundle("$n", "one user", "{$n} users",
				"cs", []string{"|one user|", "|few ({N_2}) users|", "|({N_2}) users|"})},
	})
}

func TestLog(t *testing.T) {
	var otto = otto.New()
	_, err := otto.Run(`
var console_output = '';
var console = {};
console.log = function(arg) { console_output += arg; };
var soy = {};
soy.$$escapeHtml = function(arg) { return arg; };
`)
	if err != nil {
		t.Error(err)
		return
	}
	soyfile, err := parse.SoyFile("", `
{namespace test}
{template .log}
{log}Hello {$name}.{/log}
{/template}`)
	if err != nil {
		t.Error(err)
		return
	}
	var buf bytes.Buffer
	err = Write(&buf, soyfile, Options{})
	if err != nil {
		t.Error(err)
		return
	}
	_, err = otto.Run(buf.String())
	if err != nil {
		t.Error(err)
		return
	}
	_, err = otto.Run(`test.log({name: "Rob"});`)
	if err != nil {
		t.Error(err)
		return
	}
	val, _ := otto.Get("console_output")
	if val.String() != "Hello Rob." {
		t.Errorf("got %q", val.String())
	}
}

func TestES6(t *testing.T) {
	bundle := soy.NewBundle()
	bundle.AddTemplateString("test_formatter.soy", `{namespace test}
{template .formatter}
	{call say.hello /}
{/template}`)
	bundle.AddTemplateString("say_hello.soy", `{namespace say}
{template .hello}
	Hello World!
{/template}`)
	registry, err := bundle.Compile()
	if err != nil {
		t.Error(err)
		return
	}
	expected := []string{
		`import { say__hello } from 'say.hello.js';

// This file was automatically generated from test_formatter.soy.
// Please don't edit this file by hand.

if (typeof test == 'undefined') { var test = {}; }

export function test__formatter(opt_data, opt_sb, opt_ijData) {
  var output = '';
  output += say__hello({}, opt_sb, opt_ijData);
  return output;
};`,
		`// This file was automatically generated from say_hello.soy.
// Please don't edit this file by hand.

if (typeof say == 'undefined') { var say = {}; }

export function say__hello(opt_data, opt_sb, opt_ijData) {
  var output = '';
  output += 'Hello World!';
  return output;
};`,
	}
	for i, soyfile := range registry.SoyFiles {
		var buf bytes.Buffer
		err := Write(&buf, soyfile, Options{Formatter: ES6Formatter{}})
		if err != nil {
			t.Error(err)
			return
		}
		if a, e := strings.TrimSpace(buf.String()), strings.TrimSpace(expected[i]); a != e {
			t.Errorf("ES6 Error, did not get expected results:\n%v", diff.LineDiff(e, a))
		}
	}
}

var pluralFuncBodies = map[string]string{
	"en": `
	if (n > 1) {
		return 1;
	}
	return 0;`,

	"cs": `
	if (n == 1) {
		return 0;
	} else if (n >= 2 && n <= 4) {
		return 1;
	} else {
		return 2;
	}`,
}

func runExecTests(t *testing.T, tests []execTest) {
	var nstest []nsExecTest
	for _, test := range tests {
		nstest = append(nstest, nsExecTest{
			test.name,
			test.templateName,
			[]string{test.input},
			test.output,
			test.data,
			test.ok,
			nil,
		})
	}
	runNsExecTests(t, nstest)
}

var pluralFuncTmpl = `
	var soy = soy || {};
	soy.$$pluralIndex = function(n){
		%s
	};
`

func runNsExecTests(t *testing.T, tests []nsExecTest) {
	var js = initJs(t)

TESTS_LOOP:
	for _, test := range tests {
		var js = js.Copy()

		// Parse the templates, generate and run the compiled javascript.
		var source bytes.Buffer
		for _, input := range test.input {
			if test.msgs != nil && test.msgs.locale != "" {
				fbody := pluralFuncBodies[test.msgs.locale]
				js.Run(fmt.Sprintf(pluralFuncTmpl, fbody))
			}

			var registry = template.Registry{}
			soyfile, err := parse.SoyFile(test.name, input)
			if err != nil {
				t.Errorf("%s: soyfile parse error: %v", test.name, err)
				continue TESTS_LOOP
			}
			if err := registry.Add(soyfile); err != nil {
				if test.ok {
					t.Errorf("%s: registry add error: %v", test.name, err)
				}
				continue TESTS_LOOP
			}
			parsepasses.SetGlobals(registry, globals)
			parsepasses.ProcessMessages(registry)

			var buf bytes.Buffer
			// TODO: Should loop over SoyFiles and add to buffer
			err = Write(&buf, registry.SoyFiles[0], Options{Messages: test.msgs})
			if err != nil {
				t.Errorf("%s: write error: %v", test.name, err)
				continue TESTS_LOOP
			}

			_, err = js.Run(buf.String())
			if err != nil {
				if test.ok {
					t.Errorf("%s: compile error: %v\n%v", test.name, err, numberLines(&buf))
				}
				continue TESTS_LOOP
			}
			source.Write(buf.Bytes())
		}

		// Convert test data to JSON and invoke the template.
		var jsonData, _ = json.Marshal(test.data)
		var ijJson, _ = json.Marshal(ij)
		var renderStatement = fmt.Sprintf("%s(JSON.parse(%q), undefined, JSON.parse(%q));",
			test.templateName, string(jsonData), string(ijJson))
		switch actual, err := js.Run(renderStatement); {
		case err != nil && test.ok:
			t.Errorf("render error (%s): %v\n%v\n%v", test.name, err, numberLines(&source), renderStatement)
		case err == nil && !test.ok:
			t.Errorf("expected error, got none (%s):\n%v\n%v", test.name, numberLines(&source), renderStatement)
		case test.ok && test.output != actual.String():
			t.Errorf("expected (%s):\n%v\n\nactual:\n%v\n%v\n%v",
				test.name, test.output, actual.String(), numberLines(&source), renderStatement)
		}
	}
}

func initJs(t *testing.T) *otto.Otto {
	var otto = otto.New()
	soyutilsFile, err := os.Open("lib/soyutils.js")
	if err != nil {
		panic(err)
	}
	// remove any non-otto compatible regular expressions
	var soyutilsBuf bytes.Buffer
	var scanner = bufio.NewScanner(soyutilsFile)
	var i = 1
	for scanner.Scan() {
		switch i {
		case 2565, 2579, 2586:
			// skip these regexes
			// soy.esc.$$FILTER_FOR_FILTER_CSS_VALUE_
			// soy.esc.$$FILTER_FOR_FILTER_HTML_ATTRIBUTES_
			// soy.esc.$$FILTER_FOR_FILTER_HTML_ELEMENT_NAME_
		default:
			soyutilsBuf.Write(scanner.Bytes())
			soyutilsBuf.Write([]byte("\n"))
		}
		i++
	}
	// load the soyutils library
	_, err = otto.Run(soyutilsBuf.String())
	if err != nil {
		t.Errorf("soyutils error: %v", err)
		panic(err)
	}
	return otto
}

func numberLines(soyfile io.Reader) string {
	var buf bytes.Buffer
	var scanner = bufio.NewScanner(soyfile)
	var i = 1
	for scanner.Scan() {
		buf.WriteString(fmt.Sprintf("%03d ", i))
		buf.Write(scanner.Bytes())
		buf.WriteString("\n")
		i++
	}
	return buf.String()
}
