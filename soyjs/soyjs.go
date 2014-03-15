package soyjs

import (
	"errors"
	"io"

	"github.com/robfig/soy/template"
)

// Options for js source generation.
type Options struct {
	Funcs map[string]Func // if set, this function map is used instead of the default.
}

// Generator provides an interface to a template registry capable of generating
// javascript to execute the embodied templates.
// The generated javascript requires lib/soyutils.js to already have been loaded.
type Generator struct {
	registry *template.Registry
	funcs    map[string]Func // functions by name
}

func NewGenerator(registry *template.Registry) *Generator {
	return &Generator{registry, DefaultFuncs}
}

// AddFuncs registers the given map of func name to func implementation.
func (gen *Generator) AddFuncs(funcs map[string]Func) *Generator {
	var newfuncs = make(map[string]Func)
	for k, v := range gen.funcs {
		newfuncs[k] = v
	}
	for k, v := range funcs {
		newfuncs[k] = v
	}
	gen.funcs = newfuncs
	return gen
}

var ErrNotFound = errors.New("file not found")

// WriteFile generates javascript corresponding to the soy file of the given name.
func (gen *Generator) WriteFile(out io.Writer, filename string) error {
	for _, soyfile := range gen.registry.SoyFiles {
		if soyfile.Name == filename {
			return Write(out, soyfile, Options{Funcs: gen.funcs})
		}
	}
	return ErrNotFound
}
