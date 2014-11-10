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
	// Locale returns the locale of the bundle.
	Locale() string

	// Message returns the message with the given id, or nil if none was found.
	Message(id uint64) *Message

	// PluralCase returns the index of the case to use for the given plural value.
	PluralCase(n int) int
}

// Message is a (possibly) translated message
type Message struct {
	ID    uint64 // ID is a content-based identifier for this message
	Parts []Part // Parts are the sequence of message parts that form the content.
}

// Part is an element of a Message.  It may be one of the following concrete
// types: RawTextPart, PlaceholderPart, PluralPart
type Part interface{}

// RawTextPart is a segment of a message that displays the contained text.
type RawTextPart struct {
	Text string
}

// PlaceholderPart is a segment of a message that stands in for another node.
type PlaceholderPart struct {
	Name string
}

// PluralPart is a segment of a message that has multiple forms depending on a value.
type PluralPart struct {
	VarName string
	Cases   []PluralCase
}

// PluralCase is one version of the message, for a particular plural case.
type PluralCase struct {
	Spec  PluralSpec
	Parts []Part
}

// PluralSpec is a description of a particular plural case.
type PluralSpec struct {
	Type          PluralSpecType
	ExplicitValue int // only set if Type == PluralSpecExplicit
}

// PluralSpecType is the CLDR plural class.
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

// NewMessage returns a new message, given its ID and placeholder string.
// TODO: plural parts are not parsed from the placeholder string.
func NewMessage(id uint64, phstr string) Message {
	return Message{id, Parts(phstr)}
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
			parts = append(parts, RawTextPart{str[pos:start]})
		}
		parts = append(parts, PlaceholderPart{str[start+1 : end-1]})
		pos = end
	}
	if pos < len(str) {
		parts = append(parts, RawTextPart{str[pos:]})
	}
	return parts
}

// SetPlaceholdersAndID generates and sets placeholder names for all children
// nodes, and generates and sets the message ID.
func SetPlaceholdersAndID(n *ast.MsgNode) {
	setPlaceholderNames(n)
	n.ID = calcID(n)
}
