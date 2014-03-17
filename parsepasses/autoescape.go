package parsepasses

import (
	"fmt"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/soyhtml"
	"github.com/robfig/soy/template"
)

// Autoescape rewrites all templates in the given registry to add
// appropriately-escaping print directives to all print commands.
func Autoescape(reg template.Registry) (err error) {
	var currentTemplate string
	defer func() {
		if err2 := recover(); err2 != nil {
			err = fmt.Errorf("template %v: %v", currentTemplate, err2)
		}
	}()

	for _, t := range reg.Templates {
		currentTemplate = t.Node.Name
		if t.Namespace.Autoescape == ast.AutoescapeContextual {
			panic("contextual autoescaping is not implemented")
		}
		var a = autoescaper{t.Namespace.Autoescape}
		a.walk(t.Node)
	}
	return nil
}

type autoescaper struct {
	mode ast.AutoescapeType // current escaping mode
}

func (a *autoescaper) walk(node ast.Node) {
	var prev = a.mode
	switch node := node.(type) {
	case *ast.TemplateNode:
		if node.Autoescape != ast.AutoescapeUnspecified {
			a.mode = node.Autoescape
		}
	case *ast.PrintNode:
		if a.mode == ast.AutoescapeOn || a.mode == ast.AutoescapeUnspecified {
			a.escape(node)
		}
	}
	if parent, ok := node.(ast.ParentNode); ok {
		for _, child := range parent.Children() {
			a.walk(child)
		}
	}
	a.mode = prev
}

func (a *autoescaper) escape(node *ast.PrintNode) {
	for _, dir := range node.Directives {
		var d = soyhtml.PrintDirectives[dir.Name]
		if d.CancelAutoescape {
			return
		}
	}
	node.Directives = append(node.Directives, &ast.PrintDirectiveNode{node.Pos, "escapeHtml", nil})
}
