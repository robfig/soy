package soyjs

import (
	"bytes"

	"strings"
	"testing"

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

func (t execTest) fails() execTest {
	t.ok = false
	return t
}

// TODO: run the javascript functions.
func runExecTests(t *testing.T, tests []execTest) {
	for _, test := range tests {
		soyfile, err := parse.Soy(test.name, test.input, nil)
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

		if buf.String() != test.output {
			t.Errorf("expected %q, actual\n%v", test.output, buf.String())
		}
	}

}

func exprtestwdata(name, expr, result string, data map[string]interface{}) execTest {
	return execTest{name, "test." + strings.Replace(name, " ", "_", -1),
		"{namespace test}{template ." + strings.Replace(name, " ", "_", -1) + "}" + expr + "{/template}",
		result, data, true}
}

func exprtest(name, expr, result string) execTest {
	return exprtestwdata(name, expr, result, nil)
}
