package soy

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"text/template"
	"unicode/utf8"

	"github.com/robfig/soy/data"
)

type PrintDirective struct {
	Name             string
	ValidArgSizes    []int
	CancelAutoescape bool
	Apply            PrintDirectiveFunc
}

type PrintDirectiveFunc func(value data.Value, args []data.Value) data.Value

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

func directiveInsertWordBreaks(value data.Value, args []data.Value) data.Value {
	if !isInt(args[0]) {
		panic(fmt.Errorf("Parameter of '|insertWordBreaks' is not an integer: %v", args[0]))
	}
	var (
		input    = template.HTMLEscapeString(value.String())
		maxChars = int(args[0].(data.Int))
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
	return data.String(output.String())
}

var newlinePattern = regexp.MustCompile(`\r\n|\r|\n`)

func directiveChangeNewlineToBr(value data.Value, _ []data.Value) data.Value {
	return data.String(newlinePattern.ReplaceAllString(
		template.HTMLEscapeString(value.String()),
		"<br>"))
}

func directiveTruncate(value data.Value, args []data.Value) data.Value {
	if !isInt(args[0]) {
		panic(fmt.Errorf("First parameter of '|truncate' is not an integer: %v", args[0]))
	}
	var maxLen = int(args[0].(data.Int))
	var str = value.String()
	if len(str) <= maxLen {
		return value
	}

	var ellipsis = data.Bool(true)
	if len(args) == 2 {
		var ok bool
		ellipsis, ok = args[1].(data.Bool)
		if !ok {
			panic(fmt.Errorf("Second parameter of '|truncate' is not a bool: %v", args[1]))
		}
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
	return data.String(str)
}

func directiveNoAutoescape(value data.Value, _ []data.Value) data.Value {
	return value
}

func directiveEscapeHtml(value data.Value, _ []data.Value) data.Value {
	return data.String(template.HTMLEscapeString(value.String()))
}

func directiveEscapeUri(value data.Value, _ []data.Value) data.Value {
	return data.String(url.QueryEscape(value.String()))
}

func directiveEscapeJsString(value data.Value, _ []data.Value) data.Value {
	return data.String(template.JSEscapeString(value.String()))
}
