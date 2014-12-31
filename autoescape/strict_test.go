package autoescape

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/soyhtml"
	"github.com/robfig/soy/template"
)

// "Anatomy of an XSS Hack" example from the Security documentation
const (
	xssExample = `
  <a href="{$x}"
   onclick="{$x}"
   >{$x}</a>
  <script>var x = '{$x}'</script>
  <style>
    p {lb}{sp}
      font-family: "{$x}";
      background: url(/images?q={$x});
      left: {$x}
    {rb}
  </style>`

	xssExampleVar = `javascript:/*</style></script>/**/ /<script>1/(alert(1337))//</script>`

	xssExampleAnswer = `<a href="#zSoyz" ` +
		`onclick="&#34;javascript:/*\u003c/style\u003e\u003c/script\u003e/**/ /\u003cscript\u003e1/(alert(1337))//\u003c/script\u003e&#34;">` +
		`javascript:/*&lt;/style&gt;&lt;/script&gt;/**/ /&lt;script&gt;1/(alert(1337))//&lt;/script&gt;</a>` +
		`<script>var x = 'javascript:\/*\x3c\/style\x3e\x3c\/script\x3e\/**\/ \/\x3cscript\x3e1\/(alert(1337))\/\/\x3c\/script\x3e'</script>` +
		`<style>` +
		`p {` +
		` font-family: "#zSoyz";` +
		` background: url(/images?q=javascript%3a%2f%2a%3c%2fstyle%3e%3c%2fscript%3e%2f%2a%2a%2f%20%2f%3cscript%3e1%2f%28alert%281337%29%29%2f%2f%3c%2fscript%3e);` +
		` left: zSoyz` +
		`}` +
		`</style>`
)

type testCase struct {
	value  interface{} // value of "$x"
	output string      // expected template output
}

type test struct {
	name  string     // test name
	input string     // template text
	cases []testCase // cases to test, one per input value
}

// Tests all the various cases of straight-line escaping in various contexts,
// excluding any control structures.
func TestSimpleTemplateEscaping(t *testing.T) {
	var tests = []test{
		{"anatomy of an XSS hack", xssExample, []testCase{
			{xssExampleVar, xssExampleAnswer},
		}},

		{"URL - Whole URL", `<a href="{$x}">`, []testCase{
			{"http://foo/", `<a href="http://foo/">`},
			{"/foo?a=b&c=d", `<a href="/foo?a=b&amp;c=d">`},
			{"javascript:alert(1337)", `<a href="#zSoyz">`},
		}},

		{"URL - Path", `<a href="/foo/{$x}">`, []testCase{
			{"bar", `<a href="/foo/bar">`},
			{"bar&baz/boo", `<a href="/foo/bar&amp;baz/boo">`},
		}},

		{"URL - Query", `<a href="/foo?q={$x}">`, []testCase{
			{"bar&baz=boo", `<a href="/foo?q=bar%26baz%3dboo">`},
			{"A is #1", `<a href="/foo?q=A%20is%20%231">`},
		}},

		{"JS Str", `<script>alert('{$x}');</script>`, []testCase{
			{"O'Reilly Books", `<script>alert('O\x27Reilly Books');</script>`},
			{"O\"Reilly Books", `<script>alert('O\x22Reilly Books');</script>`},
		}},

		{"JS", `<script>alert({$x});</script>`, []testCase{
			{"O'Reilly Books", `<script>alert("O'Reilly Books");</script>`},
			{42, `<script>alert( 42 );</script>`},
			{true, `<script>alert( true );</script>`},
		}},

		{"CSS - Classes and IDs", `<style>div#{$x} {lb} {rb}</style>`, []testCase{
			{"foo-bar", `<style>div#foo-bar { }</style>`},
		}},

		{"CSS - Quantities", `<div style="color: {$x}">`, []testCase{
			{"red", `<div style="color: red">`},
			{"#f00", `<div style="color: #f00">`},
			{"expression('alert(1337)')", `<div style="color: zSoyz">`},
		}},

		{"CSS - Property names", `<div style="margin-{$x}: 1em">`, []testCase{
			{"left", `<div style="margin-left: 1em">`},
			{"right", `<div style="margin-right: 1em">`},
		}},

		{"CSS - Quoted values", `<style>p {lb} font-family: '{$x}' {rb}</style>`, []testCase{
			{"Arial", `<style>p { font-family: 'Arial' }</style>`},
			{"</style>", `<style>p { font-family: '\3c\2fstyle\3e ' }</style>`},
		}},

		{"CSS - URLs", `<div style="background: url({$x})">`, []testCase{
			{"/foo/bar", `<div style="background: url(/foo/bar)">`},
			{"javascript:alert(1337)", `<div style="background: url(#zSoyz)">`},
			{"?q=(O'Reilly) OR Books", `<div style="background: url(?q=%28O%27Reilly%29%20OR%20Books)">`},
		}},
	}
	runTests(t, tests)
}

type kindedTest struct {
	name  string
	input string
	kind  string
	cases []testCase
}

// Test that applying kind correctly changes the escaping chosen.
func TestKinds(t *testing.T) {
	var tests = []kindedTest{
		{"HTML", `{$x}`, "", []testCase{
			{HTML("<b>hello</b>"), `<b>hello</b>`},
		}},

		{"HTML in CSS context", `{$x}`, "css", []testCase{
			{HTML("</style>"), `\3c\2fstyle\3e `},
		}},
	}

	runKindedTests(t, tests)
}

func runTests(t *testing.T, tests []test) {
	var ktests []kindedTest
	for _, t := range tests {
		ktests = append(ktests, kindedTest{t.name, t.input, "", t.cases})
	}
}

func runKindedTests(t *testing.T, tests []kindedTest) {
	const tmpl = `{namespace example}

/** @param x */
{template .test %s}
%s
{/template}
`

	var b bytes.Buffer
	for _, test := range tests {
		var registry = template.Registry{}
		var k string
		if test.kind != "" {
			k = `kind="` + test.kind + `"`
		}
		var tree, err = parse.SoyFile("", fmt.Sprintf(tmpl, k, test.input), nil)
		if err != nil {
			t.Errorf("%s: parse error: %s", test.name, err)
			return
		}
		registry.Add(tree)
		Strict(&registry)
		for _, testCase := range test.cases {
			b.Reset()
			err := soyhtml.NewTofu(&registry).
				NewRenderer("example.test").
				Execute(&b, data.Map{"x": data.New(testCase.value)})
			if err != nil {
				t.Error(test.name, ": ", err)
				continue
			}
			if b.String() != testCase.output {
				tmp, _ := registry.Template("example.test")
				t.Errorf("%s\n%s\n\ngot\n\t%q\nexpected\n\t%q", test.name, tmp.Node, b.String(), testCase.output)
			}
		}
	}
}
