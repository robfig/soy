package soymsg

import (
	"regexp"

	"github.com/robfig/soy/ast"
)

// Provider provides access to message bundles by locale.
type Provider interface {
	// Bundle returns messages for the given locale, which is in the form
	// [language_territory]. If no locale-specific messages could be found, an
	// empty bundle is returned, which will cause all messages to use the source
	// text.
	Bundle(locale string) Bundle
}

// Bundle is the set of messages available in a particular locale.
type Bundle interface {
	// Message returns the message with the given id, or nil if none was found.
	Message(id uint64) *Message
}

// Message is a (possibly) translated message
type Message struct {
	ID    uint64 // ID is a content-based identifier for this message
	Parts []Part // Parts is a sequence of raw text or placeholders.
}

// Part is an element of a Message
type Part struct {
	Content     string // Content is set if this part is raw text.
	Placeholder string // Placeholder is set if this part should be replaced by another node
}

var phRegex = regexp.MustCompile(`{[A-Z0-9_]+}`)

// NewMessage returns a new message, given its ID and placeholder string.
func NewMessage(id uint64, str string) Message {
	var parts []Part
	var pos = 0
	for _, loc := range phRegex.FindAllStringIndex(str, -1) {
		var start, end = loc[0], loc[1]
		if start > pos {
			parts = append(parts, Part{Content: str[pos:start]})
		}
		parts = append(parts, Part{Placeholder: str[start+1 : end-1]})
		pos = end
	}
	if pos < len(str) {
		parts = append(parts, Part{Content: str[pos:]})
	}
	return Message{id, parts}
}

// SetPlaceholdersAndID generates and sets placeholder names for all children
// nodes, and generates and sets the message ID.
func SetPlaceholdersAndID(n *ast.MsgNode) {
	setPlaceholderNames(n)
	n.ID = calcID(n)
}
