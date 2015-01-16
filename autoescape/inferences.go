package autoescape

import (
	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/template"
)

type inferences struct {
	templatesByName map[string]*template.Template

	escapingModes        map[ast.Node][]escapingMode
	idToStartContext     map[ast.Node]context
	templateToEndContext map[*ast.TemplateNode]context
}

func newInferences(reg *template.Registry) *inferences {
	var templatesByName = make(map[string]*template.Template)
	for _, t := range reg.Templates {
		templatesByName[t.Node.Name] = &t
	}
	return &inferences{
		templatesByName:      templatesByName,
		escapingModes:        make(map[ast.Node][]escapingMode),
		idToStartContext:     make(map[ast.Node]context),
		templateToEndContext: make(map[*ast.TemplateNode]context),
	}
}

func (i *inferences) setEscapingDirectives(node ast.Node, ctx context, escapes []escapingMode) {
	i.escapingModes[node] = escapes
	i.idToStartContext[node] = ctx
}

func (i *inferences) recordTemplateEndContext(tmpl *ast.TemplateNode, ctx context) {
	i.templateToEndContext[tmpl] = ctx
}
