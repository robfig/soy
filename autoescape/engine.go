package autoescape

import (
	"fmt"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/template"
)

type engine struct {
	registry            *template.Registry
	inferences          *inferences
	currentTemplateName string
}

func (e *engine) errorf(code ErrorCode, node ast.Node, f string, args ...interface{}) {
	var line = e.registry.LineNumber(e.currentTemplateName, node)
	errorf(code, line, f, args...)
}

func (e *engine) infer(node ast.Node, start context) (end context) {
	// defer func() {
	// 	// TODO: improve the situation with errors
	// 	if err := recover(); err != nil {
	// 		err2, ok := err.(*Error)
	// 		if !ok {
	// 			err2 = errorf(0, 0, fmt.Sprint(err))
	// 		}
	// 		end = context{state: stateError, err: err2}
	// 	}
	// }()
	return e.walk(node, start)
}

func (e *engine) walk(node ast.Node, start context) (end context) {
	var ctx = start
	switch node := node.(type) {
	case *ast.TemplateNode:
		e.currentTemplateName = node.Name
		if !isValidStartContextForKind(kind(node.Kind), ctx) {
			e.errorf(ErrOutputContext, node,
				"declared kind %v but visited in context %v", node.Kind, ctx.state)
		}
		ctx = context{state: startStateForKind(kind(node.Kind))}

	case *ast.RawTextNode:
		ctx = escapeText(ctx, node)
		if ctx.state == stateError {
			e.errorf(ErrOutputContext, node,
				"starting in %v, failed to compute output context for raw text:\n%s",
				ctx.state, node.Text)
		}

	case *ast.PrintNode:
		// TODO: complain about escape canceling directives
		ctx = ctx.beforeDynamicValue()
		var escapingModes = e.inferences.escapingModes[node]
		if len(escapingModes) == 0 {
			e.inferences.setEscapingDirectives(node, ctx, ctx.escapingModes())
		} else if !ctx.isCompatibleWith(escapingModes[0]) {
			e.errorf(ErrOutputContext, node, "escaping modes %v not compatible with %v: %v",
				escapingModes, ctx.state, node)
		}
		ctx = e.contextAfterEscaping(node, ctx, escapingModes)
	}

	if node, ok := node.(ast.ParentNode); ok {
		for _, child := range node.Children() {
			ctx = e.walk(child, ctx)
		}
	}

	if node, ok := node.(*ast.TemplateNode); ok {
		if !isValidEndContextForKind(kind(node.Kind), ctx) {
			e.errorf(ErrEndContext, node, "template of kind %v may not end in state %v",
				node.Name, node.Kind, ctx.state)
		}
	}

	return ctx
}

func (e *engine) contextAfterEscaping(node ast.Node, start context, escapes []escapingMode) context {
	var end = start
	if len(escapes) > 0 {
		end = start.contextAfterEscaping(escapes[0])
	}
	if end.state == stateError {
		if start.urlPart == urlPartUnknown {
			e.errorf(ErrEndContext, node, "cannot determine URL part of %v", node)
		} else {
			e.errorf(ErrEndContext, node, "{print} or {call} not allowed in comments: %v", node)
		}
	}
	return end
}

func isValidStartContextForKind(kind kind, ctx context) bool {
	if kind == "attributes" {
		return ctx.state == stateAttrName || ctx.state == stateTag
	}
	return ctx.state == startStateForKind(kind)
}

func isValidEndContextForKind(kind kind, ctx context) bool {
	switch kind {
	case kindText:
		panic("state for kind text not implemented")
	case kindNone, kindHTML:
		return ctx.state == stateText
	case kindCSS:
		return ctx.state == stateCSS
	case kindURL:
		return ctx.state == stateURL && ctx.urlPart != urlPartNone
	case kindAttr:
		return ctx.state == stateAttrName || ctx.state == stateTag
	case kindJS:
		return ctx.state == stateJS
	default:
		panic(fmt.Errorf("content kind %v has no associated end context", kind))
	}
}

func likelyEndContextMismatchCause(kind kind, ctx context) string {
	if kind == kindAttr {
		return "an unterminated attribute value, or ending with an unquoted attribute"
	}

	switch ctx.state {
	case stateTag, stateAttrName, stateAfterName, stateBeforeValue:
		return "an unterminated HTML tag or attribute"
	case stateCSS:
		return "an unclosed style block or attribute"
	case stateJS:
		return "an unclosed script block or attribute"
	case stateCSSBlockCmt, stateCSSLineCmt, stateJSBlockCmt, stateJSLineCmt:
		return "an unterminated comment"
	case stateCSSDqStr, stateCSSSqStr, stateJSDqStr, stateJSSqStr:
		return "an unterminated string literal"
	case stateURL, stateCSSURL, stateCSSDqURL, stateCSSSqURL:
		return "an unterminated or empty URI"
	case stateJSRegexp:
		return "an unterminated regular expression"
	default:
		return "unknown to compiler"
	}
}
