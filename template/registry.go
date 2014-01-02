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
	// Namespace string              // the template's namespace, e.g. "examples.test"
	// Name      string              // the partial name, e.g. ".sayHello"
	*parse.SoyDocNode   // this template's SoyDoc
	*parse.TemplateNode // this template's node
}

// Add adds the given list node (representing a soy file) to the registry.
// TODO The following rules are enforced:
// - Every soyfile must begin with a {namespace} (except for leading SoyDoc)
// - Every template must be preceeded by SoyDoc
func (r *Registry) Add(soyfile *parse.ListNode) error {
	r.SoyFileNodes = append(r.SoyFileNodes, soyfile)
	for i := 0; i < len(soyfile.Nodes); i++ {
		var tn, ok = soyfile.Nodes[i].(*parse.TemplateNode)
		if !ok {
			continue
		}
		sdn, ok := soyfile.Nodes[i-1].(*parse.SoyDocNode)
		if !ok {
			return fmt.Errorf("Template %q requires SoyDoc", tn.Name)
		}
		r.Templates = append(r.Templates, Template{sdn, tn})
	}
	return nil
}

// TODO: FQ name or what?
func (r *Registry) Template(name string) *Template {
	for _, t := range r.Templates {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

func (t Template) ParamNames() []string {
	var names []string
	for _, p := range t.Params {
		names = append(names, p.Name)
	}
	return names
}
