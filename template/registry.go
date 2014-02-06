package template

import (
	"fmt"

	"github.com/robfig/soy/parse"
)

type Registry struct {
	SoyFileNodes []*parse.ListNode
	Templates    []Template
}

type Template struct {
	*parse.SoyDocNode   // this template's SoyDoc
	*parse.TemplateNode // this template's node
}

// Add the given list node (representing a soy file) to the registry.
// The following rules are enforced:
// - Every soyfile must begin with a {namespace} (except for leading SoyDoc)
// - Every template must be preceeded by SoyDoc
func (r *Registry) Add(soyfile *parse.ListNode) error {
	for _, node := range soyfile.Nodes {
		switch node := node.(type) {
		case *parse.SoyDocNode:
			continue
		case *parse.NamespaceNode:
		default:
			return fmt.Errorf("expected namespace, found %#v", node)
		}
		break
	}

	r.SoyFileNodes = append(r.SoyFileNodes, soyfile)
	for i := 0; i < len(soyfile.Nodes); i++ {
		var tn, ok = soyfile.Nodes[i].(*parse.TemplateNode)
		if !ok {
			continue
		}

		// Technically every template requires soydoc, but having to add empty
		// soydoc just to get a template to compile is just stupid.  (There is a
		// separate data ref check to ensure any variables used are declared as
		// params, anyway).
		sdn, ok := soyfile.Nodes[i-1].(*parse.SoyDocNode)
		if !ok {
			sdn = &parse.SoyDocNode{tn.Pos, nil}
		}
		r.Templates = append(r.Templates, Template{sdn, tn})
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

// ParamNames returns the names of all params declared by the template's SoyDoc.
func (t Template) ParamNames() []string {
	var names []string
	for _, p := range t.Params {
		names = append(names, p.Name)
	}
	return names
}
