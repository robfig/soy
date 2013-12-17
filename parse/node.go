package parse

import (
	"bytes"
	"fmt"
	"strconv"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

type Node interface {
	String() string
	Position() Pos // byte position of start of node in full original input string
}

// ListNode holds a sequence of nodes.
type ListNode struct {
	Pos
	Nodes []Node // The element nodes in lexical order.
}

func newList(pos Pos) *ListNode {
	return &ListNode{Pos: pos}
}

func (l *ListNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}

func (l *ListNode) String() string {
	b := new(bytes.Buffer)
	for _, n := range l.Nodes {
		fmt.Fprint(b, n)
	}
	return b.String()
}

// TextNode holds plain text.
type TextNode struct {
	Pos
	Text []byte // The text; may span newlines.
}

func newText(pos Pos, text string) *TextNode {
	return &TextNode{Pos: pos, Text: []byte(text)}
}

func (t *TextNode) String() string {
	return fmt.Sprintf(textFormat, t.Text)
}

// NamespaceNode registers the namespace of the soy file.
type NamespaceNode struct {
	Pos
	Name string
	// TODO: attributes
}

func newNamespace(pos Pos, name string) *NamespaceNode {
	return &NamespaceNode{Pos: pos, Name: name}
}

func (c *NamespaceNode) String() string {
	return "{namespace " + c.Name + "}"
}

// TemplateNode holds a template body.
type TemplateNode struct {
	Pos
	Name string
	Body *ListNode
	// TODO: attributes
}

func newTemplate(pos Pos, name string) *TemplateNode {
	return &TemplateNode{Pos: pos, Name: name}
}

func (n *TemplateNode) String() string {
	return fmt.Sprintf("{template %s}%s{/template}", n.Name, n.Body)
}

// SoyDocParam represents a parameter to a soy template.
// They appear within a SoyDocNode.
// e.g.
//  /**
//   * Says hello to the person
//   * @param name The name of the person to say hello to.
//   */
type SoyDocParamNode struct {
	Pos
	Name string // e.g. "name"
	Desc string // e.g. "The name of a the person"
}

// SoyDocNode holds a soydoc comment plus param names
type SoyDocNode struct {
	Pos
	Comment string // TextNode?
	Params  []SoyDocParamNode
}

// func (c *SoyDocNode) append(param SoyDocParamNode) {
// 	c.Params = append(c.Params, param)
// }

func newSoyDoc(pos Pos, body string) *SoyDocNode {
	var n = &SoyDocNode{Pos: pos, Comment: body}
	// TODO: params
	return n
}

func (b *SoyDocNode) String() string {
	return b.Comment
}

type PrintNode struct {
	Pos
	Arg Node
}

func newPrint(pos Pos, arg Node) *PrintNode {
	return &PrintNode{Pos: pos, Arg: arg}
}

func (n *PrintNode) String() string {
	return n.Arg.String()
}

// IdentNode holds an ident.
type IdentNode struct {
	Pos
	Ident string // The ident's name.
}

func (i *IdentNode) String() string {
	return i.Ident
}

// Values ----------

type NullNode struct {
	Pos
}

func (s *NullNode) String() string {
	return "null"
}

// BoolNode holds a boolean constant.
type BoolNode struct {
	Pos
	True bool // The value of the boolean constant.
}

func (b *BoolNode) String() string {
	if b.True {
		return "true"
	}
	return "false"
}

type IntNode struct {
	Pos
	Value int64
}

func (n *IntNode) String() string {
	return strconv.FormatInt(n.Value, 10)
}

type FloatNode struct {
	Pos
	Value float64
}

func (n *FloatNode) String() string {
	return strconv.FormatFloat(n.Value, 'g', -1, 64)
}

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	Pos
	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func (s *StringNode) String() string {
	return s.Quoted
}

// TODO: ValueListNode, MapNode

// VariableNode represents a variable term in a Soy expression
type VariableNode struct {
	Pos
	Name string
}

func (n *VariableNode) String() string {
	return n.Name
}

type FunctionNode struct {
	Pos
	Name string
	Args []Node
}

func (n *FunctionNode) String() string {
	return n.Name
}

// Operators ----------

type NotNode struct {
	Pos
	Arg Node
}

func (n *NotNode) String() string {
	return "not"
}

type binaryOpNode struct {
	Name string
	Pos
	Arg1, Arg2 Node
}

func (n *binaryOpNode) String() string {
	return n.Name
}

type (
	MulNode   struct{ binaryOpNode }
	DivNode   struct{ binaryOpNode }
	ModNode   struct{ binaryOpNode }
	AddNode   struct{ binaryOpNode }
	SubNode   struct{ binaryOpNode }
	EqNode    struct{ binaryOpNode }
	NotEqNode struct{ binaryOpNode }
	GtNode    struct{ binaryOpNode }
	GteNode   struct{ binaryOpNode }
	LtNode    struct{ binaryOpNode }
	LteNode   struct{ binaryOpNode }
	OrNode    struct{ binaryOpNode }
	AndNode   struct{ binaryOpNode }
	ElvisNode struct{ binaryOpNode }
)

type TernaryOpNode struct {
	Pos
	Arg1, Arg2, Arg3 Node
}

func (n *TernaryOpNode) String() string {
	return "?:"
}
