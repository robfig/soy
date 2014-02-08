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
	return &Renderer{t, *tmpl}
}

type Renderer struct {
	tofu Tofu
	template.Template
}

// Render applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Renderer) Render(wr io.Writer, obj data.Map) (err error) {
	if t.TemplateNode == nil {
		return errors.New("no template found")
	}
	var autoescapeMode = t.Template.Namespace.Autoescape
	if autoescapeMode == parse.AutoescapeUnspecified {
		autoescapeMode = parse.AutoescapeOn // TODO: Contextual
	}
	state := &state{
		tmpl:       t.Template,
		registry:   t.tofu.Registry,
		namespace:  t.Namespace.Name,
		autoescape: autoescapeMode,
		wr:         wr,
		context:    scope{obj},
	}
	defer state.errRecover(&err)
	state.walk(t.TemplateNode)
	return
}
