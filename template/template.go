package template

import "github.com/robfig/soy/ast"

// Template is a Soy template's parse tree, including the relevant context
// (preceding soydoc and namespace).
type Template struct {
	Doc       *ast.SoyDocNode        // this template's SoyDoc
	Node      *ast.TemplateNode      // this template's node
	Params    []*ast.HeaderParamNode // header params, extracted from the node's Body
	Namespace *ast.NamespaceNode     // this template's namespace
}
