package bytecode

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/template"
)

type compileTest struct {
	template string
	expected *Program
}

func TestCompiler(t *testing.T) {
	tests := []compileTest{
		{
			`hello, world`,
			&Program{
				Instr: []Opcode{
					RawText, 0,
					Return,
				},
				RawTexts: [][]byte{
					[]byte(`hello, world`),
				},
				Templates: []Template{{
					Name: "test.sayHello",
					PC:   0,
				}},
			},
		},
	}

	for _, test := range tests {
		prog := mustCompileExpr(t, test.template)
		prog.TemplateByName = nil
		if diff := cmp.Diff(test.expected, prog); diff != "" {
			t.Fatalf("compiled program does not match actual:\n%s", diff)
		}
	}
}

func mustCompileExpr(t *testing.T, body string) *Program {
	registry := mustParse(t, fmt.Sprintf(`{namespace test}

{template .sayHello}
  %s
{/template}`, body))
	prog, err := Compile(registry)
	if err != nil {
		t.Fatalf("error compiling %q: %+v", body, err)
	}
	return prog
}

func mustParse(t *testing.T, templates ...string) *template.Registry {
	var registry = &template.Registry{}
	for _, soyfile := range templates {
		var tree, err = parse.SoyFile("", soyfile)
		if err != nil {
			t.Fatal(err)
		}
		if err = registry.Add(tree); err != nil {
			t.Fatal(err)
		}
	}
	return registry
}
