package template

import (
	"fmt"
	"log"
	"strings"

	"github.com/robfig/soy/parse"
)

// Registry provides convenient access to a collection of parsed Soy templates.
type Registry struct {
	SoyFiles  []*parse.Tree
	Templates []Template

	// sourceByTemplateName maps FQ template name to the input source it came from.
	sourceByTemplateName map[string]string
}

// Add the given list node (representing a soy file) to the registry.
// Every soyfile must begin with a {namespace} (except for leading SoyDoc)
func (r *Registry) Add(soyfile *parse.Tree) error {
	if r.sourceByTemplateName == nil {
		r.sourceByTemplateName = make(map[string]string)
	}
	var ns *parse.NamespaceNode
	for _, node := range soyfile.Root.Nodes {
		switch node := node.(type) {
		case *parse.SoyDocNode:
			continue
		case *parse.NamespaceNode:
			ns = node
		default:
			return fmt.Errorf("expected namespace, found %v", node)
		}
		break
	}

	r.SoyFiles = append(r.SoyFiles, soyfile)
	for i := 0; i < len(soyfile.Root.Nodes); i++ {
		var tn, ok = soyfile.Root.Nodes[i].(*parse.TemplateNode)
		if !ok {
			continue
		}

		// Technically every template requires soydoc, but having to add empty
		// soydoc just to get a template to compile is just stupid.  (There is a
		// separate data ref check to ensure any variables used are declared as
		// params, anyway).
		sdn, ok := soyfile.Root.Nodes[i-1].(*parse.SoyDocNode)
		if !ok {
			sdn = &parse.SoyDocNode{tn.Pos, nil}
		}
		r.Templates = append(r.Templates, Template{sdn, tn, ns})
		r.sourceByTemplateName[tn.Name] = soyfile.Text
	}
	return nil
}

// Template allows lookup by (fully-qualified) template name
func (r *Registry) Template(name string) *Template {
	for _, t := range r.Templates {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

// LineNumber computes the line number in the input source for the given node
// within the given template.
func (r *Registry) LineNumber(templateName string, node parse.Node) int {
	var src, ok = r.sourceByTemplateName[templateName]
	if !ok {
		log.Println("template not found:", templateName)
		return 0
	}
	return 1 + strings.Count(src[:node.Position()], "\n")
}
