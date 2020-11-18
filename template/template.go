package template

import "github.com/robfig/soy/ast"

// Template is a Soy template's parse tree, including the relevant context
// (preceding soydoc and namespace).
type Template struct {
	Doc       *ast.SoyDocNode        // this template's SoyDoc, w/ header params added to Doc.Params
	Node      *ast.TemplateNode      // this template's node
	Namespace *ast.NamespaceNode     // this template's namespace
}
