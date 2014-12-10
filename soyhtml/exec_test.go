package soyhtml

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/robfig/gettext/po"
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/parsepasses"
	"github.com/robfig/soy/soymsg"
	"github.com/robfig/soy/template"
)

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

func TestExpressions(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("arithmetic", "{2*(1+1)/(2%4)}", "2"),
		exprtest("arithmetic2", "{2.0-1.5}", "0.5"),
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
		exprtest("negate float", `{-(1+1.5)}`, "-2.5"),

		// short-circuiting
		exprtest("shortcircuit precondition undef key fails", "{$undef.key}", "").fails(),
		exprtest("shortcircuit and", "{$undef and $undef.key}", "false"),
		exprtest("shortcircuit or", "{'yay' or $undef.key}", "true"),
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
		{d{"foo": nil}},             // $foo.goo fails
		{d{"foo": "str"}},           // $foo.goo must be number
		{d{"foo": true}},            // $foo.goo must be number
		{d{"foo": d{}}},             // $foo.goo must be number
		{d{"foo": []interface{}{}}}, // $foo.goo must be number
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

	runExecTests(t, multidatatest("foreachkeys", `
{foreach $var in keys($map)}
  {$var}
{/foreach}`, []datatest{
		{d{"map": d{"a": nil}}, "a"},
	}, nil))
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
		{d{"items": "a"}}, // string is not a valid slice
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
		exprtestwdata("list", "{$foo}", "[a, 5, [2.5], [null]]",
			d{"foo": []interface{}{"a", 5, []interface{}{2.5}, []interface{}{nil}}}), // list print
		exprtestwdata("map", "{$foo}", "{a: 5, b: [true], c: {}}",
			d{"foo": d{"a": 5, "b": []interface{}{true}, "c": d{}}}), // map print

		// index lookups
		exprtestwdata("basic", "{$foo.2}", "result",
			d{"foo": []interface{}{"a", 5, "result"}}),
		exprtestwdata("out of bounds", "{$foo.7}", "",
			d{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("undefined slice", "{$foo.2}", "", d{}).fails(),
		exprtestwdata("null slice", "{$foo.2}", "", d{"foo": nil}).fails(),
		exprtestwdata("nullsafe on undefined slice", "{$foo?.2}", "null", d{}),
		exprtestwdata("nullsafe on null slice", "{$foo?.2}", "null", d{"foo": nil}),
		exprtestwdata("nullsafe does not save out of bounds", "{$foo?.2}", "",
			d{"foo": []interface{}{"a"}}).fails(),
		exprtestwdata("lookup on nonslice", "{$foo?.2}", "", d{"foo": "hello"}).fails(),

		// key lookups
		exprtestwdata("basic", "{$foo.bar}", "result", d{"foo": d{"bar": "result"}}),
		exprtestwdata("undefined map", "{$foo.bar}", "", d{}).fails(),
		exprtestwdata("null map", "{$foo.bar}", "", d{"foo": nil}).fails(),
		exprtestwdata("null value is ok", "{$foo.bar}", "null", d{"foo": d{"bar": nil}}),
		exprtestwdata("nullsafe on undefined map", "{$foo?.bar}", "null", d{}),
		exprtestwdata("nullsafe on null map", "{$foo?.bar}", "null", d{"foo": nil}),
		exprtestwdata("lookup on nonmap", "{$foo?.bar}", "", d{"foo": "hello"}).fails(),

		// expr lookups (index)
		exprtestwdata("exprbasic", "{$foo[2]}", "result", d{"foo": []interface{}{"a", 5, "result"}}),
		exprtestwdata("exprout of bounds", "{$foo[7]}", "", d{"foo": []interface{}{"a", 5, "result"}}).fails(),
		exprtestwdata("exprundefined slice", "{$foo[2]}", "", d{}).fails(),
		exprtestwdata("exprnull slice", "{$foo[2]}", "", d{"foo": nil}).fails(),
		exprtestwdata("exprnullsafe on undefined slice", "{$foo?[2]}", "null", d{}),
		exprtestwdata("exprnullsafe on null slice", "{$foo?[2]}", "null", d{"foo": nil}),
		exprtestwdata("exprnullsafe does not save out of bounds", "{$foo?[2]}", "",
			d{"foo": []interface{}{"a"}}).fails(),
		exprtestwdata("exprarith", "{$foo[1+1>3.0?8:1]}", "5", d{"foo": []interface{}{"a", 5, "z"}}),

		// expr lookups (key)
		exprtestwdata("exprkeybasic", "{$foo['bar']}", "result", d{"foo": d{"bar": "result"}}),
		exprtestwdata("exprkeyundefined map", "{$foo['bar']}", "", d{}).fails(),
		exprtestwdata("exprkeynull map", "{$foo['bar']}", "", d{"foo": nil}).fails(),
		exprtestwdata("exprkeynull value is ok", "{$foo['bar']}", "null", d{"foo": d{"bar": nil}}),
		exprtestwdata("exprkeynullsafe on undefined map", "{$foo?['bar']}", "null", d{}),
		exprtestwdata("exprkeynullsafe on null map", "{$foo?['bar']}", "null", d{"foo": nil}),
		exprtestwdata("exprkeyarith", "{$foo['b'+('a'+'r')]}", "result", d{"foo": d{"bar": "result"}}),
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

func TestLog(t *testing.T) {
	originalLogger := Logger
	defer func() { Logger = originalLogger }()

	var buf bytes.Buffer
	Logger = log.New(&buf, "", 0)
	runExecTests(t, []execTest{
		exprtestwdata("log", "{log} Hello {$name} // comment\n{/log}", ``, d{"name": "Rob"}),
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
		exprtestwdata("sanitized var", "{$var}", "&lt;a&gt;", d{"var": "<a>"}),
		exprtestwdata("noAutoescape var", "{$var |noAutoescape}", "<a>", d{"var": "<a>"}),

		// |id == |noAutoescape (it's deprecated)
		exprtest("id", "{'<a>'|id}", "<a>"),

		exprtest("escapeHtml", "{'<a>'|escapeHtml}", "&lt;a&gt;"),

		exprtest("escapeUri1", "{''|escapeUri}", ""),
		exprtest("escapeUri2", "{'a%b > c'|escapeUri}", "a%25b+%3E+c"),
		// TODO: test it escapes kind=HTML content
		// TODO: test it does not escape kind=URI content

		exprtestwdata("ejs1", "{$var|escapeJsString}", ``, d{"var": ""}),
		exprtestwdata("ejs2", "{$var|escapeJsString}", `foo`, d{"var": "foo"}),
		exprtestwdata("ejs3", "{$var|escapeJsString}", `foo\\bar`, d{"var": "foo\\bar"}),
		// TODO: test it even escapes "kind=HTML" content
		// TODO: test it does not escape "kind=JS_STR" content
		exprtestwdata("ejs4", "{$var|escapeJsString}", `\\`, d{"var": "\\"}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\'\'`, d{"var": "''"}),
		exprtestwdata("ejs5", "{$var|escapeJsString}", `\"foo\"`, d{"var": `"foo"`}),
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

var helloWorldTemplate = `{namespace examples.simple}
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
{/template}`

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

func TestStructData(t *testing.T) {
	runExecTests(t, []execTest{
		{"1 name", "examples.simple.helloName", helloWorldTemplate,
			"Hello Ana!",
			struct{ Name string }{"Ana"},
			true,
		},

		{"additional names", "examples.simple.helloNames", helloWorldTemplate,
			"Hello Ana!<br>Hello Bob!<br>Hello Cid!<br>Hello Dee!",
			struct {
				Name            string
				AdditionalNames []string
			}{"Ana", []string{"Bob", "Cid", "Dee"}},
			true,
		},
	})
}

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

// Ensure that variables have the appropriate scope.
// Ensure that the input data map is not updated.
// Ensure that let variables are not passed with data="all"
func TestLetScopes(t *testing.T) {
	var m = data.Map{"a": data.Int(1), "z": data.Map{"y": data.Int(9)}}
	var mcopy = data.Map{"a": data.Int(1), "z": data.Map{"y": data.Int(9)}}
	runExecTests(t, []execTest{
		{"letscopes", "test.main", `{namespace test}
/** @param a */
{template .main}

// starting value
{$a}

// reassign with a let
{let $a: 2 /}
{$a}

// data="all" should not pass "let" assignments
// $a should not be updated by the {let} in .inner
{call .inner data="all"/}
{$a}

// for loops should create a new scope, not update the existing variable
{for $a in range(5, 6)}
  {$a}
{/for}
{$a}

// reassign to the same value
{let $a}
  {let $b: $a/}
  {$b}
{/let}
{$a}

// reassign to a different value
{let $a:6/}
{$a}
{/template}

/**
 * @param a
 * @param? b
 */
{template .inner}
{$a}
{if $b}{$b}{/if}
{let $a: 3 /}
{$a}
{call .inner2 data="all"/}
{$a}
{/template}

/** @param a */
{template .inner2}
{$a}
{let $a: 4 /}
{$a}
{/template}
`, "121314325226", m, true},

		{"no-overwrite-map", "test.main", `{namespace test}
/** @param z */
{template .main}
{call .inner data="$z"/}
{/template}

/** @param y */
{template .inner}
{let $a: 8/}
{$y} {$a}
{let $y: 7/}
{sp}{$y}
{/template}
`, "9 8 7", m, true},

		{"no-overwrite-map-params", "test.main", `{namespace test}
/** @param z */
{template .main}
{call .inner data="$z"}
{param foo: false /}
{/call}
{/template}

/** @param y */
{template .inner}
{let $a: 8/}
{$y} {$a}
{let $y: 7/}
{sp}{$y}
{/template}
`, "9 8 7", m, true},
	})

	if !reflect.DeepEqual(m, mcopy) {
		t.Errorf("input data map changed: %v", m)
	}
}

// TestCallData checks that the various cases around passing data in {call} are
// working according to spec.
func TestCallData(t *testing.T) {
	runExecTests(t, []execTest{
		// test that data=$property is subsequently passed through data=all
		{"data=$a", "test.main", `{namespace test}
/** @param a */
{template .main}
{call .inner data="$a"/}
{/template}

/** @param b */
{template .inner}
{$b}
{let $b: 2/}
{$b}
{call .inner2 data="all"/}
{/template}

/** @param b */
{template .inner2}
{$b}
{let $b: 2/}
{$b}
{/template}`, "1212", d{"a": d{"b": 1}}, true},

		// test that explicit params are included in data="all"
		{"data=all+param", "test.main", `{namespace test}
/** @param a */
{template .main}
{call .inner data="all"}
  {param b: 2/}
{/call}
{/template}

/**
 * @param a
 * @param b
 */
{template .inner}
{call .inner2 data="all"/}
{/template}

/**
 * @param a
 * @param b
 */
{template .inner2}
{$a}{$b}
{/template}`, "12", d{"a": 1}, true},

		// test that explicit params are included in data="all"
		{"data=all+param", "test.main", `{namespace test}
/** @param a */
{template .main}
{call .inner data="$b"}
  {param b: 2/}
{/call}
{/template}

/**
 * @param a
 * @param b
 */
{template .inner}
{call .inner2 data="all"/}
{/template}

/**
 * @param a
 * @param b
 */
{template .inner2}
{$a}{$b}
{/template}`, "12", d{"b": d{"a": 1}}, true},

		// test multiple calls with different data sets
		{"multiple data=all+param", "test.main", `{namespace test}
/** @param a */
{template .main}
{call .inner data="all"}
  {param a: 2/}
{/call}
{call .inner data="all"}
  {param b: 3/}
{/call}
{/template}

/**
 * @param a
 * @param? b
 */
{template .inner}
{$a}{if $b}{$b}{/if}
{/template}
`, "213", d{"a": 1}, true},
	})
}

type fakeBundle struct {
	msgs       map[uint64]*soymsg.Message
	pluralfunc po.PluralSelector
}

func (fb *fakeBundle) Message(id uint64) *soymsg.Message {
	if fb == nil || fb.msgs == nil {
		return nil
	}
	return fb.msgs[id]
}

func (fb *fakeBundle) Locale() string {
	return "xx"
}

func (fb *fakeBundle) PluralCase(n int) int {
	return fb.pluralfunc(n)
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

func newFakeBundle(msg, tran string, pl po.PluralSelector) *fakeBundle {
	var sf, err = parse.SoyFile("", `{msg desc=""}`+msg+`{/msg}`)
	if err != nil {
		panic(err)
	}
	var msgnode = sf.Body[0].(*ast.MsgNode)
	soymsg.SetPlaceholdersAndID(msgnode)
	var m = soymsg.NewMessage(msgnode.ID, tran)
	return &fakeBundle{map[uint64]*soymsg.Message{msgnode.ID: &m}, pl}
}

func newFakePluralBundle(pluralVar, msg1, msg2 string, pl po.PluralSelector, msgstr []string) *fakeBundle {
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
	return &fakeBundle{map[uint64]*soymsg.Message{msgnode.ID: &msg}, pl}
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
{/template}`}, "Hello world", nil, nil, true},

		{"bundle lacks", "test.main", []string{`{namespace test}
{template .main}
  {msg desc=""}
    Hello world
  {/msg}
{/template}`}, "Hello world", nil, newFakeBundle("foo", "bar", nil), true},

		{"bundle has", "test.main", []string{`{namespace test}
{template .main}
  {msg desc=""}
    Hello world
  {/msg}
{/template}`}, "Sup", nil, newFakeBundle("Hello world", "Sup", nil), true},

		{"msg with variable & translation", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    a: {$a}
  {/msg}
{/template}`}, "a is 1", d{"a": 1}, newFakeBundle("a: {$a}", "a is {A}", nil), true},

		{"msg w variables", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    {$a}{$a} xx {$a}{sp}
  {/msg}
{/template}`}, "11xxx1", d{"a": 1}, newFakeBundle("{$a}{$a} xx {$a}{sp}", "{A}{A}xxx{A}", nil), true},

		{"msg w numbered placeholders", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    {$a.a}{$a.b.a}
  {/msg}
{/template}`}, "21", d{"a": d{"a": 1, "b": d{"a": 2}}},
			newFakeBundle("{$a.a}{$a.b.a}", "{A_2}{A_1}", nil), true},

		{"msg w html", "test.main", []string{`{namespace test}
/** @param a */
{template .main}
  {msg desc=""}
    Click <a>here</a>
  {/msg}
{/template}`}, "<a>Click here</a>", nil,
			newFakeBundle("Click <a>here</a>", "{START_LINK}Click here{END_LINK}", nil), true},

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
			nil, true},

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
			nil, true},

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
{/template}`}, "|one user|", d{"n": 1},
			newFakePluralBundle("$n", "one user", "{$n} users",
				pluralEnglish, []string{"|one user|", "|({N_2}) users|"}),
			true},

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
{/template}`}, "|(10) users|", d{"n": 10},
			newFakePluralBundle("$n", "one user", "{$n} users",
				pluralEnglish, []string{"|one user|", "|({N_2}) users|"}),
			true},

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
{/template}`}, "|few (3) users|", d{"n": 3},
			newFakePluralBundle("$n", "one user", "{$n} users",
				pluralCzech, []string{"|one user|", "|few ({N_2}) users|", "|({N_2}) users|"}),
			true},
	})
}

// testing cross namespace stuff requires multiple file bodies
type nsExecTest struct {
	name         string
	templateName string
	input        []string
	output       string
	data         interface{}
	msgs         *fakeBundle
	ok           bool
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
			"Hello world", nil, nil, true},
	})
}

// helpers

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

func runExecTests(t *testing.T, tests []execTest) {
	var nstest []nsExecTest
	for _, test := range tests {
		nstest = append(nstest, nsExecTest{
			test.name,
			test.templateName,
			[]string{test.input},
			test.output,
			test.data,
			nil,
			test.ok,
		})
	}
	runNsExecTests(t, nstest)
}

func runNsExecTests(t *testing.T, tests []nsExecTest) {
	b := new(bytes.Buffer)
	for _, test := range tests {
		var registry = template.Registry{}
		for _, input := range test.input {
			var tree, err = parse.SoyFile("", input)
			if err != nil {
				t.Errorf("%s: parse error: %s", test.name, err)
				continue
			}
			registry.Add(tree)
		}
		parsepasses.SetGlobals(registry, globals)
		parsepasses.ProcessMessages(registry)

		b.Reset()
		var datamap data.Map
		if test.data != nil {
			datamap = data.New(test.data).(data.Map)
		}
		tofu := NewTofu(&registry).NewRenderer(test.templateName).
			Inject(ij)
		if test.msgs != nil {
			tofu.WithMessages(test.msgs)
		}
		err := tofu.Execute(b, datamap)
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
