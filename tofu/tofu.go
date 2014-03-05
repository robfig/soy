// Package tofu renders a compiled set of Soy to HTML.
package tofu

import (
	"errors"
	"io"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/template"
)

var ErrTemplateNotFound = errors.New("template not found")

// Tofu provides access to all your parsed soy templates for rendering into HTML
type Tofu struct {
	template.Registry
}

// Template returns a Renderer that can convert the named template to HTML.
// This method never returns nil and is safe to chain.
func (t Tofu) Template(name string) *Renderer {
	var tmpl, _ = t.Registry.Template(name)
	return &Renderer{tmpl, t, nil}
}

// Renderer is the context for execution of a single template.
type Renderer struct {
	template.Template
	tofu Tofu
	ij   data.Map
}

// InjectData sets the '$ij' data map that is injected into all templates.
func (t *Renderer) InjectData(ij data.Map) *Renderer {
	t.ij = ij
	return t
}

// Render applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Renderer) Render(wr io.Writer, obj data.Map) (err error) {
	if t.Node == nil {
		return ErrTemplateNotFound
	}
	var autoescapeMode = t.Namespace.Autoescape
	if autoescapeMode == ast.AutoescapeUnspecified {
		autoescapeMode = ast.AutoescapeOn
	}
	state := &state{
		tmpl:       t.Template,
		registry:   t.tofu.Registry,
		namespace:  t.Namespace.Name,
		autoescape: autoescapeMode,
		wr:         wr,
		context:    scope{obj},
		ij:         t.ij,
	}
	defer state.errRecover(&err)
	state.walk(t.Node)
	return
}
