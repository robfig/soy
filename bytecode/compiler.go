package bytecode

import (
	"fmt"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/template"
)

type Program struct {
	Instr          []Opcode
	Templates      []Template
	RawTexts       [][]byte
	TemplateByName map[string]*Template
}

// Template holds a compiled template.
type Template struct {
	Name   string
	Params []string
	PC     int // program counter, an index into Instr
}

// Compile compiles an AST (parsed program) into virtual machine instructions.
func Compile(registry *template.Registry) (compiledProg *Program, err error) {
	defer errRecover(&err)

	var p = &Program{}
	var s = compilation{
		prog: p,
	}

	p.Templates = make([]Template, len(registry.Templates))
	p.TemplateByName = make(map[string]*Template, len(registry.Templates))
	for i, tpl := range registry.Templates {
		p.Templates[i] = Template{
			Name: tpl.Node.Name,
		}
		p.TemplateByName[tpl.Node.Name] = &p.Templates[i]
	}

	for i, tpl := range registry.Templates {
		s.tmpl = &p.Templates[i]
		s.walk(tpl.Node)
	}
	return s.prog, nil
}

type compilation struct {
	prog *Program
	tmpl *Template
	node ast.Node // current node, for errors
}

func (s *compilation) walk(node ast.Node) {
	s.at(node)
	switch node := node.(type) {
	case *ast.SoyFileNode:
		s.visitChildren(node)
	case *ast.NamespaceNode:
		return
	case *ast.SoyDocNode:
		return
	case *ast.TemplateNode:
		s.visitTemplate(node)
	case *ast.ListNode:
		s.visitChildren(node)

		// Output nodes ----------
	case *ast.RawTextNode:
		s.add(RawText, Opcode(len(s.prog.RawTexts)))
		s.prog.RawTexts = append(s.prog.RawTexts, node.Text)

	default:
		s.errorf("unknown node (%T): %v", node, node)
	}
}

func (s *compilation) visitChildren(parent ast.ParentNode) {
	for _, child := range parent.Children() {
		s.walk(child)
	}
}

func (s *compilation) visitTemplate(node *ast.TemplateNode) {
	s.tmpl.PC = len(s.prog.Instr)
	s.walk(node.Body)
	s.add(Return)
}

func (s *compilation) add(ops ...Opcode) {
	s.prog.Instr = append(s.prog.Instr, ops...)
}

// at marks the state to be on node n, for error reporting.
func (s *compilation) at(node ast.Node) {
	s.node = node
}

// errorf formats the error and terminates processing.
func (s *compilation) errorf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		*errp = fmt.Errorf("%v", e)
	}
}
