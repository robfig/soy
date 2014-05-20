package autoescape

import (
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/template"
)

type callGraph struct {
	registry        *template.Registry
	calls           map[string]string // key calls value
	calledBy        map[string]string // key is called by value
	currentTemplate string
}

func newCallGraph(reg *template.Registry) callGraph {
	var cgg = callGraph{reg, make(map[string]string), make(map[string]string), ""}
	for _, t := range reg.Templates {
		cgg.walk(t.Node)
	}
	return cgg
}

func (g *callGraph) roots() []template.Template {
	var roots []template.Template
	for _, t := range g.registry.Templates {
		if _, ok := g.calledBy[t.Node.Name]; !ok {
			roots = append(roots, t)
		}
	}
	return roots
}

func (g *callGraph) walk(node ast.Node) {
	switch node := node.(type) {
	case *ast.TemplateNode:
		g.currentTemplate = node.Name
	case *ast.CallNode:
		g.calls[g.currentTemplate] = node.Name
		g.calledBy[node.Name] = g.currentTemplate
	}
	if parent, ok := node.(ast.ParentNode); ok {
		for _, child := range parent.Children() {
			g.walk(child)
		}
	}
}
