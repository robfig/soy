package soy

import (
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"strings"
)

type loopFunc func(s *state, key string) reflect.Value

var loopFuncs = map[string]loopFunc{
	"index":   funcIndex,
	"isFirst": funcIsFirst,
	"isLast":  funcIsLast,
}

func funcIndex(s *state, key string) reflect.Value {
	return val(s.context.lookup(key + "__index").Int())
}

func funcIsFirst(s *state, key string) reflect.Value {
	return val(s.context.lookup(key+"__index").Int() == 0)
}

func funcIsLast(s *state, key string) reflect.Value {
	return val(s.context.lookup(key+"__index").Int() == s.context.lookup(key+"__lastIndex").Int())
}

type soyFunc struct {
	Func            func(...reflect.Value) reflect.Value
	ValidArgLengths []int
}

var soyFuncs = map[string]soyFunc{
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

func funcIsNonnull(v ...reflect.Value) reflect.Value {
	return val(v[0].IsValid() && (v[0].Kind() != reflect.Interface || !v[0].IsNil()))
}

func assertKind(v reflect.Value, kind reflect.Kind, name string) {
	if v.Kind() != kind {
		panic(fmt.Errorf("argument to %s(): expected %s, got %v", name, kind, v.Kind()))
	}
}

func funcLength(v ...reflect.Value) reflect.Value {
	assertKind(v[0], reflect.Slice, "length")
	return val(v[0].Len())
}

func funcKeys(v ...reflect.Value) reflect.Value {
	assertKind(v[0], reflect.Map, "keys")
	var keys []interface{}
	for _, keyVal := range v[0].MapKeys() {
		keys = append(keys, keyVal.Interface())
	}
	return val(keys)
}

func funcAugmentMap(v ...reflect.Value) reflect.Value {
	assertKind(v[0], reflect.Map, "augmentMap")
	assertKind(v[1], reflect.Map, "augmentMap")
	var m = make(map[string]interface{}, v[0].Len()+v[1].Len())
	for _, k := range v[0].MapKeys() {
		m[k.Interface().(string)] = v[0].MapIndex(k).Interface()
	}
	for _, k := range v[1].MapKeys() {
		m[k.Interface().(string)] = v[1].MapIndex(k).Interface()
	}
	return val(m)
}

func funcRound(v ...reflect.Value) reflect.Value {
	var digitsAfterPt = 0
	if len(v) == 2 {
		digitsAfterPt = int(v[1].Int())
	}
	return val(round(toFloat(v[0]), digitsAfterPt))
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

func funcFloor(v ...reflect.Value) reflect.Value {
	if isInt(v[0]) {
		return v[0]
	}
	return val(int64(math.Floor(toFloat(v[0]))))
}

func funcCeiling(v ...reflect.Value) reflect.Value {
	if isInt(v[0]) {
		return v[0]
	}
	return val(int64(math.Ceil(toFloat(v[0]))))
}

func funcMin(v ...reflect.Value) reflect.Value {
	if isInt(v[0]) && isInt(v[1]) {
		if v[0].Int() > v[1].Int() {
			return v[0]
		}
		return v[1]
	}
	return val(math.Min(toFloat(v[0]), toFloat(v[1])))
}

func funcMax(v ...reflect.Value) reflect.Value {
	if isInt(v[0]) && isInt(v[1]) {
		if v[0].Int() > v[1].Int() {
			return v[0]
		}
		return v[1]
	}
	return val(math.Max(toFloat(v[0]), toFloat(v[1])))
}

func funcRandomInt(v ...reflect.Value) reflect.Value {
	return val(rand.Int63n(v[0].Int()))
}

func funcStrContains(v ...reflect.Value) reflect.Value {
	assertKind(v[0], reflect.String, "strContains")
	assertKind(v[1], reflect.String, "strContains")
	return val(strings.Contains(v[0].String(), v[1].String()))
}

func funcRange(v ...reflect.Value) reflect.Value {
	var (
		increment = 1
		init      = 0
		limit     int
	)
	switch len(v) {
	case 3:
		increment = int(v[2].Int())
		fallthrough
	case 2:
		init = int(v[0].Int())
		limit = int(v[1].Int())
	case 1:
		limit = int(v[0].Int())
	}

	var indices []interface{}
	var i = 0
	for index := init; index < limit; index += increment {
		indices = append(indices, index)
		i++
	}
	return val(indices)
}

func funcHasData(v ...reflect.Value) reflect.Value {
	return val(true)
}
