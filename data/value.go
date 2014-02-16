package data

import (
	"math"
	"reflect"
	"strconv"
	"strings"
)

// Value represents a Soy data value, which may be one of the enumerated types.
// The zero value represents an Undefined value.
type Value interface {
	// Truthy returns true according to the Soy definition of truthy and falsy values.
	Truthy() bool

	// String formats this value for display in a template.
	String() string

	// Equals returns true if the two values are equal.  Specifically, if:
	// - They are comparable: they have the same Type, or they are Int and Float
	// - (Primitives) They have the same value
	// - (Lists, Maps) They are the same instance
	// Uncomparable types and unequal values return false.
	Equals(other Value) bool
}

// Value types
type (
	Undefined struct{}
	Null      struct{}
	Bool      bool
	Int       int64
	Float     float64
	String    string
	List      []Value
	Map       map[string]Value
)

// Index retrieves a value from this list, or Undefined if out of bounds.
func (v List) Index(i int) Value {
	if !(0 <= i && i < len(v)) {
		return Undefined{}
	}
	return v[i]
}

// Key retrieves a value under the named key, or Undefined if it doesn't exist.
func (v Map) Key(k string) Value {
	var result, ok = v[k]
	if !ok {
		return Undefined{}
	}
	return result
}

// Truthy ----------

func (v Undefined) Truthy() bool { return false }
func (v Null) Truthy() bool      { return false }
func (v Bool) Truthy() bool      { return bool(v) }
func (v Int) Truthy() bool       { return v != 0 }
func (v Float) Truthy() bool     { return v != 0.0 && float64(v) != math.NaN() }
func (v String) Truthy() bool    { return v != "" }
func (v List) Truthy() bool      { return true }
func (v Map) Truthy() bool       { return true }

// String ----------

func (v Undefined) String() string { panic("Attempted to coerce undefined value into a string.") }
func (v Null) String() string      { return "null" }
func (v Bool) String() string      { return strconv.FormatBool(bool(v)) }
func (v Int) String() string       { return strconv.FormatInt(int64(v), 10) }
func (v Float) String() string     { return strconv.FormatFloat(float64(v), 'g', -1, 64) }
func (v String) String() string    { return "'" + string(v) + "'" }

func (v List) String() string {
	var items = make([]string, len(v))
	for i, item := range v {
		items[i] = item.String()
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func (v Map) String() string {
	var items = make([]string, len(v))
	var i = 0
	for k, v := range v {
		items[i] = k + ": " + v.String()
		i++
	}
	return "{" + strings.Join(items, ", ") + "}"
}

// Equals ----------

func (v Undefined) Equals(other Value) bool {
	_, ok := other.(Undefined)
	return ok
}

func (v Null) Equals(other Value) bool {
	_, ok := other.(Null)
	return ok
}

func (v Bool) Equals(other Value) bool {
	if o, ok := other.(Bool); ok {
		return bool(v) == bool(o)
	}
	return false
}

func (v String) Equals(other Value) bool {
	if o, ok := other.(String); ok {
		return string(v) == string(o)
	}
	return false
}

func (v List) Equals(other Value) bool {
	if o, ok := other.(List); ok {
		return reflect.ValueOf(v).Pointer() == reflect.ValueOf(o).Pointer()
	}
	return false
}

func (v Map) Equals(other Value) bool {
	if o, ok := other.(Map); ok {
		return reflect.ValueOf(v).Pointer() == reflect.ValueOf(o).Pointer()
	}
	return false
}

func (v Int) Equals(other Value) bool {
	switch o := other.(type) {
	case Int:
		return v == o
	case Float:
		return float64(v) == float64(o)
	}
	return false
}

func (v Float) Equals(other Value) bool {
	switch o := other.(type) {
	case Int:
		return float64(v) == float64(o)
	case Float:
		return v == o
	}
	return false
}
