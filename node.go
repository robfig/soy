package soy

import (
	"bytes"
	"fmt"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

type Node interface {
	Type() NodeType
	String() string
	Position() Pos // byte position of start of node in full original input string
}

// NodeType identifies the type of a parse tree node.
type NodeType int

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeText    NodeType = iota // Plain text.
	NodeBool                    // A boolean constant.
	NodeCommand                 // An element of a pipeline.
	NodeSoyDoc                  // A template's soydoc.
	NodeNamespace
	nodeElse     // An else action. Not added to tree.
	nodeEnd      // An end action. Not added to tree.
	NodeIdent    // An identifier
	NodeIf       // An if action.
	NodeList     // A list of Nodes.
	NodeNumber   // A numerical constant.
	NodeFor      // A for loop.
	NodeString   // A string constant.
	NodeTemplate // A template declaration.
	NodeVariable // A $ variable.
)

// ListNode holds a sequence of nodes.
type ListNode struct {
	NodeType
	Pos
	Nodes []Node // The element nodes in lexical order.
}

func newList(pos Pos) *ListNode {
	return &ListNode{NodeType: NodeList, Pos: pos}
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
	NodeType
	Pos
	Text []byte // The text; may span newlines.
}

func newText(pos Pos, text string) *TextNode {
	return &TextNode{NodeType: NodeText, Pos: pos, Text: []byte(text)}
}

func (t *TextNode) String() string {
	return fmt.Sprintf(textFormat, t.Text)
}

// CommandNode holds a command (a pipeline inside an evaluating action).
type CommandNode struct {
	NodeType
	Pos
	Args []Node // Arguments in lexical order: Identifier, field, or constant.
}

func newCommand(pos Pos) *CommandNode {
	return &CommandNode{NodeType: NodeCommand, Pos: pos}
}

func (c *CommandNode) append(arg Node) {
	c.Args = append(c.Args, arg)
}

func (c *CommandNode) String() string {
	s := ""
	for i, arg := range c.Args {
		if i > 0 {
			s += " "
		}
		s += arg.String()
	}
	return s
}

// NamespaceNode registers the namespace of the soy file.
type NamespaceNode struct {
	NodeType
	Pos
	Name string
	// TODO: attributes
}

func newNamespace(pos Pos, name string) *NamespaceNode {
	return &NamespaceNode{NodeType: NodeNamespace, Pos: pos, Name: name}
}

func (c *NamespaceNode) String() string {
	return "{namespace " + c.Name + "}"
}

// VariableNode represents a variable term in a Soy expression
type VariableNode struct {
	NodeType
	Pos
	Name string
}

func newVariable(pos Pos, name string) *VariableNode {
	return &VariableNode{NodeType: NodeVariable, Pos: pos, Name: name}
}

func (n *VariableNode) String() string {
	return n.Name
}

// TemplateNode holds a template body.
type TemplateNode struct {
	NodeType
	Pos
	Name string
	Body *ListNode
	// TODO: attributes
}

func newTemplate(pos Pos, name string) *TemplateNode {
	return &TemplateNode{NodeType: NodeTemplate, Pos: pos, Name: name}
}

func (n *TemplateNode) String() string {
	return fmt.Sprintf("{template %s}%s{/template}", n.Name, n.Body)
}

// IdentifierNode holds an identifier.
type IdentifierNode struct {
	NodeType
	Pos
	Ident string // The identifier's name.
}

// NewIdentifier returns a new IdentifierNode with the given identifier name.
func NewIdent(ident string) *IdentifierNode {
	return &IdentifierNode{NodeType: NodeIdent, Ident: ident}
}

// SetPos sets the position. NewIdentifier is a public method so we can't modify its signature.
// Chained for convenience.
// TODO: fix one day?
func (i *IdentifierNode) SetPos(pos Pos) *IdentifierNode {
	i.Pos = pos
	return i
}

func (i *IdentifierNode) String() string {
	return i.Ident
}

func (i *IdentifierNode) Copy() Node {
	return NewIdent(i.Ident).SetPos(i.Pos)
}

// StringNode holds a string constant. The value has been "unquoted".
type StringNode struct {
	NodeType
	Pos
	Quoted string // The original text of the string, with quotes.
	Text   string // The string, after quote processing.
}

func newString(pos Pos, orig, text string) *StringNode {
	return &StringNode{NodeType: NodeString, Pos: pos, Quoted: orig, Text: text}
}

func (s *StringNode) String() string {
	return s.Quoted
}

// BoolNode holds a boolean constant.
type BoolNode struct {
	NodeType
	Pos
	True bool // The value of the boolean constant.
}

func newBool(pos Pos, true bool) *BoolNode {
	return &BoolNode{NodeType: NodeBool, Pos: pos, True: true}
}

func (b *BoolNode) String() string {
	if b.True {
		return "true"
	}
	return "false"
}

// SoyDocParam represents a parameter to a soy template.
// They appear within a SoyDocNode.
// e.g.
//  /**
//   * Says hello to the person
//   * @param name The name of the person to say hello to.
//   */
type SoyDocParamNode struct {
	NodeType
	Pos
	Name string // e.g. "name"
	Desc string // e.g. "The name of a the person"
}

// SoyDocNode holds a soydoc comment plus param names
type SoyDocNode struct {
	NodeType
	Pos
	Comment string // TextNode?
	Params  []SoyDocParamNode
}

// func (c *SoyDocNode) append(param SoyDocParamNode) {
// 	c.Params = append(c.Params, param)
// }

func newSoyDoc(pos Pos, body string) *SoyDocNode {
	var n = &SoyDocNode{NodeType: NodeSoyDoc, Pos: pos, Comment: body}
	// TODO: params
	return n
}

func (b *SoyDocNode) String() string {
	return b.Comment
}
