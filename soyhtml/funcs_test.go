package soyhtml

import (
	"testing"

	"github.com/robfig/soy/data"
)

var rangeTests = []struct{ args, result []int }{
	{[]int{0}, []int{}},
	{[]int{1}, []int{0}},
	{[]int{2}, []int{0, 1}},
	{[]int{0, 1}, []int{0}},
	{[]int{0, 2}, []int{0, 1}},
	{[]int{1, 2}, []int{1}},
	{[]int{1, 3, 1}, []int{1, 2}},
	{[]int{1, 3, 2}, []int{1}},
	{[]int{1, 4, 2}, []int{1, 3}},
}

func TestRange(t *testing.T) {
	for _, test := range rangeTests {
		var args []data.Value
		for _, a := range test.args {
			args = append(args, data.New(a))
		}
		result := funcRange(args).(data.List)
		if len(result) != len(test.result) {
			t.Errorf("%v => %v, expected %v", test.args, result, test.result)
			continue
		}
		for i, r := range test.result {
			if int64(result[i].(data.Int)) != int64(r) {
				t.Errorf("%v => %v, expected %v", test.args, result, test.result)
				break
			}
		}
	}
}

var strContainsTests = []struct {
	arg1, arg2 string
	result     bool
}{
	{"", "", true},
	{"abc", "", true},
	{"abc", "a", true},
	{"abc", "b", true},
	{"abc", "c", true},
	{"abc", "d", false},
	{"abc", "A", false},
	{"abc", "abc", true},
	{"abc", "abcd", false},
}

func TestStrContains(t *testing.T) {
	for _, test := range strContainsTests {
		actual := bool(funcStrContains([]data.Value{data.New(test.arg1), data.New(test.arg2)}).(data.Bool))
		if actual != test.result {
			t.Errorf("strcontains %s %s => %v, expected %v", test.arg1, test.arg2, actual, test.result)
		}
	}
}

func TestRound(t *testing.T) {
	type i []interface{}
	var tests = []struct {
		input    []interface{}
		expected interface{}
	}{
		{i{0}, 0},
		{i{-5}, -5},
		{i{5}, 5},
		{i{1.01}, 1},
		{i{1.99}, 2},
		{i{1.0}, 1},
		{i{-1.01}, -1},
		{i{-1.99}, -2},
		{i{-1.5}, -2},

		{i{1.2345, 1}, 1.2},
		{i{1.2345, 2}, 1.23},
		{i{1.2345, 3}, 1.235},
		{i{1.2345, 4}, 1.2345},
		{i{-1.2345, 1}, -1.2},
		{i{-1.2345, 2}, -1.23},
		{i{-1.2345, 3}, -1.235},
		{i{-1.2345, 4}, -1.2345},
		{i{1.0, 5}, 1.0},

		{i{123.456, -1}, 120},
		{i{123.456, -2}, 100},
		{i{123.456, -3}, 000},
	}

	for _, test := range tests {
		var inputValues []data.Value
		for _, num := range test.input {
			inputValues = append(inputValues, data.New(num))
		}
		actual := funcRound(inputValues)
		if len(inputValues) == 1 {
			// Passing one arg should have the same result as passing the second as 0
			if actual != funcRound(append(inputValues, data.Int(0))) {
				t.Errorf("round %v returned %v, but changed when passed explicit 0", test.input, actual)
			}
		}
		if actual != data.New(test.expected) {
			t.Errorf("round %v => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestFloor(t *testing.T) {
	var tests = []struct {
		input    interface{}
		expected interface{}
	}{
		{0, 0},
		{1, 1},
		{1.1, 1},
		{1.5, 1},
		{1.99, 1},
		{-1, -1},
		{-1.1, -2},
		{-1.9, -2},
	}

	for _, test := range tests {
		var actual = funcFloor([]data.Value{data.New(test.input)})
		if actual != data.New(test.expected) {
			t.Errorf("floor(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestCeil(t *testing.T) {
	var tests = []struct {
		input    interface{}
		expected interface{}
	}{
		{0, 0},
		{1, 1},
		{1.1, 2},
		{1.5, 2},
		{1.99, 2},
		{-1, -1},
		{-1.1, -1},
		{-1.9, -1},
	}

	for _, test := range tests {
		var actual = funcCeiling([]data.Value{data.New(test.input)})
		if actual != data.New(test.expected) {
			t.Errorf("ceiling(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestMin(t *testing.T) {
	type i []interface{}
	var tests = []struct {
		input    []interface{}
		expected interface{}
	}{
		{i{0, 0}, 0},
		{i{1, 2}, 1},
		{i{1.1, 2}, 1.1},
		{i{-1.9, -1.8}, -1.9},
	}

	for _, test := range tests {
		var actual = funcMin(data.New(test.input).(data.List))
		if actual != data.New(test.expected) {
			t.Errorf("min(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestMax(t *testing.T) {
	type i []interface{}
	var tests = []struct {
		input    []interface{}
		expected interface{}
	}{
		{i{0, 0}, 0},
		{i{1, 2}, 2},
		{i{1.1, 2}, 2.0}, // only returns int if both are ints.
		{i{-1.9, -1.8}, -1.8},
	}

	for _, test := range tests {
		var actual = funcMax(data.New(test.input).(data.List))
		if actual != data.New(test.expected) {
			t.Errorf("max(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestIsnonnull(t *testing.T) {
	var tests = []struct {
		input    data.Value
		expected bool
	}{
		{data.Null{}, false},
		{data.Undefined{}, false},
		{data.Bool(false), true},
		{data.Int(0), true},
		{data.Float(0), true},
		{data.String(""), true},
		{data.List{}, true},
		{data.Map{}, true},
	}

	for _, test := range tests {
		var actual = funcIsNonnull([]data.Value{test.input}).(data.Bool)
		if bool(actual) != test.expected {
			t.Errorf("isNonnull(%v) => %v, expected %v", test.input, actual, test.expected)
		}
	}
}

func TestAugmentMap(t *testing.T) {
	type m map[string]interface{}
	var tests = []struct {
		arg1, arg2 map[string]interface{}
		expected   map[string]interface{}
	}{
		{m{}, m{}, m{}},
		{m{"a": 0}, m{}, m{"a": 0}},
		{m{}, m{"a": 0}, m{"a": 0}},
		{m{"a": 0}, m{"a": 1}, m{"a": 1}},
		{m{"a": 0}, m{"b": 1}, m{"a": 0, "b": 1}},
	}

	for _, test := range tests {
		var actual = funcAugmentMap([]data.Value{data.New(test.arg1), data.New(test.arg2)}).(data.Map)

		if len(actual) != len(test.expected) {
			t.Errorf("augmentMap(%v, %v) => %v, expected %v",
				test.arg1, test.arg2, actual, test.expected)
		}
		for k, v := range actual {
			if v != data.New(test.expected[k]) {
				t.Errorf("augmentMap(%v, %v) => %v, expected %v",
					test.arg1, test.arg2, actual, test.expected)
			}
		}
	}
}

func TestKeys(t *testing.T) {
	type m map[string]interface{}
	type i []interface{}
	var tests = []struct {
		input    map[string]interface{}
		expected []interface{}
	}{
		{m{}, i{}},
		{m{"a": 0}, i{"a"}},
	}

	for _, test := range tests {
		var actual = funcKeys([]data.Value{data.New(test.input)}).(data.List)
		if len(actual) != len(test.expected) {
			t.Errorf("keys(%v) => %v, expected %v", test.input, actual, test.expected)
		}
		for i, v := range actual {
			if v != data.New(test.expected[i]) {
				t.Errorf("keys(%v) => %v, expected %v", test.input, actual, test.expected)
			}
		}
	}
}
