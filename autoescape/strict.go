// Package autoescape provides template rewriters that apply escaping rules.
package autoescape

import (
	"fmt"

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
// TODO: Support branches, loops, {let} and {call}
// TODO: Support autoescape-canceling directives
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

	var inferences = newInferences(reg)
	var engine = engine{
		registry:   reg,
		inferences: inferences,
	}

	for _, t := range reg.Templates {
		var start = context{state: startStateForKind(kind(t.Node.Kind))}
		var end = engine.infer(t.Node, start)
		// TODO: just panic instead of returning an error context (assuming there is
		// no case where it recovers)
		if end.err != nil {
			end.err.Name = t.Node.Name
			return end.err
		}
		inferences.recordTemplateEndContext(t.Node, end)
	}

	return nil
}

func startStateForKind(kind kind) state {
	switch kind {
	case kindCSS:
		return stateCSS
	case kindNone, kindHTML:
		return stateText
	case kindAttr:
		return stateTag
	case kindJS:
		return stateJS
	case kindURL:
		return stateURL
	case kindText:
		panic("TODO: state where escaping is disabled")
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

// filterFailsafe is an innocuous word that is emitted in place of unsafe values
// by sanitizer functions. It is not a keyword in any programming language,
// contains no special characters, is not empty, and when it appears in output
// it is distinct enough that a developer can find the source of the problem
// via a search engine.
const filterFailsafe = data.String("zSoyz")
