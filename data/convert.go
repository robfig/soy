package data

import (
	"fmt"
	"reflect"
	"time"
	"unicode"
	"unicode/utf8"
)

var timeType = reflect.TypeOf(time.Time{})

// New converts the given data into a Soy data value, using
// DefaultStructOptions for structs.
func New(value interface{}) Value {
	return NewWith(DefaultStructOptions, value)
}

// NewWith converts the given data value Soy data value, using the provided
// StructOptions for any structs encountered.
func NewWith(convert StructOptions, value interface{}) Value {
	// quick return if we're passed an existing data.Value
	if val, ok := value.(Value); ok {
		return val
	}

	if value == nil {
		return Null{}
	}

	// see if value implements MarshalValue
	if mar, ok := value.(Marshaler); ok {
		return mar.MarshalValue()
	}

	// drill through pointers and interfaces to the underlying type
	var v = reflect.ValueOf(value)
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if !v.IsValid() {
		return Null{}
	}

	if v.Type() == timeType {
		return String(v.Interface().(time.Time).Format(convert.TimeFormat))
	}

	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Int(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return Int(v.Uint())
	case reflect.Float32, reflect.Float64:
		return Float(v.Float())
	case reflect.Bool:
		return Bool(v.Bool())
	case reflect.String:
		return String(v.String())
	case reflect.Slice:
		if v.IsNil() {
			return List(nil)
		}
		slice := []Value{}
		for i := 0; i < v.Len(); i++ {
			slice = append(slice, NewWith(convert, v.Index(i).Interface()))
		}
		return List(slice)
	case reflect.Map:
		var m = make(map[string]Value)
		for _, key := range v.MapKeys() {
			if key.Kind() != reflect.String {
				panic("map keys must be strings")
			}
			m[key.String()] = NewWith(convert, v.MapIndex(key).Interface())
		}
		return Map(m)
	case reflect.Struct:
		return convert.Data(v.Interface())
	default:
		panic(fmt.Errorf("unexpected data type: %T (%v)", value, value))
	}
}

var DefaultStructOptions = StructOptions{
	LowerCamel: true,
	TimeFormat: time.RFC3339,
}

// StructOptions provides flexibility in conversion of structs to soy's
// data.Map format.
type StructOptions struct {
	LowerCamel bool   // if true, convert field names to lowerCamel.
	TimeFormat string // format string for time.Time. (if empty, use ISO-8601)
}

func (c StructOptions) Data(obj interface{}) Map {
	var m = make(map[string]Value)
	var v = reflect.ValueOf(obj)
	var valType = v.Type()
	for i := 0; i < valType.NumField(); i++ {
		if !v.Field(i).CanInterface() {
			continue
		}
		var key = valType.Field(i).Name
		if c.LowerCamel {
			var firstRune, size = utf8.DecodeRuneInString(key)
			key = string(unicode.ToLower(firstRune)) + key[size:]
		}
		m[key] = NewWith(c, v.Field(i).Interface())
	}
	return Map(m)
}

// Marshaler is the interface implemented by entities that can marshal
// themselves into a data.Value.
type Marshaler interface {
	MarshalValue() Value
}
