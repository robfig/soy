// Package autoescape provides template rewriters that apply escaping rules.
package autoescape

import (
	"fmt"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/soyhtml"
	"github.com/robfig/soy/template"
)

// Strict rewrites all templates in the given registry to add
// contextually-appropriate escaping directives to all print commands.
//
// Instead of specifying an escaping routine to use for a dynamic value, specify
// the "kind" of the data (text, html, css, uri, js, attributes) and the correct
// escaping routines will be used for the kind of data and the context in which
// it's used.
//
// It implements Strict Autoescaping as documented on the official
// site. However, it does not support mixing autoescape types and will return an
// error if the template requests something other than "strict".
//
// TODO: Support autoescape="false"
// TODO: Support kind
// TODO: Support branches, loops, {let} and {call}
//
// NOTE: There are some differences in the escaping behavior from the official
// implementation. Roughly, this implementation is a little more conservative.
// Here is a partial list
//
//  +----------------+------+-----------+---------+
//  | Context        | From | To (Java) | To (Go) |
//  +----------------+------+-----------+---------+
//  | Attributes     | '    | '         | &#34;   |
//  | JS             | <    | &lt;      | \u003c  |
//  | JS             | >    | &gt;      | \u003e  |
//  | JS String      | /    | /         | \/      |
//  | JS String      | '    | \'        | \x27    |
//  | JS String      | "    | \"        | \x22    |
//  +----------------+------+-----------+---------+
//
func Strict(reg *template.Registry) (err error) {
	var currentTemplate string
	defer func() {
		if err2 := recover(); err2 != nil {
			err = fmt.Errorf("template %v: %v", currentTemplate, err2)
		}
	}()

	e := newEscaper(reg)

	var callGraph = newCallGraph(reg)
	for _, root := range callGraph.roots() {
		// TODO: For now, assume the roots are all HTML context.
		c := e.escape(context{state: stateText}, root.Node)
		if c.err != nil {
			c.err.Name = root.Node.Name
			return c.err
		}
	}

	e.commit()
	return nil
}

func startStateForKind(kind string) state {
	switch kind {
	case "css":
		return stateCSS
	case "", "html":
		return stateText
	case "attributes":
		return stateTag
	case "js":
		return stateJS
	case "uri":
		return stateURL
	case "text":
		// TODO: state where escaping is disabled?
	default:
		panic("unknown kind: " + kind)
	}
}

// funcMap maps command names to functions that render their inputs safe.
// missing: filterHtmlAttributes
// extra: commentEscaper
var funcMap = map[string]func(value data.Value, args []data.Value) data.Value{
	"escapeHtmlAttribute":        attrEscaper,
	"escapeCssString":            cssEscaper,
	"filterCssValue":             cssValueFilter,
	"filterHtmlElementName":      htmlNameFilter,
	"escapeHtml":                 htmlEscaper,
	"escapeJsRegex":              jsRegexpEscaper,
	"escapeJsString":             jsStrEscaper,
	"escapeJsValue":              jsValEscaper,
	"escapeHtmlAttributeNospace": htmlNospaceEscaper,
	"escapeHtmlRcdata":           rcdataEscaper,
	"escapeUri":                  urlEscaper,
	"filterNormalizeUri":         urlFilter,
	"normalizeUri":               urlNormalizer,
}

func init() {
	for k, v := range funcMap {
		soyhtml.PrintDirectives[k] = soyhtml.PrintDirective{v, []int{0}, true}
	}
}

// escaper collects type inferences about templates and changes needed to make
// templates injection safe.
type escaper struct {
	reg *template.Registry
	// xxxNodeEdits are the accumulated edits to apply during commit.
	// Such edits are not applied immediately in case a template set
	// executes a given template in different escaping contexts.
	printNodeEdits map[*ast.PrintNode][]string
}

// newEscaper creates a blank escaper for the given set.
func newEscaper(reg *template.Registry) *escaper {
	return &escaper{
		reg,
		make(map[*ast.PrintNode][]string),
	}
}

// filterFailsafe is an innocuous word that is emitted in place of unsafe values
// by sanitizer functions. It is not a keyword in any programming language,
// contains no special characters, is not empty, and when it appears in output
// it is distinct enough that a developer can find the source of the problem
// via a search engine.
const filterFailsafe = data.String("zSoyz")

// escape escapes a template node.
func (e *escaper) escape(c context, n ast.Node) context {
	switch n := n.(type) {
	case *ast.TemplateNode:
		return e.escape(c, n.Body)
	case *ast.ListNode:
		return e.escapeList(c, n.Nodes)
	case *ast.RawTextNode:
		return escapeText(c, n)
	case *ast.PrintNode:
		return e.escapePrint(c, n)
	}
	panic("escaping " + n.String() + " is unimplemented")
}

// escapeList escapes a list of nodes that provide sequential content.
func (e *escaper) escapeList(c context, nodes []ast.Node) context {
	for _, m := range nodes {
		c = e.escape(c, m)
	}
	return c
}

func (e *escaper) escapePrint(c context, n *ast.PrintNode) context {
	c = nudge(c)
	s := make([]string, 0, 3)
	switch c.state {
	case stateError:
		return c
	case stateURL, stateCSSDqStr, stateCSSSqStr, stateCSSDqURL, stateCSSSqURL, stateCSSURL:
		switch c.urlPart {
		case urlPartNone:
			s = append(s, "filterNormalizeUri")
			fallthrough
		case urlPartPreQuery:
			switch c.state {
			case stateCSSDqStr, stateCSSSqStr:
				s = append(s, "escapeCssString")
			default:
				s = append(s, "normalizeUri")
			}
		case urlPartQueryOrFrag:
			s = append(s, "escapeUri")
		case urlPartUnknown:
			return context{
				state: stateError,
				err:   errorf(ErrAmbigContext, 0, "%s appears in an ambiguous URL context", n),
			}
		default:
			panic(c.urlPart.String())
		}
	case stateJS:
		s = append(s, "escapeJsValue")
		// A slash after a value starts a div operator.
		c.jsCtx = jsCtxDivOp
	case stateJSDqStr, stateJSSqStr:
		s = append(s, "escapeJsString")
	case stateJSRegexp:
		s = append(s, "escapeJsRegex")
	case stateCSS:
		s = append(s, "filterCssValue")
	case stateText:
		s = append(s, "escapeHtml")
	case stateRCDATA:
		s = append(s, "escapeHtmlRcData")
	case stateAttr:
		// Handled below in delim check.
	case stateAttrName, stateTag:
		c.state = stateAttrName
		s = append(s, "filterHtmlElementName")
	default:
		if isComment(c.state) {
			panic("may not {print} within a comment")
		} else {
			panic("unexpected state " + c.state.String())
		}
	}
	switch c.delim {
	case delimNone:
		// No extra-escaping needed for raw text content.
	case delimSpaceOrTagEnd:
		s = append(s, "escapeHtmlAttributeNospace")
	default:
		s = append(s, "escapeHtmlAttribute")
	}
	e.editPrintNode(n, s)
	return c
}

// editPrintNode records a change to a print node
func (e *escaper) editPrintNode(n *ast.PrintNode, directives []string) {
	if _, ok := e.printNodeEdits[n]; ok {
		panic(fmt.Sprintf("node %s already edited", n))
	}
	e.printNodeEdits[n] = directives
}

// commit applies changes to print nodes
func (e *escaper) commit() {
	for node, directives := range e.printNodeEdits {
		for _, directive := range directives {
			node.Directives = append(node.Directives, &ast.PrintDirectiveNode{node.Pos, directive, nil})
		}
	}
}
