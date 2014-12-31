package autoescape

type AutoescapeType string

const (
	AutoescapeUnspecified AutoescapeType = ""
	AutoescapeOff                        = "false"
	AutoescapeOn                         = "true"
	AutoescapeStrict                     = "strict"
)
