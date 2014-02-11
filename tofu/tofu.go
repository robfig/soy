// Package tofu renders a compiled set of Soy to HTML.
package tofu

import (
	"errors"
	"io"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/parse"
	"github.com/robfig/soy/template"
)

// Tofu aggregates your soy.
type Tofu struct {
	template.Registry
}

func (t Tofu) Template(name string) *Renderer {
	var tmpl = t.Registry.Template(name)
	if tmpl == nil {
		return nil
	}
	return &Renderer{*tmpl, t, nil}
}

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
	if t.TemplateNode == nil {
		return errors.New("no template found")
	}
	var autoescapeMode = t.Namespace.Autoescape
	if autoescapeMode == parse.AutoescapeUnspecified {
		autoescapeMode = parse.AutoescapeOn
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
	state.walk(t.TemplateNode)
	return
}
