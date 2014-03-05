// Package soyhtml renders a compiled set of Soy to HTML.
package soyhtml

import (
	"errors"
	"io"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/template"
)

var ErrTemplateNotFound = errors.New("template not found")

// Renderer provides parameters to template execution.
type Renderer struct {
	Registry   *template.Registry        // a registry of all templates in a bundle
	Template   string                    // fully-qualified name of the template to render
	Inject     data.Map                  // data for the $ij map
	Funcs      map[string]Func           // augments default funcs.
	Directives map[string]PrintDirective // augments default print directives
}

// Execute applies a parsed template to the specified data object,
// and writes the output to wr.
func (t Renderer) Execute(wr io.Writer, obj data.Map) (err error) {
	var tmpl, ok = t.Registry.Template(t.Template)
	if !ok {
		return ErrTemplateNotFound
	}

	var autoescapeMode = tmpl.Namespace.Autoescape
	if autoescapeMode == ast.AutoescapeUnspecified {
		autoescapeMode = ast.AutoescapeOn
	}

	var funcs = DefaultFuncs
	if t.Funcs != nil {
		funcs = make(map[string]Func)
		for k, v := range DefaultFuncs {
			funcs[k] = v
		}
		for k, v := range t.Funcs {
			funcs[k] = v
		}
	}

	var directives = DefaultPrintDirectives
	if t.Directives != nil {
		directives = make(map[string]PrintDirective)
		for k, v := range DefaultPrintDirectives {
			directives[k] = v
		}
		for k, v := range t.Directives {
			directives[k] = v
		}
	}

	state := &state{
		tmpl:       tmpl,
		registry:   *t.Registry,
		namespace:  tmpl.Namespace.Name,
		autoescape: autoescapeMode,
		wr:         wr,
		context:    scope{obj},
		ij:         t.Inject,
		funcs:      funcs,
		directives: directives,
	}
	defer state.errRecover(&err)
	state.walk(tmpl.Node)
	return
}
