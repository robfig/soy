package parsepasses

import (
	"fmt"

	"github.com/robfig/soy/ast"
	"github.com/robfig/soy/data"
	"github.com/robfig/soy/template"
)

// SetGlobals sets the value of all global nodes in the given registry.
// An error is returned if any globals were left undefined.
func SetGlobals(reg template.Registry, globals data.Map) error {
	for _, t := range reg.Templates {
		if err := SetNodeGlobals(t.Node, globals); err != nil {
			return fmt.Errorf("template %v: %v", t.Node.Name, err)
		}
	}
	return nil
}

// SetNodeGlobals sets global values on the given node and all children nodes,
// using the given data map.  An error is returned if any global nodes were left
// undefined.
func SetNodeGlobals(node ast.Node, globals data.Map) error {
	switch node := node.(type) {
	case *ast.GlobalNode:
		if val, ok := globals[node.Name]; ok {
			node.Value = val
		} else {
			return fmt.Errorf("global %q is undefined", node.Name)
		}
	default:
		if parent, ok := node.(ast.ParentNode); ok {
			for _, child := range parent.Children() {
				if err := SetNodeGlobals(child, globals); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
