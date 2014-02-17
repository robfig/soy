package template

import "github.com/robfig/soy/ast"

// Template is a Soy template's parse tree, including its preceeding soydoc.
type Template struct {
	*ast.SoyDocNode   // this template's SoyDoc
	*ast.TemplateNode // this template's node

	Namespace *ast.NamespaceNode // this template's namespace
}
