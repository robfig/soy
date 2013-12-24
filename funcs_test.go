package soy

import (
	"reflect"
	"testing"
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
		var args []reflect.Value
		for _, a := range test.args {
			args = append(args, val(a))
		}
		result := funcRange(args...)
		if result.Len() != len(test.result) {
			t.Errorf("%v => %v, expected %v", test.args, result.Interface(), test.result)
			continue
		}
		for i, r := range test.result {
			if drill(result.Index(i)).Int() != int64(r) {
				t.Errorf("%v => %v, expected %v", test.args, result.Interface(), test.result)
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
		actual := funcStrContains(val(test.arg1), val(test.arg2)).Bool()
		if actual != test.result {
			t.Errorf("strcontains %s %s => %v, expected %v", test.arg1, test.arg2, actual, test.result)
		}
	}
}
