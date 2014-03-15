package soyjs

import "github.com/robfig/soy/ast"

// JSWriter is provided to functions to write to the generated javascript.
type JSWriter interface {
	// Write writes the given arguments into the generated javascript.  It is
	// recommended to only pass strings and ast.Nodes to Write. Other types
	// are printed using their default string representation (fmt.Sprintf("%v")).
	Write(...interface{})
}

func (s *state) Write(args ...interface{}) {
	s.js(args...)
}

// Func represents a soy function that may invoked within a template.
type Func struct {
	Apply           func(js JSWriter, args []ast.Node)
	ValidArgLengths []int
}

// DefaultFuncs contains the builtin soy functions.
var DefaultFuncs = map[string]Func{
	"isNonnull":     {funcIsNonnull, []int{1}},
	"length":        {funcLength, []int{1}},
	"keys":          {builtinFunc("getMapKeys"), []int{1}},
	"augmentMap":    {builtinFunc("augmentMap"), []int{2}},
	"round":         {funcRound, []int{1, 2}},
	"floor":         {funcFloor, []int{1}},
	"ceiling":       {funcCeiling, []int{1}},
	"min":           {funcMin, []int{2}},
	"max":           {funcMax, []int{2}},
	"randomInt":     {funcRandomInt, []int{1}},
	"strContains":   {funcStrContains, []int{2}},
	"hasData":       {funcHasData, []int{0}},
	"bidiGlobalDir": {funcBidiGlobalDir, []int{0}},
	"bidiDirAttr":   {funcBidiDirAttr, []int{0}},
	"bidiStartEdge": {funcBidiStartEdge, []int{0}},
	"bidiEndEdge":   {funcBidiEndEdge, []int{0}},
}

// builtinFunc returns a function that writes a call to a soy.$$ builtin func.
func builtinFunc(name string) func(js JSWriter, args []ast.Node) {
	var funcStart = "soy.$$" + name + "("
	return func(js JSWriter, args []ast.Node) {
		js.Write(funcStart)
		for i, arg := range args {
			if i != 0 {
				js.Write(",")
			}
			js.Write(arg)
		}
		js.Write(")")
	}
}

func funcIsNonnull(js JSWriter, args []ast.Node) {
	js.Write(args[0], "!= null")
}

func funcLength(js JSWriter, args []ast.Node) {
	js.Write(args[0], ".length")
}

func funcRound(js JSWriter, args []ast.Node) {
	switch len(args) {
	case 1:
		js.Write("Math.round(", args[0], ")")
	default:
		js.Write(
			"Math.round(", args[0], "* Math.pow(10, ", args[1], ")) / Math.pow(10, ", args[1], ")")
	}
}

func funcFloor(js JSWriter, args []ast.Node) {
	js.Write("Math.floor(", args[0], ")")
}

func funcCeiling(js JSWriter, args []ast.Node) {
	js.Write("Math.ceil(", args[0], ")")
}

func funcMin(js JSWriter, args []ast.Node) {
	js.Write("Math.min(", args[0], ",", args[1], ")")
}

func funcMax(js JSWriter, args []ast.Node) {
	js.Write("Math.max(", args[0], ",", args[1], ")")
}

func funcRandomInt(js JSWriter, args []ast.Node) {
	js.Write("Math.floor(Math.random() * ", args[0], ")")
}

func funcStrContains(js JSWriter, args []ast.Node) {
	js.Write(args[0], ".indexOf(", args[1], ") != -1")
}

func funcHasData(js JSWriter, args []ast.Node) {
	js.Write("true")
}

func funcBidiGlobalDir(js JSWriter, args []ast.Node) {
	js.Write("1")
}

func funcBidiDirAttr(js JSWriter, args []ast.Node) {
	js.Write("soy.$$bidiDirAttr(0, ", args[0], ")")
}

func funcBidiStartEdge(js JSWriter, args []ast.Node) {
	js.Write("'left'")
}

func funcBidiEndEdge(js JSWriter, args []ast.Node) {
	js.Write("'right'")
}
