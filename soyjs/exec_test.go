package soyjs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"strings"
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
)

// This is the same test suite as tofu/exec_test, verifying that the JS versions
// get the same result.

type d map[string]interface{}

type execTest struct {
	name         string
	templateName string
	input        string
	output       string
	data         interface{}
	ok           bool
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

// func TestLog(t *testing.T) {
// 	// Get console.logs somehow
// 	var buf bytes.Buffer
// 	runExecTests(t, []execTest{
// 		exprtestwdata("log", "{log} Hello {$name} // comment\n{/log}", ``, d{"name": "Rob"}),
// 	})
// 	if strings.TrimSpace(buf.String()) != "Hello Rob" {
// 		t.Errorf("logger didn't match: %q", buf.String())
// 	}
// }

func TestDebugger(t *testing.T) {
	runExecTests(t, []execTest{
		exprtest("debugger", `{debugger}`, ``),
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

func runExecTests(t *testing.T, tests []execTest) {
	for _, test := range tests {
		soyfile, err := parse.Soy(test.name, test.input, globals)
		if err != nil {
			t.Error(err)
			continue
		}

		var buf bytes.Buffer
		err = Write(&buf, soyfile)
		if err != nil {
			t.Error(err)
			continue
		}

		var otto = otto.New()
		_, err = otto.Run(buf.String())
		if err != nil {
			t.Errorf("compile error: %v\n%v", err, buf.String())
			continue
		}

		var jsonData, _ = json.Marshal(test.data)
		var renderStatement = fmt.Sprintf("%s(%s);", test.templateName, string(jsonData))
		actual, err := otto.Run(renderStatement)
		if err != nil {
			t.Errorf("render error: %v\n%v\n%v", err, buf.String(), renderStatement)
			continue
		}

		if test.output != actual.String() {
			t.Errorf("expected:\n%v\n\nactual:\n%v\n%v", test.output, actual.String(), renderStatement)
		}
	}
}
