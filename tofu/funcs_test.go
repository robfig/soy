package tofu

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
