package data

import (
	"reflect"
	"testing"
)

// Ensure all of the data types implement Value
var (
	_ Value = Undefined{}
	_ Value = Null{}
	_ Value = Bool(false)
	_ Value = Int(0)
	_ Value = Float(0.0)
	_ Value = String("")
	_ Value = List{}
	_ Value = Map{}
)

func TestKey(t *testing.T) {
	tests := []struct {
		input    interface{}
		key      string
		expected interface{}
	}{
		{map[string]interface{}{}, "foo", Undefined{}},
		{map[string]interface{}{"foo": nil}, "foo", Null{}},
	}

	for _, test := range tests {
		actual := New(test.input).(Map).Key(test.key)
		if !reflect.DeepEqual(test.expected, actual) {
			t.Errorf("%v => %#v, expected %#v", test.input, actual, test.expected)
		}
	}
}

func TestIndex(t *testing.T) {
	tests := []struct {
		input    interface{}
		index    int
		expected interface{}
	}{
		{[]interface{}{}, 0, Undefined{}},
		{[]interface{}{1}, 0, Int(1)},
	}

	for _, test := range tests {
		actual := New(test.input).(List).Index(test.index)
		if !reflect.DeepEqual(test.expected, actual) {
			t.Errorf("%v => %#v, expected %#v", test.input, actual, test.expected)
		}
	}
}
