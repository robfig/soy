package soymsg

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/robfig/soy/ast"
)

// setPlaceholderNames generates the placeholder names for all children of the
// given message node, setting the .Name property on them.
func setPlaceholderNames(n *ast.MsgNode) {
	// This follows the same algorithm as official Soy.
	// Read the comments there for help following this.
	var (
		baseNameToRepNodes  = make(map[string][]ast.Node)
		equivNodeToRepNodes = make(map[ast.Node]ast.Node)
	)

	var nodeQueue []ast.Node
	for _, child := range n.Body {
		switch child := child.(type) {
		case *ast.MsgPlaceholderNode:
			nodeQueue = append(nodeQueue, child)
		}
	}

	for len(nodeQueue) > 0 {
		var node = nodeQueue[0]
		nodeQueue = nodeQueue[1:]

		var baseName string
		switch node := node.(type) {
		case *ast.MsgPlaceholderNode:
			baseName = genBasePlaceholderName(node.Body)
		default:
			panic("unexpected")
		}

		if nodes, ok := baseNameToRepNodes[baseName]; !ok {
			baseNameToRepNodes[baseName] = []ast.Node{node}
		} else {
			var isNew = true
			var str = node.(*ast.MsgPlaceholderNode).Body.String()
			for _, other := range nodes {
				if other.(*ast.MsgPlaceholderNode).Body.String() == str {
					equivNodeToRepNodes[other] = node
					isNew = false
					break
				}
			}
			if isNew {
				baseNameToRepNodes[baseName] = append(nodes, node)
			}
		}
	}

	var phNameToRepNodes = make(map[string]ast.Node)
	for baseName, nodes := range baseNameToRepNodes {
		if len(nodes) == 1 {
			phNameToRepNodes[baseName] = nodes[0]
			continue
		}

		var nextSuffix = 1
		for _, node := range nodes {
			for {
				var newName = baseName + "_" + strconv.Itoa(nextSuffix)
				if _, ok := phNameToRepNodes[newName]; !ok {
					phNameToRepNodes[newName] = node
					break
				}
				nextSuffix++
			}
		}
	}

	var phNodeToName = make(map[ast.Node]string)
	for name, node := range phNameToRepNodes {
		phNodeToName[node] = name
	}
	for repNode, other := range equivNodeToRepNodes {
		phNodeToName[other] = phNodeToName[repNode]
	}

	for phn, name := range phNodeToName {
		switch phn := phn.(type) {
		case *ast.MsgPlaceholderNode:
			phn.Name = name
		default:
			panic("unexpected: " + phn.String())
		}
	}
}

func genBasePlaceholderName(node ast.Node) string {
	// TODO: user supplied placeholder (phname)
	switch part := node.(type) {
	case *ast.PrintNode:
		return genBasePlaceholderNameFromExpr(part.Arg)
	}
	return "XXX"
}

func genBasePlaceholderNameFromExpr(expr ast.Node) string {
	switch expr := expr.(type) {
	case *ast.GlobalNode:
		return toUpperUnderscore(expr.Name)
	case *ast.DataRefNode:
		if len(expr.Access) == 0 {
			return toUpperUnderscore(expr.Key)
		}
		var lastChild = expr.Access[len(expr.Access)-1]
		if lastChild, ok := lastChild.(*ast.DataRefKeyNode); ok {
			return toUpperUnderscore(lastChild.Key)
		}
	}
	return "XXX"
}

var (
	leadingOrTrailing_ = regexp.MustCompile("^_+|_+$")
	consecutive_       = regexp.MustCompile("__+")
	wordBoundary1      = regexp.MustCompile("([a-zA-Z])([A-Z][a-z])") // <letter>_<upper><lower>
	wordBoundary2      = regexp.MustCompile("([a-zA-Z])([0-9])")      // <letter>_<digit>
	wordBoundary3      = regexp.MustCompile("([0-9])([a-zA-Z])")      // <digit>_<letter>
)

func toUpperUnderscore(ident string) string {
	ident = leadingOrTrailing_.ReplaceAllString(ident, "")
	ident = consecutive_.ReplaceAllString(ident, "${1}_${2}")
	ident = wordBoundary1.ReplaceAllString(ident, "${1}_${2}")
	ident = wordBoundary2.ReplaceAllString(ident, "${1}_${2}")
	ident = wordBoundary3.ReplaceAllString(ident, "${1}_${2}")
	return strings.ToUpper(ident)
}
