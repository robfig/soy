package parse

import "testing"

func TestStringNode(t *testing.T) {
	var node = StringNode{0, "Aa`! \n \r \t \\ ' \""}
	if node.String() != "'Aa`! \\n \\r \\t \\\\ \\' \"'" {
		t.Errorf("incorrect unescaping: %v", node.String())
	}

	node = StringNode{0, "\u2222 \uEEEE \u9EC4 \u607A"}
	if node.String() != "'\u2222 \uEEEE \u9EC4 \u607A'" {
		t.Errorf("incorrect quoting of unicode")
	}
}

// "[:]"
// "['aaa': 'blah', 'bbb': 123, $boo: $foo]"
func TestMapLiteralNode(t *testing.T) {}

// "[]"
// "['blah', 123, $foo]"
func TestListLiteralNode(t *testing.T) {}
