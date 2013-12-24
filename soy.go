package soy

import (
	"errors"
	"strings"

	"github.com/robfig/soy/parse"
)

// Tofu is the aggregate of all your soy.
// The zero value is ready to use.
type Tofu struct {
	templates map[string]*parse.TemplateNode
}

func New() Tofu {
	return Tofu{make(map[string]*parse.TemplateNode)}
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
