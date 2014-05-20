package autoescape

import (
	"bytes"
	"testing"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/soyhtml"
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

type nsExecTest struct {
	name         string
	templateName string
	input        []string
	output       string
	data         interface{}
	ok           bool
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
		})
	}
	runNsExecTests(t, nstest)
}

func runNsExecTests(t *testing.T, tests []nsExecTest) {
	var err error
	var b = new(bytes.Buffer)
	for _, test := range tests {
		var registry = template.Registry{}
		for _, input := range test.input {
			var tree, err = parse.SoyFile("", input, nil)
			if err != nil {
				t.Errorf("%s: parse error: %s", test.name, err)
				continue
			}
			registry.Add(tree)
		}
		err = Simple(&registry)
		if err == nil {
			b.Reset()
			var datamap data.Map
			if test.data != nil {
				datamap = data.New(test.data).(data.Map)
			}
			err = soyhtml.NewTofu(&registry).NewRenderer(test.templateName).
				Execute(b, datamap)
		}
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
