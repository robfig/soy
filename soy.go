package soy

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/robfig/soy/parse"
)

// Tofu is the aggregate of all your soy.
type Tofu struct {
	templates map[string]*parse.TemplateNode
	globals   map[string]interface{}
}

func New() Tofu {
	return Tofu{make(map[string]*parse.TemplateNode), make(map[string]interface{})}
}

// Globals configures Tofu with the given input file, in the form:
// <global_name> = <primitive_data>
// - Empty lines and lines beginning with '//' are ignored.
// - <primitive_data> must be a valid template expression literal for a primitive
//   type (null, boolean, integer, float, or string)
func (tofu Tofu) ParseGlobals(input io.Reader) error {
	var scanner = bufio.NewScanner(input)
	for scanner.Scan() {
		var line = scanner.Text()
		if len(line) == 0 || strings.HasPrefix(line, "//") {
			continue
		}
		var eq = strings.Index(line, "=")
		if eq == -1 {
			return fmt.Errorf("no equals on line: %q", line)
		}
		var (
			name = strings.TrimSpace(line[:eq])
			expr = strings.TrimSpace(line[eq+1:])
		)
		if _, ok := tofu.globals[name]; ok {
			return fmt.Errorf("global %s is already defined", name)
		}
		var node, err = parse.ParseExpr(expr)
		if err != nil {
			return err
		}
		exprValue, err := EvalExpr(node)
		if err != nil {
			return err
		}
		tofu.globals[name] = exprValue.Interface()
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func (tofu Tofu) Parse(input string) error {
	var tree, err = parse.Parse("", input)
	if err != nil {
		return err
	}
	// collect the parsed templates, associated with the template names.
	var nodes = tree.Root.Nodes
	if len(nodes) == 0 {
		return errors.New("empty")
	}

	// require namespace before any templates
	var namespace *parse.NamespaceNode
	for _, node := range nodes {
		var ok bool
		if namespace, ok = node.(*parse.NamespaceNode); ok {
			break
		}
		if _, ok = node.(*parse.TemplateNode); ok {
			return errors.New("namespace must come before any templates")
		}
	}

	// get all the template nodes
	for _, n := range tree.Root.Nodes[1:] {
		if tmpl, ok := n.(*parse.TemplateNode); ok {
			tofu.templates[namespace.Name+tmpl.Name] = tmpl
		}
	}
	return nil
}

func (tofu Tofu) Template(name string) (tmpl Template, ok bool) {
	node, ok := tofu.templates[name]
	if !ok {
		return
	}
	return Template{node, namespace(name), tofu}, true
}

func namespace(fqTemplateName string) string {
	return fqTemplateName[:strings.LastIndex(fqTemplateName, ".")]
}

type Template struct {
	node      *parse.TemplateNode
	namespace string
	tofu      Tofu
}
