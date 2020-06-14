package soyjs

import (
	"strings"
)

// The JSFormatter interface allows for callers to choose which
// version of Javascript they would like soyjs to output. To
// maintain backwards compatibility, if no JSFormatter is specified
// in the Options, soyjs will default to the ES5Formatter implemented
// in exec.go
type JSFormatter interface {
	// Template returns two values, the name of the template to save
	// in the defined functions map, and how the function should be defined.
	Template(name string) (string, string)
	// Call returns two values, the name of the template to save
	// in the called functions map, and a string that is written
	// into the imports
	Call(name string) (string, string)
	// Directive takes in a PrintDirective and returns a string
	// that is written into the imports
	Directive(PrintDirective) string
	// Function takes in a Func and returns a string
	// that is written into the imports
	Function(Func) string
}

// ES5Formatter implements the JSFormatter interface
// and creates Javascript files following the ES5
// Javascript format (without imports)
type ES5Formatter struct{}

// ES6Formatter implements the JSFormatter interface
// and creates Javascript files following the ES6
// Javascript format (with imports)
type ES6Formatter struct{}

var _ JSFormatter = (*ES6Formatter)(nil)
var _ JSFormatter = (*ES5Formatter)(nil)

// Template returns two values, the name of the template to save
// in the defined functions map, and how the function should be defined.
// For ES5, the function is not exported, but defined globally
func (f ES5Formatter) Template(name string) (string, string) {
	return name, name + " = function"
}

// Call returns two values, the name of the template to save
// in the called functions map, and a string that is written
// into the imports - for ES5, there are no imports
func (f ES5Formatter) Call(name string) (string, string) {
	return name, ""
}

// Directive takes in a PrintDirective and returns a string
// that is written into the imports - for ES5, there
// are no imports
func (f ES5Formatter) Directive(dir PrintDirective) string {
	return ""
}

// Function takes in a Func and returns a string
// that is written into the imports - for ES5, there
// are no imports
func (f ES5Formatter) Function(fn Func) string {
	return ""
}

// ES6Identifier creates an ES6 compatible function name
// without periods. It replaces all periods, which usually
// denominate namespaces in soy, with a double underscore.
// For example, from the file
//  {namespace say}
//  {template .hello_world}
//  Hello World
//  {/template}
// when ES6Identifier is called on
//  say.hello_world
// it will return
//  say__hello_world
func ES6Identifier(s string) string {
	return strings.Replace(s, ".", "__", -1)
}

// Template returns two values, the name of the template to save
// in the defined functions map, and how the function should be defined.
// For ES6, the function is not defined globally, but exported
func (f ES6Formatter) Template(name string) (string, string) {
	return ES6Identifier(name), "export function " + ES6Identifier(name)
}

// Call returns two values, the name of the template to save
// in the called functions map, and a string that is written
// into the imports
func (f ES6Formatter) Call(name string) (string, string) {
	return ES6Identifier(name), "import { " + ES6Identifier(name) + " } from '" + name + ".js';"
}

// Directive takes in a PrintDirective and returns a string
// that is written into the imports
func (f ES6Formatter) Directive(dir PrintDirective) string {
	return "import { " + ES6Identifier(dir.Name) + " } from '" + dir.Name + ".js';"
}

// Function takes in a Func and returns a string
// that is written into the imports
func (f ES6Formatter) Function(fn Func) string {
	return "import { " + ES6Identifier(fn.Name) + " } from '" + fn.Name + ".js';"
}
