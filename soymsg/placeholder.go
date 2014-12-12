package soymsg

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/robfig/soy/ast"
)

// setPlaceholderNames generates the placeholder names for all children of the
// given message node, setting the .Name property on them.
func setPlaceholderNames(n *ast.MsgNode) {
	// Step 1: Determine representative nodes and build preliminary map
	var (
		baseNameToRepNodes  = make(map[string][]ast.Node)
		equivNodeToRepNodes = make(map[ast.Node]ast.Node)
	)

	var nodeQueue []ast.Node = phNodes(n.Body)
	for len(nodeQueue) > 0 {
		var node = nodeQueue[0]
		nodeQueue = nodeQueue[1:]

		var baseName string
		switch node := node.(type) {
		case *ast.MsgPlaceholderNode:
			baseName = genBasePlaceholderName(node.Body, "XXX")
		case *ast.MsgPluralNode:
			nodeQueue = append(nodeQueue, pluralCaseBodies(node)...)
			baseName = genBasePlaceholderName(node.Value, "NUM")
		default:
			panic("unexpected")
		}

		if nodes, ok := baseNameToRepNodes[baseName]; !ok {
			baseNameToRepNodes[baseName] = []ast.Node{node}
		} else {
			var isNew = true
			var str = node.String()
			for _, other := range nodes {
				if other.String() == str {
					equivNodeToRepNodes[node] = other
					isNew = false
					break
				}
			}
			if isNew {
				baseNameToRepNodes[baseName] = append(nodes, node)
			}
		}
	}

	// Step 2: Build final maps of name to representative node
	var nameToRepNodes = make(map[string]ast.Node)
	for baseName, nodes := range baseNameToRepNodes {
		if len(nodes) == 1 {
			nameToRepNodes[baseName] = nodes[0]
			continue
		}

		var nextSuffix = 1
		for _, node := range nodes {
			for {
				var newName = baseName + "_" + strconv.Itoa(nextSuffix)
				if _, ok := nameToRepNodes[newName]; !ok {
					nameToRepNodes[newName] = node
					break
				}
				nextSuffix++
			}
		}
	}

	// Step 3: Create maps of every node to its name
	var nodeToName = make(map[ast.Node]string)
	for name, node := range nameToRepNodes {
		nodeToName[node] = name
	}
	for other, repNode := range equivNodeToRepNodes {
		nodeToName[other] = nodeToName[repNode]
	}

	// Step 4: Set the calculated names on all the nodes.
	for node, name := range nodeToName {
		switch node := node.(type) {
		case *ast.MsgPlaceholderNode:
			node.Name = name
		case *ast.MsgPluralNode:
			node.VarName = name
		default:
			panic("unexpected: " + node.String())
		}
	}
}

func phNodes(n ast.ParentNode) []ast.Node {
	var nodeQueue []ast.Node
	for _, child := range n.Children() {
		switch child := child.(type) {
		case *ast.MsgPlaceholderNode, *ast.MsgPluralNode:
			nodeQueue = append(nodeQueue, child)
		}
	}
	return nodeQueue
}

func pluralCaseBodies(node *ast.MsgPluralNode) []ast.Node {
	var r []ast.Node
	for _, plCase := range node.Cases {
		r = append(r, phNodes(plCase.Body)...)
	}
	return append(r, phNodes(node.Default)...)
}

func genBasePlaceholderName(node ast.Node, defaultName string) string {
	// TODO: user supplied placeholder (phname)
	switch part := node.(type) {
	case *ast.PrintNode:
		return genBasePlaceholderNameFromExpr(part.Arg, defaultName)
	case *ast.MsgHtmlTagNode:
		return genBasePlaceholderNameFromHtml(part)
	case *ast.DataRefNode:
		return genBasePlaceholderNameFromExpr(node, defaultName)
	}
	return defaultName
}

func genBasePlaceholderNameFromExpr(expr ast.Node, defaultName string) string {
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
	return defaultName
}

var htmlTagNames = map[string]string{
	"a":   "link",
	"br":  "break",
	"b":   "bold",
	"i":   "italic",
	"li":  "item",
	"ol":  "ordered_list",
	"ul":  "unordered_list",
	"p":   "paragraph",
	"img": "image",
	"em":  "emphasis",
}

func genBasePlaceholderNameFromHtml(node *ast.MsgHtmlTagNode) string {
	var tag, tagType = tagName(node.Text)
	if prettyName, ok := htmlTagNames[tag]; ok {
		tag = prettyName
	}
	return toUpperUnderscore(tagType + tag)
}

func tagName(text []byte) (name, tagType string) {
	switch {
	case bytes.HasPrefix(text, []byte("</")):
		tagType = "END_"
	case bytes.HasSuffix(text, []byte("/>")):
		tagType = ""
	default:
		tagType = "START_"
	}

	text = bytes.TrimPrefix(text, []byte("<"))
	text = bytes.TrimPrefix(text, []byte("/"))
	for i, ch := range text {
		if !isAlphaNumeric(ch) {
			return strings.ToLower(string(text[:i])), tagType
		}
	}
	// the parser should never produce html tag nodes that tagName can't handle.
	panic("no tag name found: " + string(text))
}

func isAlphaNumeric(r byte) bool {
	return 'A' <= r && r <= 'Z' ||
		'a' <= r && r <= 'z' ||
		'0' <= r && r <= '9'
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
