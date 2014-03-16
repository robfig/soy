package soyhtml

import (
	"math"
	"math/rand"
	"strings"

	"github.com/robfig/soy/data"
)

type loopFunc func(s *state, key string) data.Value

var loopFuncs = map[string]loopFunc{
	"index":   funcIndex,
	"isFirst": funcIsFirst,
	"isLast":  funcIsLast,
}

func funcIndex(s *state, key string) data.Value {
	return s.context.lookup(key + "__index")
}

func funcIsFirst(s *state, key string) data.Value {
	return data.Bool(s.context.lookup(key+"__index").(data.Int) == 0)
}

func funcIsLast(s *state, key string) data.Value {
	return data.Bool(
		s.context.lookup(key+"__index").(data.Int) == s.context.lookup(key+"__lastIndex").(data.Int))
}

// Func represents a soy function that may be invoked within a soy template.
type Func struct {
	Apply           func([]data.Value) data.Value
	ValidArgLengths []int
}

// Funcs contains the builtin soy functions.
// Callers may add their own functions to this map as well.
var Funcs = map[string]Func{
	"isNonnull":   {funcIsNonnull, []int{1}},
	"length":      {funcLength, []int{1}},
	"keys":        {funcKeys, []int{1}},
	"augmentMap":  {funcAugmentMap, []int{2}},
	"round":       {funcRound, []int{1, 2}},
	"floor":       {funcFloor, []int{1}},
	"ceiling":     {funcCeiling, []int{1}},
	"min":         {funcMin, []int{2}},
	"max":         {funcMax, []int{2}},
	"randomInt":   {funcRandomInt, []int{1}},
	"strContains": {funcStrContains, []int{2}},
	"range":       {funcRange, []int{1, 2, 3}},
	"hasData":     {funcHasData, []int{0}},
}

func funcIsNonnull(v []data.Value) data.Value {
	return data.Bool(!(v[0] == data.Null{} || v[0] == data.Undefined{}))
}

func funcLength(v []data.Value) data.Value {
	return data.Int(len(v[0].(data.List)))
}

func funcKeys(v []data.Value) data.Value {
	var keys data.List
	for k, _ := range v[0].(data.Map) {
		keys = append(keys, data.String(k))
	}
	return keys
}

func funcAugmentMap(v []data.Value) data.Value {
	var m1 = v[0].(data.Map)
	var m2 = v[1].(data.Map)
	var result = make(data.Map, len(m1)+len(m2)+4)
	for k, v := range m1 {
		result[k] = v
	}
	for k, v := range m2 {
		result[k] = v
	}
	return result
}

func funcRound(v []data.Value) data.Value {
	var digitsAfterPt = 0
	if len(v) == 2 {
		digitsAfterPt = int(v[1].(data.Int))
	}
	var result = round(toFloat(v[0]), digitsAfterPt)
	if digitsAfterPt <= 0 {
		return data.Int(result)
	}
	return data.Float(result)
}

func round(x float64, prec int) float64 {
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	if intermed < 0.0 {
		intermed -= 0.5
	} else {
		intermed += 0.5
	}
	return float64(int64(intermed)) / float64(pow)
}

func funcFloor(v []data.Value) data.Value {
	if isInt(v[0]) {
		return v[0]
	}
	return data.Int(math.Floor(toFloat(v[0])))
}

func funcCeiling(v []data.Value) data.Value {
	if isInt(v[0]) {
		return v[0]
	}
	return data.Int(math.Ceil(toFloat(v[0])))
}

func funcMin(v []data.Value) data.Value {
	if isInt(v[0]) && isInt(v[1]) {
		if v[0].(data.Int) < v[1].(data.Int) {
			return v[0]
		}
		return v[1]
	}
	return data.Float(math.Min(toFloat(v[0]), toFloat(v[1])))
}

func funcMax(v []data.Value) data.Value {
	if isInt(v[0]) && isInt(v[1]) {
		if v[0].(data.Int) > v[1].(data.Int) {
			return v[0]
		}
		return v[1]
	}
	return data.Float(math.Max(toFloat(v[0]), toFloat(v[1])))
}

func funcRandomInt(v []data.Value) data.Value {
	return data.Int(rand.Int63n(int64(v[0].(data.Int))))
}

func funcStrContains(v []data.Value) data.Value {
	return data.Bool(strings.Contains(string(v[0].(data.String)), string(v[1].(data.String))))
}

func funcRange(v []data.Value) data.Value {
	var (
		increment = 1
		init      = 0
		limit     int
	)
	switch len(v) {
	case 3:
		increment = int(v[2].(data.Int))
		fallthrough
	case 2:
		init = int(v[0].(data.Int))
		limit = int(v[1].(data.Int))
	case 1:
		limit = int(v[0].(data.Int))
	}

	var indices data.List
	var i = 0
	for index := init; index < limit; index += increment {
		indices = append(indices, data.Int(index))
		i++
	}
	return indices
}

func funcHasData(v []data.Value) data.Value {
	return data.Bool(true)
}
