package soyhtml

import (
	"fmt"
	"io"

	"github.com/robfig/soy/data"
	"github.com/robfig/soy/template"
)

// Tofu is a bundle of compiled soy, ready to render to HTML.
type Tofu struct {
	registry *template.Registry
}

// NewTofu returns a new instance that is ready to provide HTML rendering
// services for the given templates, with the default functions and print
// directives.
func NewTofu(registry *template.Registry) *Tofu {
	return &Tofu{registry}
}

// Render is a convenience function that executes the Soy template of the given
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

// NewRenderer returns a new instance of a Soy html renderer, given the
// fully-qualified name of the template to render.
func (tofu *Tofu) NewRenderer(name string) *Renderer {
	return &Renderer{
		tofu: tofu,
		name: name,
	}
}
