package data

import (
	"reflect"
	"testing"
	"time"
)

type AInt struct{ A int }

var jan1, _ = time.Parse(time.RFC3339, "2014-01-01T00:00:00Z")

func TestNew(t *testing.T) {
	tests := []struct{ input, expected interface{} }{
		// basic types
		{nil, Null{}},
		{true, Bool(true)},
		{int(0), Int(0)},
		{int64(0), Int(0)},
		{uint32(0), Int(0)},
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
		{&jan1, String(jan1.Format(time.RFC3339))},

		// structs with all of the above, and unexported fields.
		// also, structs have their fields lowerCamel and Time's default formatting.
		{struct {
			A  Int
			L  List
			PI *int
			no Int
			T  time.Time
		}{Int(5), List{}, pInt(2), 5, jan1},
			Map{"a": Int(5), "l": List{}, "pI": Int(2), "t": String(jan1.Format(time.RFC3339))}},
		{[]*struct {
			PI *AInt
		}{{nil}},
			List{Map{"pI": Null{}}}},
		{testIDURL{1, "https://github.com/robfig/soy"},
			Map{"iD": Int(1), "uRL": String("https://github.com/robfig/soy")}},
		{testIDURLMarshaler{1, "https://github.com/robfig/soy"},
			Map{"id": Int(1), "url": String("https://github.com/robfig/soy")}},
	}

	for _, test := range tests {
		output := New(test.input)
		if !reflect.DeepEqual(test.expected, output) {
			t.Errorf("%#v =>\n %#v, expected:\n%#v", test.input, output, test.expected)
		}
	}
}

type testIDURL struct {
	ID  int
	URL string
}

type testIDURLMarshaler testIDURL

func (t testIDURLMarshaler) MarshalValue() Value {
	return Map{
		"id":  New(t.ID),
		"url": New(t.URL),
	}
}

func TestStructOptions(t *testing.T) {
	var testStruct = struct {
		CaseFormat int
		Time       time.Time
		unexported int
		Nested     struct {
			CaseFormat *bool
			Time       *time.Time
		}
		NestedSlice []interface{}
		NestedMap   map[string]interface{}
	}{
		CaseFormat: 5,
		Time:       jan1,
		NestedSlice: []interface{}{
			"a",
			2,
			DefaultStructOptions,
			true,
			nil,
			5.0,
			[]uint8{1, 2, 3},
			[]string{"a", "b", "c"},
			map[string]interface{}{
				"foo": 1,
				"bar": 2,
				"baz": 3,
			},
		},
		NestedMap: map[string]interface{}{
			"string": "a",
			"int":    1,
			"float":  5.0,
			"nil":    nil,
			"slice":  []*int{pInt(1), pInt(2), pInt(3)},
			"Struct": DefaultStructOptions,
		},
	}

	tests := []struct {
		input    interface{}
		convert  StructOptions
		expected Map
	}{
		{testStruct, DefaultStructOptions, Map{
			"caseFormat": Int(5),
			"time":       String(jan1.Format(time.RFC3339)),
			"nested": Map{
				"caseFormat": Null{},
				"time":       Null{},
			},
			"nestedSlice": List{
				String("a"),
				Int(2),
				Map{
					"lowerCamel": Bool(true),
					"timeFormat": String(time.RFC3339),
				},
				Bool(true),
				Null{},
				Float(5.),
				List{Int(1), Int(2), Int(3)},
				List{String("a"), String("b"), String("c")},
				Map{"foo": Int(1), "bar": Int(2), "baz": Int(3)},
			},
			"nestedMap": Map{
				"string": String("a"),
				"int":    Int(1),
				"float":  Float(5.0),
				"nil":    Null{},
				"slice":  List{Int(1), Int(2), Int(3)},
				"Struct": Map{
					"lowerCamel": Bool(true),
					"timeFormat": String(time.RFC3339),
				}},
		}},

		{testStruct, StructOptions{false, time.Stamp}, Map{
			"CaseFormat": Int(5),
			"Time":       String(jan1.Format(time.Stamp)),
			"Nested": Map{
				"CaseFormat": Null{},
				"Time":       Null{},
			},
			"NestedSlice": List{
				String("a"),
				Int(2),
				Map{
					"LowerCamel": Bool(true),
					"TimeFormat": String(time.RFC3339),
				},
				Bool(true),
				Null{},
				Float(5.),
				List{Int(1), Int(2), Int(3)},
				List{String("a"), String("b"), String("c")},
				Map{"foo": Int(1), "bar": Int(2), "baz": Int(3)},
			},
			"NestedMap": Map{
				"string": String("a"),
				"int":    Int(1),
				"float":  Float(5.0),
				"nil":    Null{},
				"slice":  List{Int(1), Int(2), Int(3)},
				"Struct": Map{
					"LowerCamel": Bool(true),
					"TimeFormat": String(time.RFC3339),
				}},
		}},
	}

	for _, test := range tests {
		output := test.convert.Data(test.input)
		if !reflect.DeepEqual(test.expected, output) {
			t.Errorf("%#v =>\n%#v, expected:\n%#v", test.input, output, test.expected)
		}
	}
}

func BenchmarkStructOptions(b *testing.B) {
	var testStruct = struct {
		CaseFormat int
		Time       time.Time
		unexported int
		Nested     struct {
			CaseFormat *bool
			Time       *time.Time
		}
		NestedSlice []interface{}
		NestedMap   map[string]interface{}
	}{
		CaseFormat: 5,
		Time:       jan1,
		NestedSlice: []interface{}{
			"a",
			2,
			DefaultStructOptions,
			true,
			nil,
			5.0,
			[]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			[]string{"a", "b", "c", "d", "e", "f", "g", "h", "i"},
			map[string]interface{}{
				"foo": 1,
				"bar": 2,
				"baz": 3,
				"boo": 4,
				"poo": 5,
			},
		},
		NestedMap: map[string]interface{}{
			"string": "a",
			"int":    1,
			"float":  5.0,
			"nil":    nil,
			"slice":  []*int{pInt(1), pInt(2), pInt(3)},
			"Struct": DefaultStructOptions,
		},
	}

	for i := 0; i < b.N; i++ {
		var output = NewWith(DefaultStructOptions, testStruct).(Map)
		if len(output) != 5 {
			b.Errorf("unexpected output")
		}
	}
}

func pInt(i int) *int {
	return &i
}
