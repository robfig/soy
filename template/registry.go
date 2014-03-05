// Package template provides convenient access to groups of parsed soy files.
package template

import (
	"fmt"
	"log"
	"strings"

	"github.com/robfig/soy/ast"
)

// Registry provides convenient access to a collection of parsed Soy templates.
type Registry struct {
	SoyFiles  []*ast.SoyFileNode
	Templates []Template

	// sourceByTemplateName maps FQ template name to the input source it came from.
	sourceByTemplateName map[string]string
}

// Add the given soy file node to the registry.
// Every soyfile must begin with a {namespace} (except for leading SoyDoc)
func (r *Registry) Add(soyfile *ast.SoyFileNode) error {
	if r.sourceByTemplateName == nil {
		r.sourceByTemplateName = make(map[string]string)
	}
	var ns *ast.NamespaceNode
	for _, node := range soyfile.Body {
		switch node := node.(type) {
		case *ast.SoyDocNode:
			continue
		case *ast.NamespaceNode:
			ns = node
		default:
			return fmt.Errorf("expected namespace, found %v", node)
		}
		break
	}
	if ns == nil {
		return fmt.Errorf("namespace required")
	}

	r.SoyFiles = append(r.SoyFiles, soyfile)
	for i := 0; i < len(soyfile.Body); i++ {
		var tn, ok = soyfile.Body[i].(*ast.TemplateNode)
		if !ok {
			continue
		}

		// Technically every template requires soydoc, but having to add empty
		// soydoc just to get a template to compile is just stupid.  (There is a
		// separate data ref check to ensure any variables used are declared as
		// params, anyway).
		sdn, ok := soyfile.Body[i-1].(*ast.SoyDocNode)
		if !ok {
			sdn = &ast.SoyDocNode{tn.Pos, nil}
		}
		r.Templates = append(r.Templates, Template{sdn, tn, ns})
		r.sourceByTemplateName[tn.Name] = soyfile.Text
	}
	return nil
}

// Template allows lookup by (fully-qualified) template name
// If the template is not found, the zero value for Template is returned.
func (r *Registry) Template(name string) (Template, bool) {
	for _, t := range r.Templates {
		if t.Node.Name == name {
			return t, true
		}
	}
	return Template{}, false
}

// LineNumber computes the line number in the input source for the given node
// within the given template.
func (r *Registry) LineNumber(templateName string, node ast.Node) int {
	var src, ok = r.sourceByTemplateName[templateName]
	if !ok {
		log.Println("template not found:", templateName)
		return 0
	}
	return 1 + strings.Count(src[:node.Position()], "\n")
}
