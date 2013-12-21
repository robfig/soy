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

type RawTextNode struct {
	Pos
	Text []byte // The text; may span newlines.
}

func newText(pos Pos, text string) *RawTextNode {
	return &RawTextNode{Pos: pos, Text: []byte(text)}
}

func (t *RawTextNode) String() string {
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

type IdentNode struct {
	Pos
	Ident string // The ident's name.
}

func (i *IdentNode) String() string {
	return i.Ident
}

type MsgNode struct {
	Pos
	Desc string
	Body Node
}

func (n *MsgNode) String() string {
	return fmt.Sprintf("{msg desc=%q}", n.Desc)
}

// Control flow ----------

type IfNode struct {
	Pos
	Conds []*IfCondNode
}

func (n *IfNode) String() string {
	var expr string
	for i, cond := range n.Conds {
		if i == 0 {
			expr += "{if "
		} else if cond.Cond == nil {
			expr += "{else}"
		} else {
			expr += "{elseif "
		}
		expr += cond.String()
	}
	return expr + "{/if}"
}

type IfCondNode struct {
	Pos
	Cond Node // nil if "else"
	Body Node
}

func (n *IfCondNode) String() string {
	var expr string
	if n.Cond != nil {
		expr = n.Cond.String() + "}"
	}
	return expr + n.Body.String()
}

type SwitchNode struct {
	Pos
	Value Node
	Cases []*SwitchCaseNode
}

func (n *SwitchNode) String() string {
	var expr = "{switch " + n.Value.String() + "}"
	for _, caseNode := range n.Cases {
		expr += caseNode.String()
	}
	return expr + "{/switch}"
}

type SwitchCaseNode struct {
	Pos
	Values []Node // len(Values) == 0 => default case
	Body   Node
}

func (n *SwitchCaseNode) String() string {
	var expr = "{case "

	for i, val := range n.Values {
		if i > 0 {
			expr += ","
		}
		expr += val.String()
	}
	return expr + "}" + n.Body.String()
}

// Note:
// - "For" node is required to have a range() call as the List
// - "Foreach" node is required to have a VariableNode as the List
type ForNode struct {
	Pos
	Var     string
	List    Node
	Body    Node
	IfEmpty Node
}

func (n *ForNode) String() string {
	var _, isForeach = n.List.(*DataRefNode)
	var name = "for"
	if isForeach {
		name = "foreach"
	}

	var expr = "{" + name + " "
	expr += n.Var + " in " + n.List.String() + "}" + n.Body.String()
	if n.IfEmpty != nil {
		expr += "{ifempty}" + n.IfEmpty.String()
	}
	return expr + "{/" + name + "}"
}

// Values ----------

type NullNode struct {
	Pos
}

func (s *NullNode) String() string {
	return "null"
}

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

type StringNode struct {
	Pos
	Value string
}

func (s *StringNode) String() string {
	return quoteString(s.Value)
}

// TODO: ValueListNode, MapNode

type FunctionNode struct {
	Pos
	Name string
	Args []Node
}

func (n *FunctionNode) String() string {
	var expr = n.Name + "("
	for i, arg := range n.Args {
		if i > 0 {
			expr += ","
		}
		expr += arg.String()
	}
	return expr + ")"
}

// Data References ----------

type DataRefNode struct {
	Pos
	Key    string
	Access []Node
}

func (n *DataRefNode) String() string {
	var expr = "$" + n.Key
	for _, access := range n.Access {
		expr += access.String()
	}
	return expr
}

type DataRefIndexNode struct {
	Pos
	NullSafe bool
	Index    int
}

func (n *DataRefIndexNode) String() string {
	var expr = "."
	if n.NullSafe {
		expr = "?" + expr
	}
	return expr + strconv.Itoa(n.Index)
}

type DataRefExprNode struct {
	Pos
	NullSafe bool
	Arg      Node
}

func (n *DataRefExprNode) String() string {
	var expr = "["
	if n.NullSafe {
		expr = "?" + expr
	}
	return expr + n.Arg.String() + "]"
}

type DataRefKeyNode struct {
	Pos
	NullSafe bool
	Key      string
}

func (n *DataRefKeyNode) String() string {
	var expr = "."
	if n.NullSafe {
		expr = "?" + expr
	}
	return expr + n.Key
}

type ValueListNode struct {
	Pos
	Items []Node
}

func (n *ValueListNode) String() string {
	var expr = "["
	for i, item := range n.Items {
		if i > 0 {
			expr += ", "
		}
		expr += item.String()
	}
	return expr + "]"
}

type ValueMapNode struct {
	Pos
	Items map[string]Node
}

func (n *ValueMapNode) String() string {
	var expr = "["
	var first = true
	for k, v := range n.Items {
		if !first {
			expr += ", "
		}
		expr += fmt.Sprintf("'%s': %s", k, v.String())
		first = false
	}
	return expr + "]"
}

// Operators ----------

type NotNode struct {
	Pos
	Arg Node
}

func (n *NotNode) String() string {
	return "not " + n.Arg.String()
}

type NegateNode struct {
	Pos
	Arg Node
}

func (n *NegateNode) String() string {
	return "-" + n.Arg.String()
}

type binaryOpNode struct {
	Name string
	Pos
	Arg1, Arg2 Node
}

func (n *binaryOpNode) String() string {
	return n.Arg1.String() + n.Name + n.Arg2.String()
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

type TernNode struct {
	Pos
	Arg1, Arg2, Arg3 Node
}

func (n *TernNode) String() string {
	return "?:"
}
