package pomsg

import (
	"bytes"
	"fmt"

	"github.com/robfig/soy/ast"
)

// Validate checks if the given message is representable in a PO file.
// A MsgNode must be validated before trying to caculate its msgid or msgid_plural
//
// Rules:
//  - If a message contains a plural, it must be the sole child.
//  - A plural contains exactly {case 1} and {default} cases.
func Validate(n *ast.MsgNode) error {
	for i, child := range n.Body.Children() {
		if n, ok := child.(*ast.MsgPluralNode); ok {
			if i != 0 {
				return fmt.Errorf("plural node must be the sole child")
			}
			if len(n.Cases) != 1 || n.Cases[0].Value != 1 {
				return fmt.Errorf("PO requires two plural cases [1, default]. found %v", n.Cases)
			}
		}
	}
	return nil
}

// MsgId returns the msgid for the given msg node.
func Msgid(n *ast.MsgNode) string {
	return msgidn(n, true)
}

// MsgPlural returns the msgid_plural for the given message.
func MsgidPlural(n *ast.MsgNode) string {
	return msgidn(n, false)
}

func msgidn(n *ast.MsgNode, singular bool) string {
	var body = n.Body
	var children = body.Children()
	if len(children) == 0 {
		return ""
	}
	if pluralNode, ok := children[0].(*ast.MsgPluralNode); ok {
		body = pluralCase(pluralNode, singular)
	} else if !singular {
		return ""
	}
	var buf bytes.Buffer
	for _, child := range body.Children() {
		writeph(&buf, child)
	}
	return buf.String()
}

// pluralCase returns the singular or plural message body.
func pluralCase(n *ast.MsgPluralNode, singular bool) ast.ParentNode {
	if singular {
		return n.Cases[0].Body
	}
	return n.Default
}

// writeph writes the placeholder string for the given node to the given buffer.
func writeph(buf *bytes.Buffer, child ast.Node) {
	switch child := child.(type) {
	case *ast.RawTextNode:
		buf.Write(child.Text)
	case *ast.MsgPlaceholderNode:
		buf.WriteString("{" + child.Name + "}")
	}
}
