package soy

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"

	"github.com/robfig/soy/parse"
)

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl *parse.TemplateNode
	wr   io.Writer
	node parse.Node // current node, for errors
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value reflect.Value
}

var zero reflect.Value

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// doublePercent returns the string with %'s replaced by %%, if necessary,
// so it can be used safely inside a Printf format string.
func doublePercent(str string) string {
	if strings.Contains(str, "%") {
		str = strings.Replace(str, "%", "%%", -1)
	}
	return str
}

// errorf formats the error and terminates processing.
func (s *state) errorf(format string, args ...interface{}) {
	name := doublePercent(s.tmpl.Name)
	format = fmt.Sprintf("template: %s: %s", name, format)
	panic(fmt.Errorf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case error:
			*errp = err
		default:
			panic(e)
		}
	}
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Template) Execute(wr io.Writer, data interface{}) (err error) {
	if t.node == nil {
		return errors.New("no template found")
	}
	defer errRecover(&err)
	value := reflect.ValueOf(data)
	state := &state{
		tmpl: t.node,
		wr:   wr,
	}
	state.walk(value, t.node)
	return
}

// Walk functions step through the major pieces of the template structure,
// generating output as they go.
func (s *state) walk(dot reflect.Value, node parse.Node) {
	s.at(node)
	switch node := node.(type) {
	case *parse.TemplateNode:
		s.walk(dot, node.Body)
	case *parse.VariableNode:
		s.printVariable(dot, node)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(dot, node)
		}
	case *parse.TextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.errorf("%s", err)
		}
	default:
		fmt.Println("unknown node:", node)
		s.errorf("unknown node: %s", node)
	}
}

func (s *state) printVariable(dot reflect.Value, node *parse.VariableNode) {
	val := dot.MapIndex(reflect.ValueOf(node.Name[1:])) // peel off the $
	if !val.IsValid() {
		s.errorf("variable %s is not valid", node.Name)
	}
	if _, err := s.wr.Write([]byte(fmt.Sprint(val.Interface()))); err != nil {
		s.errorf("%s", err)
	}
}
