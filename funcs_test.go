package soy

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
