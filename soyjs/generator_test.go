package soyjs

import (
	"bytes"
	"testing"

	"github.com/robertkrimen/otto"
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/template"
)

func TestGenerator(t *testing.T) {
	var otto = otto.New()
	var _, err = otto.Run(`
var soy = {};
soy.$$escapeHtml = function(arg) { return arg; };
`)
	if err != nil {
		t.Error(err)
		return
	}

	Funcs["capitalize"] = Func{func(js JSWriter, args []ast.Node) {
		js.Write("(", args[0], ".charAt(0).toUpperCase() + ", args[0], ".slice(1))")
	}, []int{1}}
	defer delete(Funcs, "capitalize")

	soyfile, err := parse.SoyFile("name.soy", `
{namespace test}
{template .funcs}
{let $place: 'world'/}
{capitalize('hel' + 'lo')}, {capitalize($place)}
{/template}`)
	if err != nil {
		t.Error(err)
		return
	}

	var registry = template.Registry{}
	if err = registry.Add(soyfile); err != nil {
		t.Error(err)
		return
	}

	var gen = NewGenerator(&registry)
	var buf bytes.Buffer
	err = gen.WriteFile(&buf, "name.soy")
	if err != nil {
		t.Error(err)
		return
	}

	_, err = otto.Run(buf.String())
	if err != nil {
		t.Error(err)
		return
	}

	output, err := otto.Run(`test.funcs();`)
	if err != nil {
		t.Error(err)
		return
	}
	if output.String() != "Hello, World" {
		t.Errorf("Got %q, expected Hello, World", output.String())
	}
}
