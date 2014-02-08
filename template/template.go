package template

import "github.com/robfig/soy/parse"

// Template is a Soy template's parse tree, including its preceeding soydoc.
type Template struct {
	*parse.SoyDocNode   // this template's SoyDoc
	*parse.TemplateNode // this template's node

	Namespace *parse.NamespaceNode // this template's namespace
}
