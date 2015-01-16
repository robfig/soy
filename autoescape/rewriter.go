package autoescape

import (
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/template"
)

type rewriter struct {
	inferences *inferences
}

func rewrite(inferences *inferences, registry *template.Registry) {
	var rewriter = rewriter{inferences}
	for _, t := range registry.Templates {
		rewriter.walk(t.Node)
	}
}

func (r *rewriter) walk(node ast.Node) {
	switch node := node.(type) {
	case *ast.PrintNode:
		// Add print directives for the given escaping modes.
		// TODO: There are some cases where they shouldn't be at the end
		for _, escapingMode := range r.inferences.escapingModes[node] {
			node.Directives = append(node.Directives, &ast.PrintDirectiveNode{
				Pos:  node.Pos,
				Name: escapingMode.directiveName,
			})
		}

	case *ast.CallNode:
		// TODO
	}

	if node, ok := node.(ast.ParentNode); ok {
		for _, child := range node.Children() {
			r.walk(child)
		}
	}
}
