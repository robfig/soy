package soyjs

// PrintDirective represents a transformation applied when printing a value.
type PrintDirective struct {
	Name             string
	CancelAutoescape bool
}

// PrintDirectives are the builtin print directives.
// Callers may add their own print directives to this map.
var PrintDirectives = map[string]PrintDirective{
	"insertWordBreaks":  {"soy.$$insertWordBreaks", true},
	"changeNewlineToBr": {"soy.$$changeNewlineToBr", true},
	"truncate":          {"soy.$$truncate", false},
	"id":                {"", true}, // visitPrint() will turn into a noop
	"noAutoescape":      {"", true}, // visitPrint() will turn into a noop
	"escapeHtml":        {"soy.$$escapeHtml", true},
	"escapeUri":         {"soy.$$escapeUri", true},
	"escapeJsString":    {"soy.$$escapeJsString", true},
	"bidiSpanWrap":      {"soy.$$bidiSpanWrap", false},
	"bidiUnicodeWrap":   {"soy.$$bidiUnicodeWrap", false},
	"json":              {"JSON.stringify", true},
}
