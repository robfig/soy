package soyhtml

import (
	"fmt"
	"io"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/template"
)

// Tofu is a bundle of compiled soy, ready to render to HTML.
type Tofu struct {
	registry   *template.Registry
	funcs      map[string]Func           // functions by name
	directives map[string]PrintDirective // print directives by name
}

// NewTofu returns a new instance that is ready to provide HTML rendering
// services for the given templates, with the default functions and print
// directives.
func NewTofu(registry *template.Registry) *Tofu {
	return &Tofu{registry, DefaultFuncs, DefaultPrintDirectives}
}

// AddFuncs makes funcs available to the template under the given names.
func (tofu *Tofu) AddFuncs(funcs map[string]Func) *Tofu {
	var newfuncs = make(map[string]Func)
	for k, v := range tofu.funcs {
		newfuncs[k] = v
	}
	for k, v := range funcs {
		newfuncs[k] = v
	}
	tofu.funcs = newfuncs
	return tofu
}

// AddDirectives adds print directives
func (tofu *Tofu) AddDirectives(directives map[string]PrintDirective) *Tofu {
	var newdirectives = make(map[string]PrintDirective)
	for k, v := range tofu.directives {
		newdirectives[k] = v
	}
	for k, v := range directives {
		newdirectives[k] = v
	}
	tofu.directives = newdirectives
	return tofu
}

// Render is a convenience function that executes the soy template of the given
// name, using the given object (converted to data.Map) as context, and writes
// the results to the given Writer.
//
// When converting structs to soy's data format, the DefaultStructOptions are
// used. In particular, note that struct properties are converted to lowerCamel
// by default, since that is the Soy naming convention. The caller may update
// those options to change the behavior of this function.
func (tofu Tofu) Render(wr io.Writer, name string, obj interface{}) error {
	var m data.Map
	if obj != nil {
		var ok bool
		m, ok = data.New(obj).(data.Map)
		if !ok {
			return fmt.Errorf("invalid data type. expected map/struct, got %T", obj)
		}
	}
	return tofu.NewRenderer(name).Execute(wr, m)
}

// NewRenderer returns a new instance of a soy html renderer.
func (tofu *Tofu) NewRenderer(name string) *Renderer {
	return &Renderer{
		tofu: tofu,
		name: name,
	}
}
