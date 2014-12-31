// Package ast contains definitions for the in-memory representation of a Soy
// template.
package ast

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/robfig/soy/data"
)

// Node represents any singular piece of a soy template.  For example, a
// sequence of raw text or a print tag.
type Node interface {
	String() string // String returns the soy source representation of this node.
	Position() Pos  // byte position of start of node in full original input string
}

// ParentNode is any Node that has descendent nodes.  For example, the Children
// of a AddNode are the two nodes that should be added.
type ParentNode interface {
	Node
	Children() []Node
}

// Pos represents a byte position in the original input text from which this
// template was parsed.  It is useful to construct helpful error messages.
type Pos int

// Position returns this position.  It is implemented as a method so that Nodes
// may embed a Pos and fulfill this part of the Node interface for free.
func (p Pos) Position() Pos {
	return p
}

// SoyFileNode represents a soy file.
type SoyFileNode struct {
	Name string
	Text string
	Body []Node
}

func (n SoyFileNode) Position() Pos {
	return 0
}

func (n SoyFileNode) Children() []Node {
	return n.Body
}

func (n SoyFileNode) String() string {
	var b bytes.Buffer
	for _, n := range n.Body {
		fmt.Fprint(&b, n)
	}
	return b.String()
}

// ListNode holds a sequence of nodes.
type ListNode struct {
	Pos
	Nodes []Node // The element nodes in lexical order.
}

func (l *ListNode) String() string {
	b := new(bytes.Buffer)
	for _, n := range l.Nodes {
		fmt.Fprint(b, n)
	}
	return b.String()
}

func (l *ListNode) Children() []Node {
	return l.Nodes
}

type RawTextNode struct {
	Pos
	Text []byte // The text; may span newlines.
}

func (t *RawTextNode) String() string {
	return string(t.Text)
}

// NamespaceNode registers the namespace of the soy file.
type NamespaceNode struct {
	Pos
	Name       string
	Autoescape string
}

func (c *NamespaceNode) String() string {
	return "{namespace " + c.Name + attrs("autoescape", c.Autoescape) + "}"
}

// TemplateNode holds a template body.
type TemplateNode struct {
	Pos
	Name       string
	Body       *ListNode
	Autoescape string
	Kind       string
}

func (n *TemplateNode) String() string {
	return fmt.Sprintf("{template %s%s}\n%s\n{/template}\n",
		n.Name, attrs("autoescape", n.Autoescape, "kind", n.Kind), n.Body)
}

func (n *TemplateNode) Children() []Node {
	return []Node{n.Body}
}

func attrs(args ...string) string {
	var r string
	for i := 0; i < len(args)-1; i += 2 {
		if args[i+1] != "" {
			r += " " + args[i] + "=" + strconv.Quote(args[i+1])
		}
	}
	return r
}

type SoyDocNode struct {
	Pos
	Params []*SoyDocParamNode
}

func (n *SoyDocNode) String() string {
	if len(n.Params) == 0 {
		return "\n/** */\n"
	}
	var expr = "\n/**"
	for _, param := range n.Params {
		expr += "\n * " + param.String()
	}
	return expr + "\n */\n"
}

func (n *SoyDocNode) Children() []Node {
	var nodes []Node
	for _, param := range n.Params {
		nodes = append(nodes, param)
	}
	return nodes
}

// SoyDocParam represents a parameter to a soy template.
// e.g.
//  /**
//   * Says hello to the person
//   * @param name The name of the person to say hello to.
//   */
type SoyDocParamNode struct {
	Pos
	Name     string // e.g. "name"
	Optional bool
}

func (n *SoyDocParamNode) String() string {
	var expr = "@param"
	if n.Optional {
		expr += "?"
	}
	return expr + " " + n.Name
}

type PrintNode struct {
	Pos
	Arg        Node
	Directives []*PrintDirectiveNode
}

func (n *PrintNode) String() string {
	var expr = "{" + n.Arg.String()
	for _, d := range n.Directives {
		expr += d.String()
	}
	return expr + "}"
}

func (n *PrintNode) Children() []Node {
	var nodes = []Node{n.Arg}
	for _, child := range n.Directives {
		nodes = append(nodes, child)
	}
	return nodes
}

type PrintDirectiveNode struct {
	Pos
	Name string
	Args []Node
}

func (n *PrintDirectiveNode) String() string {
	if len(n.Args) == 0 {
		return "|" + n.Name
	}
	var expr = "|" + n.Name + ":"
	var first = false
	for _, arg := range n.Args {
		if first {
			expr += ","
		}
		expr += arg.String()
	}
	return expr
}

func (n *PrintDirectiveNode) Children() []Node {
	return n.Args
}

type LiteralNode struct {
	Pos
	Body string
}

func (n *LiteralNode) String() string {
	return "{literal}" + n.Body + "{/literal}"
}

type CssNode struct {
	Pos
	Expr   Node
	Suffix string
}

func (n *CssNode) String() string {
	var expr = "{css "
	if n.Expr != nil {
		expr += n.Expr.String() + ", "
	}
	return expr + n.Suffix + "}"
}

func (n *CssNode) Children() []Node {
	return []Node{n.Expr}
}

type LogNode struct {
	Pos
	Body Node
}

func (n *LogNode) String() string {
	return "{log}" + n.Body.String() + "{/log}"
}

func (n *LogNode) Children() []Node {
	return []Node{n.Body}
}

type DebuggerNode struct {
	Pos
}

func (n *DebuggerNode) String() string {
	return "{debugger}"
}

type LetValueNode struct {
	Pos
	Name string
	Expr Node
}

func (n *LetValueNode) String() string {
	return fmt.Sprintf("{let $%s: %s /}", n.Name, n.Expr)
}

func (n *LetValueNode) Children() []Node {
	return []Node{n.Expr}
}

type LetContentNode struct {
	Pos
	Name string
	Kind string
	Body Node
}

func (n *LetContentNode) String() string {
	return fmt.Sprintf("{let $%s%s}%s{/let}", n.Name, attrs("kind", n.Kind), n.Body)
}

func (n *LetContentNode) Children() []Node {
	return []Node{n.Body}
}

type MsgNode struct {
	Pos
	Desc string
	Body Node
}

func (n *MsgNode) String() string {
	return fmt.Sprintf("{msg desc=%q}", n.Desc)
}

func (n *MsgNode) Children() []Node {
	return []Node{n.Body}
}

type CallNode struct {
	Pos
	Name    string
	AllData bool
	Data    Node
	Params  []Node
}

func (n *CallNode) String() string {
	var expr = fmt.Sprintf("{call %s", n.Name)
	if n.AllData {
		expr += ` data="all"`
	} else if n.Data != nil {
		expr += fmt.Sprintf(` data="%s"`, n.Data.String())
	}
	if n.Params == nil {
		return expr + "/}"
	}
	expr += "}"
	for _, param := range n.Params {
		expr += param.String()
	}
	return expr + "{/call}"
}

func (n *CallNode) Children() []Node {
	var nodes []Node
	nodes = append(nodes, n.Data)
	for _, child := range n.Params {
		nodes = append(nodes, child)
	}
	return nodes
}

type CallParamValueNode struct {
	Pos
	Key   string
	Value Node
}

func (n *CallParamValueNode) String() string {
	return fmt.Sprintf("{param %s: %s/}", n.Key, n.Value.String())
}

func (n *CallParamValueNode) Children() []Node {
	return []Node{n.Value}
}

type CallParamContentNode struct {
	Pos
	Key     string
	Kind    string
	Content Node
}

func (n *CallParamContentNode) String() string {
	return fmt.Sprintf("{param %s%s}%s{/param}", n.Key, attrs("kind", n.Kind), n.Content.String())
}

func (n *CallParamContentNode) Children() []Node {
	return []Node{n.Content}
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

func (n *IfNode) Children() []Node {
	var nodes []Node
	for _, child := range n.Conds {
		nodes = append(nodes, child)
	}
	return nodes
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

func (n *IfCondNode) Children() []Node {
	return []Node{n.Cond, n.Body}
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

func (n *SwitchNode) Children() []Node {
	var nodes = []Node{n.Value}
	for _, child := range n.Cases {
		nodes = append(nodes, child)
	}
	return nodes
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

func (n *SwitchCaseNode) Children() []Node {
	var nodes = []Node{n.Body}
	for _, child := range n.Values {
		nodes = append(nodes, child)
	}
	return nodes
}

// Note:
// - "For" node is required to have a range() call as the List
// - "Foreach" node is required to have a DataRefNode as the List
type ForNode struct {
	Pos
	Var     string // without the leading $
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
	expr += "$" + n.Var + " in " + n.List.String() + "}" + n.Body.String()
	if n.IfEmpty != nil {
		expr += "{ifempty}" + n.IfEmpty.String()
	}
	return expr + "{/" + name + "}"
}

func (n *ForNode) Children() []Node {
	var children = make([]Node, 2, 3)
	children[0] = n.List
	children[1] = n.Body
	if n.IfEmpty != nil {
		children = append(children, n.IfEmpty)
	}
	return children
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
	True bool
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
	Quoted string // e.g. 'hello\tworld'
	Value  string // e.g. hello	world
}

func (s *StringNode) String() string {
	return s.Quoted
}

type GlobalNode struct {
	Pos
	Name string
	data.Value
}

func (n *GlobalNode) String() string {
	return n.Name
}

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

func (n *FunctionNode) Children() []Node {
	return n.Args
}

type ListLiteralNode struct {
	Pos
	Items []Node
}

func (n *ListLiteralNode) String() string {
	var expr = "["
	for i, item := range n.Items {
		if i > 0 {
			expr += ", "
		}
		expr += item.String()
	}
	return expr + "]"
}

func (n *ListLiteralNode) Children() []Node {
	return n.Items
}

type MapLiteralNode struct {
	Pos
	Items map[string]Node
}

func (n *MapLiteralNode) String() string {
	if len(n.Items) == 0 {
		return "[:]"
	}
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

func (n *MapLiteralNode) Children() []Node {
	var nodes []Node
	for _, v := range n.Items {
		nodes = append(nodes, v)
	}
	return nodes
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

func (n *DataRefNode) Children() []Node {
	return n.Access
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

func (n *DataRefExprNode) Children() []Node {
	return []Node{n.Arg}
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

// Operators ----------

type NotNode struct {
	Pos
	Arg Node
}

func (n *NotNode) String() string {
	return "not " + n.Arg.String()
}

func (n *NotNode) Children() []Node {
	return []Node{n.Arg}
}

type NegateNode struct {
	Pos
	Arg Node
}

func (n *NegateNode) String() string {
	return "-" + n.Arg.String()
}

func (n *NegateNode) Children() []Node {
	return []Node{n.Arg}
}

type BinaryOpNode struct {
	Name string
	Pos
	Arg1, Arg2 Node
}

func (n *BinaryOpNode) String() string {
	return n.Arg1.String() + n.Name + n.Arg2.String()
}

func (n *BinaryOpNode) Children() []Node {
	return []Node{n.Arg1, n.Arg2}
}

type (
	MulNode   struct{ BinaryOpNode }
	DivNode   struct{ BinaryOpNode }
	ModNode   struct{ BinaryOpNode }
	AddNode   struct{ BinaryOpNode }
	SubNode   struct{ BinaryOpNode }
	EqNode    struct{ BinaryOpNode }
	NotEqNode struct{ BinaryOpNode }
	GtNode    struct{ BinaryOpNode }
	GteNode   struct{ BinaryOpNode }
	LtNode    struct{ BinaryOpNode }
	LteNode   struct{ BinaryOpNode }
	OrNode    struct{ BinaryOpNode }
	AndNode   struct{ BinaryOpNode }
	ElvisNode struct{ BinaryOpNode }
)

type TernNode struct {
	Pos
	Arg1, Arg2, Arg3 Node
}

func (n *TernNode) String() string {
	return n.Arg1.String() + "?" + n.Arg2.String() + ":" + n.Arg3.String()
}

func (n *TernNode) Children() []Node {
	return []Node{n.Arg1, n.Arg2, n.Arg3}
}
