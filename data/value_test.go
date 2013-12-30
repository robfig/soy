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

func TestNew(t *testing.T) {
	tests := []struct{ input, expected interface{} }{
		// basic types
		{nil, Null{}},
		{true, Bool(true)},
		{int(0), Int(0)},
		{int64(0), Int(0)},
		{float32(0), Float(0)},
		{"", String("")},
		{[]string{"a"}, List{String("a")}},
		{[]interface{}{"a"}, List{String("a")}},
		{map[string]string{}, Map{}},
		{map[string]string{"a": "b"}, Map{"a": String("b")}},
		{map[string]interface{}{"a": nil}, Map{"a": Null{}}},
		{map[string]interface{}{"a": []int{1}}, Map{"a": List{Int(1)}}},

		// type aliases
		{[]Int{5}, List{Int(5)}},
		{map[string]Value{"a": List{Int(1)}}, Map{"a": List{Int(1)}}},
		{Map{"foo": Null{}}, Map{"foo": Null{}}},

		// pointers
		{pInt(5), Int(5)},

		// structs with all of the above
		// also, structs have their fields lower cased
		{struct {
			A  Int
			L  List
			PI *int
		}{Int(5), List{}, pInt(2)},
			Map{"a": Int(5), "l": List(nil), "pI": Int(2)}},
	}

	for _, test := range tests {
		output := New(test.input)
		if !reflect.DeepEqual(test.expected, output) {
			t.Errorf("%v => %#v, expected %#v", test.input, output, test.expected)
		}
	}
}

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

func pInt(i int) *int {
	return &i
}
