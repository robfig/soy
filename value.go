package soy

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// val makes code that uses ValueOf heavily easier to read.
var val = func(v interface{}) reflect.Value { //reflect.ValueOf
	if reflect.TypeOf(v) == reflect.TypeOf(reflect.Value{}) {
		panic("passed value to val()")
	}
	return reflect.ValueOf(v)
}

// TODO: Double check that nils don't end up equal to undefined
var (
	undefinedValue = reflect.Value{}
	nullValue      = reflect.ValueOf(nil)
)

// type tests

func isInt(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
	return false
}

func isFloat(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

// functions

func accessIndex(ref, access reflect.Value, index int) reflect.Value {
	if ref.Kind() != reflect.Slice {
		panic(fmt.Sprintf("While evaluating \"%s\", encountered non-list"+
			" just before accessing \"%s\".", toString(ref), toString(access)))
	}
	if index >= ref.Len() {
		return undefinedValue
	}
	result := ref.Index(index)
	for result.Kind() == reflect.Interface {
		result = result.Elem()
	}
	return result
}

func accessKey(ref, access reflect.Value, key string) reflect.Value {
	result := ref.MapIndex(val(key))
	for result.Kind() == reflect.Interface {
		result = result.Elem()
	}
	return result
}

// equals compares two data values in the '==' soy sense.
// returns false if values are not comparable (e.g. int vs bool).
func equals(val1, val2 reflect.Value) bool {
	switch val1 {
	case undefinedValue:
		return val2 == undefinedValue
	case nullValue:
		return val2 == nullValue
	}
	switch val1.Kind() {
	case reflect.String:
		return val2.Kind() == reflect.String && val1.String() == val2.String()
	case reflect.Bool:
		return val2.Kind() == reflect.Bool && val1.Bool() == val2.Bool()
	case reflect.Slice:
		return val2.Kind() == reflect.Slice && val1.Pointer() == val2.Pointer()
	case reflect.Map:
		return val2.Kind() == reflect.Map && val1.Pointer() == val2.Pointer()
	}
	if (isInt(val1) || isFloat(val1)) && (isInt(val2) || isFloat(val2)) {
		return toFloat(val1) == toFloat(val2)
	}
	return false
}

// toFloat returns the float representation of the given value.
// panics if val is not an int or float
func toFloat(val reflect.Value) float64 {
	for val.Kind() == reflect.Interface {
		val = val.Elem()
	}
	if isFloat(val) {
		return val.Float()
	}
	return float64(val.Int())
}

// truthiness returns the value of this value if coerced into a
// boolean. (true if this object is truthy, false if this object is falsy)
func truthiness(val reflect.Value) bool {
	switch {
	case val == nullValue:
		return false
	case val == undefinedValue:
		return false
	case isInt(val):
		return val.Int() != 0
	case isFloat(val):
		return val.Float() != 0.0 && val.Float() != math.NaN()
	}

	switch val.Kind() {
	case reflect.Bool:
		return val.Bool()
	case reflect.String:
		return len(val.String()) > 0
	case reflect.Slice:
		return true
	case reflect.Map:
		return true
	}
	panic(fmt.Sprintf("unexpected type: %v", val))
}

// toString coerces the given value into a string.
func toString(val reflect.Value) string {
	if !val.IsValid() { // ensure this doesn't fire for nullValue
		panic("Attempted to coerce undefined value into a string.")
	}

	// TODO: is this the right null check?
	for val.Kind() == reflect.Interface || val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return "null"
		}
		val = val.Elem()
	}

	switch {
	case isInt(val):
		return strconv.FormatInt(val.Int(), 10)
	case isFloat(val):
		return strconv.FormatFloat(val.Float(), 'g', -1, 64)
	}

	switch val.Kind() {
	case reflect.Bool:
		if val.Bool() {
			return "true"
		}
		return "false"
	case reflect.String:
		return val.String()
	case reflect.Slice:
		var items = make([]string, val.Len())
		for i := 0; i < val.Len(); i++ {
			items[i] = toString(val.Index(i))
		}
		return "[" + strings.Join(items, ", ") + "]"
	case reflect.Map:
		var keys = val.MapKeys()
		var items = make([]string, len(keys))
		for i := 0; i < len(keys); i++ {
			items[i] = toString(keys[i]) + ": " + toString(val.MapIndex(keys[i]))
		}
		return "{" + strings.Join(items, ", ") + "}"
	}
	panic(fmt.Sprintf("unexpected type: %#v", val.Interface()))
}
