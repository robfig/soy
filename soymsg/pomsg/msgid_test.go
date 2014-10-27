package pomsg

import (
	"fmt"
	"testing"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/soymsg"
)

func TestValidate(t *testing.T) {
	type test struct {
		msg       *ast.MsgNode
		validates bool
	}
	var tests = []test{
		{msg(""), true},
		{msg("hello world"), true},
		{msg("{plural $n}{case 1}one{default}other{/plural}"), true},
		{msg("{plural $n}{default}other{/plural}"), false},
		{msg("{plural $n}{case 2}two{default}other{/plural}"), false},
	}

	for _, test := range tests {
		var err = Validate(test.msg)
		switch {
		case test.validates && err != nil:
			t.Errorf("should validate, but got %v: %v", err, test.msg)
		case !test.validates && err == nil:
			t.Errorf("should fail, but didn't: %v", test.msg)
		}
	}
}

func TestMsgId(t *testing.T) {
	type test struct {
		msg         *ast.MsgNode
		msgid       string
		msgidPlural string
	}
	var tests = []test{
		{msg(""), "", ""},
		{msg("hello world"), "hello world", ""},
		{msg("{plural length($users)}{case 1}one{default}other{/plural}"), "one", "other"},
		{msg("{plural length($users)}{case 1}one{default}{length($users)} users{/plural}"),
			"one", "{XXX} users"},
	}

	for _, test := range tests {
		var (
			msgid       = Msgid(test.msg)
			msgidPlural = MsgidPlural(test.msg)
		)
		if msgid != test.msgid {
			t.Errorf("(actual) %v != %v (expected)", msgid, test.msgid)
		}
		if msgidPlural != test.msgidPlural {
			t.Errorf("(actual) %v != %v (expected)", msgidPlural, test.msgidPlural)
		}
	}
}

func msg(body string) *ast.MsgNode {
	var msgtmpl = fmt.Sprintf(`{msg desc=""}%s{/msg}`, body)
	var sf, err = parse.SoyFile("", msgtmpl)
	if err != nil {
		panic(err)
	}
	var msgnode = sf.Body[0].(*ast.MsgNode)
	soymsg.SetPlaceholdersAndID(msgnode)
	return msgnode
}
