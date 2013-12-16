package soy

import (
	"errors"

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
	// Maybe don't require namespaces
	// if nodes[0].Type() != NodeNamespace {
	// 	return errors.New("input must begin with a namespace declaration")
	// }
	// var namespace = nodes[0].Name
	for _, n := range tree.Root.Nodes {
		if n.Type() == parse.NodeTemplate {
			tmpl := n.(*parse.TemplateNode)
			tofu.templates[tmpl.Name] = tmpl
		}
	}
	return nil
}

func (tofu Tofu) Template(name string) (tmpl Template, ok bool) {
	node, ok := tofu.templates[name]
	if !ok {
		return
	}
	return Template{node}, true
}

type Template struct {
	node *parse.TemplateNode
}
