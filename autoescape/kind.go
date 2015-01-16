package autoescape

type kind string

const (
	kindNone kind = ""
	kindText kind = "text"
	kindHTML kind = "html"
	kindCSS  kind = "css"
	kindURL  kind = "uri"
	kindAttr kind = "attributes"
	kindJS   kind = "js"
)
