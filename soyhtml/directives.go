package soyhtml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"text/template"
	"unicode/utf8"

	"github.com/robfig/soy/data"
)

// PrintDirective represents a transformation applied when printing a value.
type PrintDirective struct {
	Apply            func(value data.Value, args []data.Value) data.Value
	ValidArgLengths  []int
	CancelAutoescape bool
}

// PrintDirectives are the builtin print directives.
// Callers may add their own print directives to this map.
var PrintDirectives = map[string]PrintDirective{
	"insertWordBreaks":  {directiveInsertWordBreaks, []int{1}, true},
	"changeNewlineToBr": {directiveChangeNewlineToBr, []int{0}, true},
	"truncate":          {directiveTruncate, []int{1, 2}, false},
	"id":                {directiveNoAutoescape, []int{0}, true},
	"noAutoescape":      {directiveNoAutoescape, []int{0}, true},
	"escapeHtml":        {directiveEscapeHtml, []int{0}, true},
	"escapeUri":         {directiveEscapeUri, []int{0}, true},
	"escapeJsString":    {directiveEscapeJsString, []int{0}, true},
	"bidiSpanWrap":      {nil, []int{0}, false}, // unimplemented
	"bidiUnicodeWrap":   {nil, []int{0}, false}, // unimplemented
	"json":              {directiveJson, []int{0}, true},
}

func directiveInsertWordBreaks(value data.Value, args []data.Value) data.Value {
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

func directiveJson(value data.Value, _ []data.Value) data.Value {
	j, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Errorf("Error JSON encoding value: %v", err))
	}
	return data.String(j)
}
