package bytecode

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type execTest struct {
	template string
	expected string
}

func TestExec(t *testing.T) {
	tests := []execTest{
		{
			`hello, world`,
			`hello, world`,
		},
	}

	for _, test := range tests {
		var buf bytes.Buffer
		prog := mustCompileExpr(t, test.template)
		if err := prog.Execute(&buf, "test.sayHello", nil); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(test.expected, buf.String()); diff != "" {
			t.Fatalf("output does not match expected:\n%s", diff)
		}
	}
}
