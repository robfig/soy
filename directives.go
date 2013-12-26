package soy

import (
	"bytes"
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"text/template"
	"unicode/utf8"
)

type PrintDirective struct {
	Name             string
	ValidArgSizes    []int
	CancelAutoescape bool
	Apply            PrintDirectiveFunc
}

type PrintDirectiveFunc func(value reflect.Value, args []reflect.Value) reflect.Value

var printDirectives = []*PrintDirective{
	{"insertWordBreaks", []int{1}, true, directiveInsertWordBreaks},
	{"changeNewlineToBr", []int{0}, true, directiveChangeNewlineToBr},
	{"truncate", []int{1, 2}, false, directiveTruncate},
	{"id", []int{0}, true, directiveNoAutoescape},
	{"noAutoescape", []int{0}, true, directiveNoAutoescape},
	{"escapeHtml", []int{0}, true, directiveEscapeHtml},
	{"escapeUri", []int{0}, true, directiveEscapeUri},
	{"escapeJsString", []int{0}, true, directiveEscapeJsString},
}

var printDirectiveByName = make(map[string]*PrintDirective)

func init() {
	for _, directive := range printDirectives {
		printDirectiveByName[directive.Name] = directive
	}
}

func directiveInsertWordBreaks(value reflect.Value, args []reflect.Value) reflect.Value {
	if !isInt(args[0]) {
		panic(fmt.Errorf("Parameter of '|insertWordBreaks' is not an integer: %v", args[0].Interface()))
	}
	var (
		input    = template.HTMLEscapeString(toString(value))
		maxChars = int(args[0].Int())
		chars    = 0
		output   *bytes.Buffer // create the buffer lazily
	)
	for i, ch := range input {
		switch {
		case ch == ' ':
			chars = 0
		case chars >= maxChars:
			if output == nil {
				output = bytes.NewBufferString(input[:i])
			}
			output.WriteString("<wbr>")
			chars = 1
		default:
			chars++
		}
		if output != nil {
			output.WriteRune(ch)
		}
	}
	if output == nil {
		return value
	}
	return val(output.String())
}

var newlinePattern = regexp.MustCompile(`\r\n|\r|\n`)

func directiveChangeNewlineToBr(value reflect.Value, _ []reflect.Value) reflect.Value {
	return val(newlinePattern.ReplaceAllString(template.HTMLEscapeString(toString(value)), "<br>"))
}

func directiveTruncate(value reflect.Value, args []reflect.Value) reflect.Value {
	if !isInt(args[0]) {
		panic(fmt.Errorf("First parameter of '|truncate' is not an integer: %v", args[0].Interface()))
	}
	var maxLen = int(args[0].Int())
	var str = toString(value)
	if len(str) <= maxLen {
		return value
	}

	var ellipsis = true
	if len(args) == 2 {
		if args[1].Kind() != reflect.Bool {
			panic(fmt.Errorf("Second parameter of '|truncate' is not a bool: %v", args[1].Interface()))
		}
		ellipsis = args[1].Bool()
	}

	if ellipsis {
		if maxLen > 3 {
			maxLen -= 3
		} else {
			ellipsis = false
		}
	}

	for !utf8.RuneStart(str[maxLen]) {
		maxLen--
	}

	str = str[:maxLen]
	if ellipsis {
		str += "..."
	}
	return val(str)
}

func directiveNoAutoescape(value reflect.Value, args []reflect.Value) reflect.Value {
	return value
}

func directiveEscapeHtml(value reflect.Value, args []reflect.Value) reflect.Value {
	return val(template.HTMLEscapeString(toString(value)))
}

func directiveEscapeUri(value reflect.Value, args []reflect.Value) reflect.Value {
	return val(url.QueryEscape(toString(value)))
}

func directiveEscapeJsString(value reflect.Value, args []reflect.Value) reflect.Value {
	return val(template.JSEscapeString(toString(value)))
}
