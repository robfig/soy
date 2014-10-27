package soymsg

import (
	"bytes"
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
	Cases []Case // Cases are the plural cases for this message.
}

// PluralSpec is a description of a particular plural case.
type PluralSpec struct {
	Type          PluralSpecType
	ExplicitValue int // only set if Type == PluralSpecExplicit
}

// PluralSpecType is the class of plural.
type PluralSpecType int

const (
	PluralSpecExplicit PluralSpecType = iota
	PluralSpecZero
	PluralSpecOne
	PluralSpecTwo
	PluralSpecFew
	PluralSpecMany
	PluralSpecOther
)

// Case is one version of the message, for a particular plural case.
type Case struct {
	Spec  PluralSpec
	Parts []Part
}

// Part is an element of a Message.  It is a sequence of text and placeholders
type Part struct {
	Content     string // Content is set if this part is raw text.
	Placeholder string // Placeholder is set if this part should be replaced by another node
}

// NewMessage returns a new message, given its ID and placeholder string.
func NewMessage(id uint64, phstr string) Message {
	return Message{id, []Case{
		{PluralSpec{PluralSpecOther, -1}, Parts(phstr)},
	}}
}

// PlaceholderString returns a string representation of the message containing
// braced placeholders for variables.
func PlaceholderString(n *ast.MsgNode) string {
	var buf bytes.Buffer
	writeFingerprint(&buf, n, true)
	return buf.String()
}

var phRegex = regexp.MustCompile(`{[A-Z0-9_]+}`)

// Parts returns the sequence of raw text and placeholders for the given
// message placeholder string.
func Parts(str string) []Part {
	var pos = 0
	var parts []Part
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
	return parts
}

// SetPlaceholdersAndID generates and sets placeholder names for all children
// nodes, and generates and sets the message ID.
func SetPlaceholdersAndID(n *ast.MsgNode) {
	setPlaceholderNames(n)
	n.ID = calcID(n)
}
